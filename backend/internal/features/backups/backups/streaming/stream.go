// Package streaming provides a reusable pipeline that copies a plain io.Reader
// (e.g. a Redis SYNC snapshot or a RabbitMQ definitions response) into storage
// through the shared encryption and byte-counting writers.
//
// The exec-based engines (Postgres/MySQL/MariaDB/MongoDB) manage a subprocess
// and keep their own pipeline; this helper exists only for engines whose source
// is already an io.Reader, so they don't each reimplement the pipe wiring,
// shutdown checks and cancellation handling.
package streaming

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"databasus-backend/internal/config"
	common "databasus-backend/internal/features/backups/backups/common"
	backup_encryption "databasus-backend/internal/features/backups/backups/encryption"
	backups_config "databasus-backend/internal/features/backups/config"
	encryption_secrets "databasus-backend/internal/features/encryption/secrets"
	"databasus-backend/internal/features/storages"
	"databasus-backend/internal/util/encryption"
)

const (
	backupTimeout            = 23 * time.Hour
	shutdownCheckInterval    = 1 * time.Second
	copyBufferSize           = 8 * 1024 * 1024
	progressReportIntervalMB = 1.0
)

// Spec carries everything needed to stream a single source into storage.
type Spec struct {
	BackupID         uuid.UUID
	FileName         string
	BackupConfig     *backups_config.BackupConfig
	Source           io.Reader
	SourceCloser     io.Closer
	Storage          *storages.Storage
	ProgressListener func(completedMBs float64)
	Logger           *slog.Logger
	Encryptor        encryption.FieldEncryptor
	SecretKeyService *encryption_secrets.SecretKeyService
}

// Run copies spec.Source into storage and returns the resulting backup
// metadata along with the number of plaintext bytes streamed.
func Run(parentCtx context.Context, spec Spec) (*common.BackupMetadata, int64, error) {
	ctx, cancel := createBackupContext(parentCtx)
	defer cancel()

	// Closing the source on cancellation unblocks a Read that would otherwise
	// hang forever on a stalled connection.
	if spec.SourceCloser != nil {
		stop := context.AfterFunc(ctx, func() {
			_ = spec.SourceCloser.Close()
		})
		defer stop()
	}

	storageReader, storageWriter := io.Pipe()

	finalWriter, encryptionWriter, backupMetadata, err := setupBackupEncryption(spec, storageWriter)
	if err != nil {
		return nil, 0, err
	}

	countingWriter := common.NewCountingWriter(finalWriter)

	saveErrCh := make(chan error, 1)
	go func() {
		saveErr := spec.Storage.SaveFile(
			ctx,
			spec.Encryptor,
			spec.Logger,
			spec.FileName,
			storageReader,
		)
		if saveErr != nil {
			_ = storageReader.CloseWithError(saveErr)
			cancel()
		}
		saveErrCh <- saveErr
	}()

	bytesWritten, copyErr := copyWithShutdownCheck(
		ctx,
		countingWriter,
		spec.Source,
		spec.ProgressListener,
	)

	if copyErr != nil {
		_ = storageWriter.CloseWithError(copyErr)
		<-saveErrCh

		if ctxErr := contextFailure(ctx); ctxErr != nil {
			return nil, 0, ctxErr
		}

		return nil, 0, fmt.Errorf("copy to storage: %w", copyErr)
	}

	if err := closeWriters(spec.Logger, encryptionWriter, storageWriter); err != nil {
		<-saveErrCh
		return nil, 0, err
	}

	saveErr := <-saveErrCh
	if saveErr != nil {
		return nil, 0, fmt.Errorf("save to storage: %w", saveErr)
	}

	if spec.ProgressListener != nil {
		spec.ProgressListener(float64(bytesWritten) / (1024 * 1024))
	}

	return &backupMetadata, bytesWritten, nil
}

func createBackupContext(parentCtx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parentCtx, backupTimeout)

	go func() {
		ticker := time.NewTicker(shutdownCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if config.IsShouldShutdown() {
					cancel()
					return
				}
			}
		}
	}()

	return ctx, cancel
}

func setupBackupEncryption(
	spec Spec,
	storageWriter io.WriteCloser,
) (io.Writer, *backup_encryption.EncryptionWriter, common.BackupMetadata, error) {
	backupMetadata := common.BackupMetadata{
		BackupID:   spec.BackupID,
		Encryption: backups_config.BackupEncryptionNone,
	}

	if spec.BackupConfig.Encryption != backups_config.BackupEncryptionEncrypted {
		return storageWriter, nil, backupMetadata, nil
	}

	masterKey, err := spec.SecretKeyService.GetSecretKey()
	if err != nil {
		return nil, nil, backupMetadata, fmt.Errorf("failed to get master key: %w", err)
	}

	encSetup, err := backup_encryption.SetupEncryptionWriter(storageWriter, masterKey, spec.BackupID)
	if err != nil {
		return nil, nil, backupMetadata, err
	}

	backupMetadata.Encryption = backups_config.BackupEncryptionEncrypted
	backupMetadata.EncryptionSalt = &encSetup.SaltBase64
	backupMetadata.EncryptionIV = &encSetup.NonceBase64

	return encSetup.Writer, encSetup.Writer, backupMetadata, nil
}

func copyWithShutdownCheck(
	ctx context.Context,
	dst io.Writer,
	src io.Reader,
	progressListener func(completedMBs float64),
) (int64, error) {
	buf := make([]byte, copyBufferSize)
	var totalWritten int64
	var lastReportedMB float64

	for {
		select {
		case <-ctx.Done():
			return totalWritten, ctx.Err()
		default:
		}

		if config.IsShouldShutdown() {
			return totalWritten, errors.New("shutdown requested")
		}

		nr, readErr := src.Read(buf)
		if nr > 0 {
			nw, writeErr := dst.Write(buf[:nr])
			if writeErr != nil {
				return totalWritten, writeErr
			}

			if nw != nr {
				return totalWritten, io.ErrShortWrite
			}

			totalWritten += int64(nw)

			if progressListener != nil {
				currentMB := float64(totalWritten) / (1024 * 1024)
				if currentMB-lastReportedMB >= progressReportIntervalMB {
					progressListener(currentMB)
					lastReportedMB = currentMB
				}
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return totalWritten, nil
			}

			return totalWritten, readErr
		}
	}
}

func closeWriters(
	logger *slog.Logger,
	encryptionWriter *backup_encryption.EncryptionWriter,
	storageWriter *io.PipeWriter,
) error {
	if encryptionWriter != nil {
		if err := encryptionWriter.Close(); err != nil {
			logger.Error("Failed to close encryption writer", "error", err)
			return fmt.Errorf("failed to close encryption writer: %w", err)
		}
	}

	if err := storageWriter.Close(); err != nil {
		logger.Error("Failed to close storage writer", "error", err)
		return fmt.Errorf("failed to close storage writer: %w", err)
	}

	return nil
}

func contextFailure(ctx context.Context) error {
	select {
	case <-ctx.Done():
		if config.IsShouldShutdown() {
			return errors.New("backup cancelled due to shutdown")
		}

		return fmt.Errorf("backup cancelled: %w", ctx.Err())
	default:
		return nil
	}
}

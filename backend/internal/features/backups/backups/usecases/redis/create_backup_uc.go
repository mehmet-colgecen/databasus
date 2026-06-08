package usecases_redis

import (
	"context"
	"errors"
	"log/slog"

	common "databasus-backend/internal/features/backups/backups/common"
	backups_core "databasus-backend/internal/features/backups/backups/core"
	"databasus-backend/internal/features/backups/backups/streaming"
	backups_config "databasus-backend/internal/features/backups/config"
	"databasus-backend/internal/features/databases"
	encryption_secrets "databasus-backend/internal/features/encryption/secrets"
	"databasus-backend/internal/features/storages"
	"databasus-backend/internal/util/encryption"
)

type CreateRedisBackupUsecase struct {
	logger           *slog.Logger
	secretKeyService *encryption_secrets.SecretKeyService
	fieldEncryptor   encryption.FieldEncryptor
}

func (uc *CreateRedisBackupUsecase) Execute(
	ctx context.Context,
	backup *backups_core.Backup,
	backupConfig *backups_config.BackupConfig,
	db *databases.Database,
	storage *storages.Storage,
	backupProgressListener func(completedMBs float64),
) (*common.BackupMetadata, error) {
	logger := uc.logger.With("database_id", db.ID, "backup_id", backup.ID, "storage_id", storage.ID)
	logger.Info("creating Redis backup via SYNC")

	redisDb := db.Redis
	if redisDb == nil {
		return nil, errors.New("redis database configuration is required")
	}

	rawSizeMB, err := redisDb.GetRawDbSizeMb(ctx, logger, uc.fieldEncryptor)
	if err != nil {
		logger.Warn("failed to fetch raw db size before backup", "error", err)
	} else {
		backup.BackupRawDbSizeMb = rawSizeMB
	}

	source, closer, err := redisDb.OpenSyncStream(ctx, uc.fieldEncryptor)
	if err != nil {
		return nil, err
	}

	metadata, _, err := streaming.Run(ctx, streaming.Spec{
		BackupID:         backup.ID,
		FileName:         backup.FileName,
		BackupConfig:     backupConfig,
		Source:           source,
		SourceCloser:     closer,
		Storage:          storage,
		ProgressListener: backupProgressListener,
		Logger:           logger,
		Encryptor:        uc.fieldEncryptor,
		SecretKeyService: uc.secretKeyService,
	})
	if err != nil {
		return nil, err
	}

	return metadata, nil
}

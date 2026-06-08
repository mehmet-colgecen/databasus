package usecases_rabbitmq

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

type CreateRabbitmqBackupUsecase struct {
	logger           *slog.Logger
	secretKeyService *encryption_secrets.SecretKeyService
	fieldEncryptor   encryption.FieldEncryptor
}

func (uc *CreateRabbitmqBackupUsecase) Execute(
	ctx context.Context,
	backup *backups_core.Backup,
	backupConfig *backups_config.BackupConfig,
	db *databases.Database,
	storage *storages.Storage,
	backupProgressListener func(completedMBs float64),
) (*common.BackupMetadata, error) {
	logger := uc.logger.With("database_id", db.ID, "backup_id", backup.ID, "storage_id", storage.ID)
	logger.Info("creating RabbitMQ backup via definitions export")

	rabbitmqDb := db.Rabbitmq
	if rabbitmqDb == nil {
		return nil, errors.New("rabbitmq database configuration is required")
	}

	source, closer, err := rabbitmqDb.OpenDefinitionsStream(ctx, uc.fieldEncryptor)
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

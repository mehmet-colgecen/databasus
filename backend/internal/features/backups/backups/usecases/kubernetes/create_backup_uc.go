package usecases_kubernetes

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

type CreateKubernetesBackupUsecase struct {
	logger           *slog.Logger
	secretKeyService *encryption_secrets.SecretKeyService
	fieldEncryptor   encryption.FieldEncryptor
}

func (uc *CreateKubernetesBackupUsecase) Execute(
	ctx context.Context,
	backup *backups_core.Backup,
	backupConfig *backups_config.BackupConfig,
	db *databases.Database,
	storage *storages.Storage,
	backupProgressListener func(completedMBs float64),
) (*common.BackupMetadata, error) {
	logger := uc.logger.With("database_id", db.ID, "backup_id", backup.ID, "storage_id", storage.ID)
	logger.Info("creating Kubernetes backup via resource export")

	kubernetesDb := db.Kubernetes
	if kubernetesDb == nil {
		return nil, errors.New("kubernetes database configuration is required")
	}

	source, closer, err := kubernetesDb.OpenExportStream(ctx)
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

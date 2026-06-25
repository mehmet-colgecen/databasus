package usecases

import (
	"context"
	"errors"

	backups_core "databasus-backend/internal/features/backups/backups/core"
	backups_config "databasus-backend/internal/features/backups/config"
	"databasus-backend/internal/features/databases"
	restores_core "databasus-backend/internal/features/restores/core"
	usecases_postgresql "databasus-backend/internal/features/restores/usecases/postgresql"
	"databasus-backend/internal/features/storages"
)

type RestoreBackupUsecase struct {
	restorePostgresqlBackupUsecase *usecases_postgresql.RestorePostgresqlBackupUsecase
}

func (uc *RestoreBackupUsecase) Execute(
	ctx context.Context,
	backupConfig *backups_config.BackupConfig,
	restore restores_core.Restore,
	originalDB *databases.Database,
	restoringToDB *databases.Database,
	backup *backups_core.Backup,
	storage *storages.Storage,
	isExcludeExtensions bool,
) error {
	switch originalDB.Type {
	case databases.DatabaseTypePostgres:
		return uc.restorePostgresqlBackupUsecase.Execute(
			ctx,
			originalDB,
			restoringToDB,
			backupConfig,
			restore,
			backup,
			storage,
			isExcludeExtensions,
		)
	default:
		return errors.New("database type not supported")
	}
}

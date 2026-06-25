package usecases

import (
	usecases_postgresql "databasus-backend/internal/features/restores/usecases/postgresql"
)

var restoreBackupUsecase = &RestoreBackupUsecase{
	usecases_postgresql.GetRestorePostgresqlBackupUsecase(),
}

func GetRestoreBackupUsecase() *RestoreBackupUsecase {
	return restoreBackupUsecase
}

package usecases

import (
	usecases_kubernetes "databasus-backend/internal/features/backups/backups/usecases/kubernetes"
	usecases_postgresql "databasus-backend/internal/features/backups/backups/usecases/postgresql"
	usecases_rabbitmq "databasus-backend/internal/features/backups/backups/usecases/rabbitmq"
	usecases_redis "databasus-backend/internal/features/backups/backups/usecases/redis"
)

var createBackupUsecase = &CreateBackupUsecase{
	usecases_postgresql.GetCreatePostgresqlBackupUsecase(),
	usecases_redis.GetCreateRedisBackupUsecase(),
	usecases_rabbitmq.GetCreateRabbitmqBackupUsecase(),
	usecases_kubernetes.GetCreateKubernetesBackupUsecase(),
}

func GetCreateBackupUsecase() *CreateBackupUsecase {
	return createBackupUsecase
}

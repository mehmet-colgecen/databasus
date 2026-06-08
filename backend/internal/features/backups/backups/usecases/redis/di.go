package usecases_redis

import (
	encryption_secrets "databasus-backend/internal/features/encryption/secrets"
	"databasus-backend/internal/util/encryption"
	"databasus-backend/internal/util/logger"
)

var createRedisBackupUsecase = &CreateRedisBackupUsecase{
	logger.GetLogger(),
	encryption_secrets.GetSecretKeyService(),
	encryption.GetFieldEncryptor(),
}

func GetCreateRedisBackupUsecase() *CreateRedisBackupUsecase {
	return createRedisBackupUsecase
}

package usecases_rabbitmq

import (
	encryption_secrets "databasus-backend/internal/features/encryption/secrets"
	"databasus-backend/internal/util/encryption"
	"databasus-backend/internal/util/logger"
)

var createRabbitmqBackupUsecase = &CreateRabbitmqBackupUsecase{
	logger.GetLogger(),
	encryption_secrets.GetSecretKeyService(),
	encryption.GetFieldEncryptor(),
}

func GetCreateRabbitmqBackupUsecase() *CreateRabbitmqBackupUsecase {
	return createRabbitmqBackupUsecase
}

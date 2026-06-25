package usecases_kubernetes

import (
	encryption_secrets "databasus-backend/internal/features/encryption/secrets"
	"databasus-backend/internal/util/encryption"
	"databasus-backend/internal/util/logger"
)

var createKubernetesBackupUsecase = &CreateKubernetesBackupUsecase{
	logger.GetLogger(),
	encryption_secrets.GetSecretKeyService(),
	encryption.GetFieldEncryptor(),
}

func GetCreateKubernetesBackupUsecase() *CreateKubernetesBackupUsecase {
	return createKubernetesBackupUsecase
}

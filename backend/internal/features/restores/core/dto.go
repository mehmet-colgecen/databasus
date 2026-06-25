package restores_core

import (
	"databasus-backend/internal/features/databases/databases/postgresql"
)

type RestoreBackupRequest struct {
	PostgresqlDatabase *postgresql.PostgresqlDatabase `json:"postgresqlDatabase"`
}

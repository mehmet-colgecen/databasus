package databases

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"databasus-backend/internal/features/databases/databases/kubernetes"
	"databasus-backend/internal/features/databases/databases/postgresql"
	"databasus-backend/internal/features/databases/databases/rabbitmq"
	"databasus-backend/internal/features/databases/databases/redis"
	"databasus-backend/internal/features/notifiers"
	"databasus-backend/internal/util/encryption"
)

type Database struct {
	ID uuid.UUID `json:"id" gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`

	// WorkspaceID can be null when a database is created via restore operation
	// outside the context of any workspace
	WorkspaceID *uuid.UUID   `json:"workspaceId" gorm:"column:workspace_id;type:uuid"`
	Name        string       `json:"name"        gorm:"column:name;type:text;not null"`
	Type        DatabaseType `json:"type"        gorm:"column:type;type:text;not null"`

	Postgresql *postgresql.PostgresqlDatabase `json:"postgresql,omitzero" gorm:"foreignKey:DatabaseID"`
	Redis      *redis.RedisDatabase           `json:"redis,omitzero"      gorm:"foreignKey:DatabaseID"`
	Rabbitmq   *rabbitmq.RabbitmqDatabase     `json:"rabbitmq,omitzero"   gorm:"foreignKey:DatabaseID"`
	Kubernetes *kubernetes.KubernetesDatabase `json:"kubernetes,omitzero" gorm:"foreignKey:DatabaseID"`

	Notifiers []notifiers.Notifier `json:"notifiers" gorm:"many2many:database_notifiers;"`

	// these fields are not reliable, but
	// they are used for pretty UI
	LastBackupTime         *time.Time `json:"lastBackupTime,omitzero"          gorm:"column:last_backup_time;type:timestamp with time zone"`
	LastBackupErrorMessage *string    `json:"lastBackupErrorMessage,omitempty" gorm:"column:last_backup_error_message;type:text"`

	HealthStatus *HealthStatus `json:"healthStatus" gorm:"column:health_status;type:text;not null"`

	AgentToken            *string `json:"-"                     gorm:"column:agent_token;type:text"`
	IsAgentTokenGenerated bool    `json:"isAgentTokenGenerated" gorm:"column:is_agent_token_generated;not null;default:false"`
}

func (d *Database) Validate() error {
	if d.Name == "" {
		return errors.New("name is required")
	}

	switch d.Type {
	case DatabaseTypePostgres:
		if d.Postgresql == nil {
			return errors.New("postgresql database is required")
		}
		return d.Postgresql.Validate()
	case DatabaseTypeRedis:
		if d.Redis == nil {
			return errors.New("redis database is required")
		}
		return d.Redis.Validate()
	case DatabaseTypeRabbitmq:
		if d.Rabbitmq == nil {
			return errors.New("rabbitmq database is required")
		}
		return d.Rabbitmq.Validate()
	case DatabaseTypeKubernetes:
		if d.Kubernetes == nil {
			return errors.New("kubernetes database is required")
		}
		return d.Kubernetes.Validate()
	default:
		return errors.New("invalid database type: " + string(d.Type))
	}
}

func (d *Database) ValidateUpdate(old, new Database) error {
	// Database type cannot be changed after creation — the entire backup
	// structure (storage files, schedulers, WAL hierarchy, etc.) is tied to
	// the type at creation time. Recreating that state automatically is
	// error-prone; it is safer for the user to create a new database and
	// remove the old one.
	if old.Type != new.Type {
		return errors.New("database type cannot be changed; create a new database instead")
	}

	if old.Type == DatabaseTypePostgres && old.Postgresql != nil && new.Postgresql != nil {
		if err := new.Postgresql.ValidateUpdate(old.Postgresql); err != nil {
			return err
		}
	}

	return nil
}

func (d *Database) TestConnection(
	logger *slog.Logger,
	encryptor encryption.FieldEncryptor,
) error {
	return d.getSpecificDatabase().TestConnection(logger, encryptor)
}

func (d *Database) GetRawDbSizeMb(
	ctx context.Context,
	logger *slog.Logger,
	encryptor encryption.FieldEncryptor,
) (float64, error) {
	return d.getSpecificDatabase().GetRawDbSizeMb(ctx, logger, encryptor)
}

func (d *Database) IsUserReadOnly(
	ctx context.Context,
	logger *slog.Logger,
	encryptor encryption.FieldEncryptor,
) (bool, []string, error) {
	switch d.Type {
	case DatabaseTypePostgres:
		return d.Postgresql.IsUserReadOnly(ctx, logger, encryptor)
	default:
		return false, nil, errors.New("read-only check not supported for this database type")
	}
}

func (d *Database) HideSensitiveData() {
	d.getSpecificDatabase().HideSensitiveData()
}

func (d *Database) EncryptSensitiveFields(encryptor encryption.FieldEncryptor) error {
	if d.Postgresql != nil {
		return d.Postgresql.EncryptSensitiveFields(encryptor)
	}
	if d.Redis != nil {
		return d.Redis.EncryptSensitiveFields(encryptor)
	}
	if d.Rabbitmq != nil {
		return d.Rabbitmq.EncryptSensitiveFields(encryptor)
	}
	if d.Kubernetes != nil {
		return d.Kubernetes.EncryptSensitiveFields(encryptor)
	}
	return nil
}

func (d *Database) PopulateDbData(
	logger *slog.Logger,
	encryptor encryption.FieldEncryptor,
) error {
	if d.Postgresql != nil {
		return d.Postgresql.PopulateDbData(logger, encryptor)
	}
	if d.Redis != nil {
		return d.Redis.PopulateDbData(logger, encryptor)
	}
	if d.Rabbitmq != nil {
		return d.Rabbitmq.PopulateDbData(logger, encryptor)
	}
	if d.Kubernetes != nil {
		return d.Kubernetes.PopulateDbData(logger, encryptor)
	}
	return nil
}

func (d *Database) Update(incoming *Database) {
	d.Name = incoming.Name
	d.Type = incoming.Type
	d.Notifiers = incoming.Notifiers

	switch d.Type {
	case DatabaseTypePostgres:
		if d.Postgresql != nil && incoming.Postgresql != nil {
			d.Postgresql.Update(incoming.Postgresql)
		}
	case DatabaseTypeRedis:
		if d.Redis != nil && incoming.Redis != nil {
			d.Redis.Update(incoming.Redis)
		}
	case DatabaseTypeRabbitmq:
		if d.Rabbitmq != nil && incoming.Rabbitmq != nil {
			d.Rabbitmq.Update(incoming.Rabbitmq)
		}
	case DatabaseTypeKubernetes:
		if d.Kubernetes != nil && incoming.Kubernetes != nil {
			d.Kubernetes.Update(incoming.Kubernetes)
		}
	}
}

func (d *Database) IsAgentManagedBackup() bool {
	return d.Type == DatabaseTypePostgres &&
		d.Postgresql != nil &&
		d.Postgresql.BackupType == postgresql.PostgresBackupTypeWalV1
}

func (d *Database) getSpecificDatabase() DatabaseConnector {
	switch d.Type {
	case DatabaseTypePostgres:
		return d.Postgresql
	case DatabaseTypeRedis:
		return d.Redis
	case DatabaseTypeRabbitmq:
		return d.Rabbitmq
	case DatabaseTypeKubernetes:
		return d.Kubernetes
	}

	panic("invalid database type: " + string(d.Type))
}

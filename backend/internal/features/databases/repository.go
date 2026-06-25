package databases

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"databasus-backend/internal/features/databases/databases/kubernetes"
	"databasus-backend/internal/features/databases/databases/postgresql"
	"databasus-backend/internal/features/databases/databases/rabbitmq"
	"databasus-backend/internal/features/databases/databases/redis"
	"databasus-backend/internal/storage"
)

type DatabaseRepository struct{}

func (r *DatabaseRepository) Save(database *Database) (*Database, error) {
	db := storage.GetDb()

	isNew := database.ID == uuid.Nil
	if isNew {
		database.ID = uuid.New()
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		switch database.Type {
		case DatabaseTypePostgres:
			if database.Postgresql == nil {
				return errors.New("postgresql configuration is required for PostgreSQL database")
			}
			database.Postgresql.DatabaseID = &database.ID
		case DatabaseTypeRedis:
			if database.Redis == nil {
				return errors.New("redis configuration is required for Redis database")
			}
			database.Redis.DatabaseID = &database.ID
		case DatabaseTypeRabbitmq:
			if database.Rabbitmq == nil {
				return errors.New("rabbitmq configuration is required for RabbitMQ database")
			}
			database.Rabbitmq.DatabaseID = &database.ID
		case DatabaseTypeKubernetes:
			if database.Kubernetes == nil {
				return errors.New("kubernetes configuration is required for Kubernetes database")
			}
			database.Kubernetes.DatabaseID = &database.ID
		}

		if isNew {
			if err := tx.Create(database).
				Omit("Postgresql", "Redis", "Rabbitmq", "Kubernetes", "Notifiers").
				Error; err != nil {
				return err
			}
		} else {
			if err := tx.Save(database).
				Omit("Postgresql", "Redis", "Rabbitmq", "Kubernetes", "Notifiers").
				Error; err != nil {
				return err
			}
		}

		switch database.Type {
		case DatabaseTypePostgres:
			database.Postgresql.DatabaseID = &database.ID
			if database.Postgresql.ID == uuid.Nil {
				database.Postgresql.ID = uuid.New()
				if err := tx.Create(database.Postgresql).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Save(database.Postgresql).Error; err != nil {
					return err
				}
			}
		case DatabaseTypeRedis:
			database.Redis.DatabaseID = &database.ID
			if database.Redis.ID == uuid.Nil {
				database.Redis.ID = uuid.New()
				if err := tx.Create(database.Redis).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Save(database.Redis).Error; err != nil {
					return err
				}
			}
		case DatabaseTypeRabbitmq:
			database.Rabbitmq.DatabaseID = &database.ID
			if database.Rabbitmq.ID == uuid.Nil {
				database.Rabbitmq.ID = uuid.New()
				if err := tx.Create(database.Rabbitmq).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Save(database.Rabbitmq).Error; err != nil {
					return err
				}
			}
		case DatabaseTypeKubernetes:
			database.Kubernetes.DatabaseID = &database.ID
			if database.Kubernetes.ID == uuid.Nil {
				database.Kubernetes.ID = uuid.New()
				if err := tx.Create(database.Kubernetes).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Save(database.Kubernetes).Error; err != nil {
					return err
				}
			}
		}

		if err := tx.
			Model(database).
			Association("Notifiers").
			Replace(database.Notifiers); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return database, nil
}

func (r *DatabaseRepository) FindByID(id uuid.UUID) (*Database, error) {
	var database Database

	if err := storage.
		GetDb().
		Preload("Postgresql").
		Preload("Redis").
		Preload("Rabbitmq").
		Preload("Kubernetes").
		Preload("Notifiers").
		Where("id = ?", id).
		First(&database).Error; err != nil {
		return nil, err
	}

	return &database, nil
}

func (r *DatabaseRepository) FindByWorkspaceID(workspaceID uuid.UUID) ([]*Database, error) {
	var databases []*Database

	if err := storage.
		GetDb().
		Preload("Postgresql").
		Preload("Redis").
		Preload("Rabbitmq").
		Preload("Kubernetes").
		Preload("Notifiers").
		Where("workspace_id = ?", workspaceID).
		Order("CASE WHEN health_status = 'UNAVAILABLE' THEN 1 WHEN health_status = 'AVAILABLE' THEN 2 WHEN health_status IS NULL THEN 3 ELSE 4 END, name ASC").
		Find(&databases).Error; err != nil {
		return nil, err
	}

	return databases, nil
}

func (r *DatabaseRepository) Delete(id uuid.UUID) error {
	db := storage.GetDb()

	return db.Transaction(func(tx *gorm.DB) error {
		var database Database
		if err := tx.Where("id = ?", id).First(&database).Error; err != nil {
			return err
		}

		if err := tx.Model(&database).Association("Notifiers").Clear(); err != nil {
			return err
		}

		switch database.Type {
		case DatabaseTypePostgres:
			if err := tx.
				Where("database_id = ?", id).
				Delete(&postgresql.PostgresqlDatabase{}).Error; err != nil {
				return err
			}
		case DatabaseTypeRedis:
			if err := tx.
				Where("database_id = ?", id).
				Delete(&redis.RedisDatabase{}).Error; err != nil {
				return err
			}
		case DatabaseTypeRabbitmq:
			if err := tx.
				Where("database_id = ?", id).
				Delete(&rabbitmq.RabbitmqDatabase{}).Error; err != nil {
				return err
			}
		case DatabaseTypeKubernetes:
			if err := tx.
				Where("database_id = ?", id).
				Delete(&kubernetes.KubernetesDatabase{}).Error; err != nil {
				return err
			}
		}

		if err := tx.Delete(&Database{}, id).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *DatabaseRepository) IsNotifierUsing(notifierID uuid.UUID) (bool, error) {
	var count int64

	if err := storage.
		GetDb().
		Table("database_notifiers").
		Where("notifier_id = ?", notifierID).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *DatabaseRepository) GetAllDatabases() ([]*Database, error) {
	var databases []*Database

	if err := storage.
		GetDb().
		Preload("Postgresql").
		Preload("Redis").
		Preload("Rabbitmq").
		Preload("Kubernetes").
		Preload("Notifiers").
		Find(&databases).Error; err != nil {
		return nil, err
	}

	return databases, nil
}

func (r *DatabaseRepository) FindByAgentTokenHash(hash string) (*Database, error) {
	var database Database

	if err := storage.GetDb().
		Where("agent_token = ?", hash).
		First(&database).Error; err != nil {
		return nil, err
	}

	return &database, nil
}

func (r *DatabaseRepository) GetDatabasesIDsByNotifierID(
	notifierID uuid.UUID,
) ([]uuid.UUID, error) {
	var databasesIDs []uuid.UUID

	if err := storage.
		GetDb().
		Table("database_notifiers").
		Where("notifier_id = ?", notifierID).
		Pluck("database_id", &databasesIDs).Error; err != nil {
		return nil, err
	}

	return databasesIDs, nil
}

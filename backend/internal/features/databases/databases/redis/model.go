package redis

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"

	"databasus-backend/internal/util/encryption"
)

type RedisDatabase struct {
	ID         uuid.UUID  `json:"id"         gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	DatabaseID *uuid.UUID `json:"databaseId" gorm:"type:uuid;column:database_id"`

	Version string `json:"version" gorm:"type:text;not null;default:''"`

	Host     string `json:"host"     gorm:"type:text;not null"`
	Port     int    `json:"port"     gorm:"type:int;not null;default:6379"`
	Username string `json:"username" gorm:"type:text;not null;default:''"`
	Password string `json:"password" gorm:"type:text;not null"`
	IsTls    bool   `json:"isTls"    gorm:"column:is_tls;type:boolean;not null;default:false"`
}

func (r *RedisDatabase) TableName() string {
	return "redis_databases"
}

func (r *RedisDatabase) Validate() error {
	if r.Host == "" {
		return errors.New("host is required")
	}

	if r.Port <= 0 || r.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}

	if r.Password == "" {
		return errors.New("password is required")
	}

	return nil
}

func (r *RedisDatabase) TestConnection(
	logger *slog.Logger,
	encryptor encryption.FieldEncryptor,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := r.connect(ctx, encryptor)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Error("Failed to close Redis connection", "error", closeErr)
		}
	}()

	if err := conn.Ping(); err != nil {
		return fmt.Errorf("failed to ping Redis: %w", err)
	}

	version, err := r.detectVersion(conn)
	if err != nil {
		return err
	}
	r.Version = version

	return nil
}

func (r *RedisDatabase) GetRawDbSizeMb(
	ctx context.Context,
	logger *slog.Logger,
	encryptor encryption.FieldEncryptor,
) (float64, error) {
	conn, err := r.connect(ctx, encryptor)
	if err != nil {
		return 0, err
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Error("Failed to close Redis connection", "error", closeErr)
		}
	}()

	info, err := conn.Info("memory")
	if err != nil {
		return 0, fmt.Errorf("failed to read Redis memory info: %w", err)
	}

	usedMemory := ParseInfoField(info, "used_memory")
	if usedMemory == "" {
		return 0, nil
	}

	bytesUsed, err := strconv.ParseFloat(usedMemory, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse used_memory '%s': %w", usedMemory, err)
	}

	return bytesUsed / (1024 * 1024), nil
}

func (r *RedisDatabase) HideSensitiveData() {
	if r == nil {
		return
	}
	r.Password = ""
}

func (r *RedisDatabase) Update(incoming *RedisDatabase) {
	r.Version = incoming.Version
	r.Host = incoming.Host
	r.Port = incoming.Port
	r.Username = incoming.Username
	r.IsTls = incoming.IsTls

	if incoming.Password != "" {
		r.Password = incoming.Password
	}
}

func (r *RedisDatabase) EncryptSensitiveFields(
	encryptor encryption.FieldEncryptor,
) error {
	if r.Password != "" {
		encrypted, err := encryptor.Encrypt(r.Password)
		if err != nil {
			return err
		}
		r.Password = encrypted
	}

	return nil
}

func (r *RedisDatabase) PopulateDbData(
	logger *slog.Logger,
	encryptor encryption.FieldEncryptor,
) error {
	return r.PopulateVersion(logger, encryptor)
}

func (r *RedisDatabase) PopulateVersion(
	logger *slog.Logger,
	encryptor encryption.FieldEncryptor,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := r.connect(ctx, encryptor)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Error("Failed to close Redis connection", "error", closeErr)
		}
	}()

	version, err := r.detectVersion(conn)
	if err != nil {
		return err
	}
	r.Version = version

	return nil
}

// OpenSyncStream connects, authenticates and issues SYNC, returning a reader
// over the RDB snapshot together with the connection to close once the backup
// is complete.
func (r *RedisDatabase) OpenSyncStream(
	ctx context.Context,
	encryptor encryption.FieldEncryptor,
) (io.Reader, io.Closer, error) {
	conn, err := r.connect(ctx, encryptor)
	if err != nil {
		return nil, nil, err
	}

	reader, err := conn.StartSync()
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("failed to start Redis SYNC: %w", err)
	}

	return reader, conn, nil
}

func (r *RedisDatabase) connect(
	ctx context.Context,
	encryptor encryption.FieldEncryptor,
) (*Conn, error) {
	password, err := decryptPasswordIfNeeded(r.Password, encryptor)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt password: %w", err)
	}

	conn, err := DialContext(ctx, r.Host, r.Port, r.IsTls)
	if err != nil {
		return nil, err
	}

	if err := conn.Auth(r.Username, password); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to authenticate with Redis: %w", err)
	}

	return conn, nil
}

func (r *RedisDatabase) detectVersion(conn *Conn) (string, error) {
	info, err := conn.Info("server")
	if err != nil {
		return "", fmt.Errorf("failed to read Redis server info: %w", err)
	}

	version := ParseInfoField(info, "redis_version")
	if version == "" {
		return "", errors.New("could not detect Redis version")
	}

	return version, nil
}

func decryptPasswordIfNeeded(
	password string,
	encryptor encryption.FieldEncryptor,
) (string, error) {
	if encryptor == nil {
		return password, nil
	}

	return encryptor.Decrypt(password)
}

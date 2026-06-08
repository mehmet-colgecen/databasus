package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"databasus-backend/internal/util/encryption"
)

const errorBodyLimit = 64 * 1024

type RabbitmqDatabase struct {
	ID         uuid.UUID  `json:"id"         gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	DatabaseID *uuid.UUID `json:"databaseId" gorm:"type:uuid;column:database_id"`

	Version string `json:"version" gorm:"type:text;not null;default:''"`

	Host           string `json:"host"           gorm:"type:text;not null"`
	ManagementPort int    `json:"managementPort" gorm:"column:management_port;type:int;not null;default:15672"`
	Username       string `json:"username"       gorm:"type:text;not null"`
	Password       string `json:"password"       gorm:"type:text;not null"`
	IsHttps        bool   `json:"isHttps"        gorm:"column:is_https;type:boolean;not null;default:false"`
}

func (r *RabbitmqDatabase) TableName() string {
	return "rabbitmq_databases"
}

func (r *RabbitmqDatabase) Validate() error {
	if r.Host == "" {
		return errors.New("host is required")
	}

	if r.ManagementPort <= 0 || r.ManagementPort > 65535 {
		return errors.New("management port must be between 1 and 65535")
	}

	if r.Username == "" {
		return errors.New("username is required")
	}

	if r.Password == "" {
		return errors.New("password is required")
	}

	return nil
}

func (r *RabbitmqDatabase) TestConnection(
	logger *slog.Logger,
	encryptor encryption.FieldEncryptor,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	version, err := r.fetchVersion(ctx, encryptor)
	if err != nil {
		return err
	}
	r.Version = version

	return nil
}

// GetRawDbSizeMb returns 0: a RabbitMQ backup captures definitions (topology
// and configuration), which are kilobytes of JSON rather than a dataset. A
// non-zero raw size would distort billing semantics shared with the other
// database types.
func (r *RabbitmqDatabase) GetRawDbSizeMb(
	_ context.Context,
	_ *slog.Logger,
	_ encryption.FieldEncryptor,
) (float64, error) {
	return 0, nil
}

func (r *RabbitmqDatabase) HideSensitiveData() {
	if r == nil {
		return
	}
	r.Password = ""
}

func (r *RabbitmqDatabase) Update(incoming *RabbitmqDatabase) {
	r.Version = incoming.Version
	r.Host = incoming.Host
	r.ManagementPort = incoming.ManagementPort
	r.Username = incoming.Username
	r.IsHttps = incoming.IsHttps

	if incoming.Password != "" {
		r.Password = incoming.Password
	}
}

func (r *RabbitmqDatabase) EncryptSensitiveFields(
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

func (r *RabbitmqDatabase) PopulateDbData(
	logger *slog.Logger,
	encryptor encryption.FieldEncryptor,
) error {
	return r.PopulateVersion(logger, encryptor)
}

func (r *RabbitmqDatabase) PopulateVersion(
	_ *slog.Logger,
	encryptor encryption.FieldEncryptor,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	version, err := r.fetchVersion(ctx, encryptor)
	if err != nil {
		return err
	}
	r.Version = version

	return nil
}

// OpenDefinitionsStream performs an authenticated GET /api/definitions and
// returns the JSON response body for streaming. The caller closes the returned
// closer (the response body) once the backup is complete.
func (r *RabbitmqDatabase) OpenDefinitionsStream(
	ctx context.Context,
	encryptor encryption.FieldEncryptor,
) (io.Reader, io.Closer, error) {
	password, err := decryptPasswordIfNeeded(r.Password, encryptor)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decrypt password: %w", err)
	}

	client := NewHTTPClient(r.IsHttps)

	response, err := r.Get(ctx, client, "/api/definitions", password)
	if err != nil {
		return nil, nil, err
	}

	if response.StatusCode != http.StatusOK {
		defer func() { _ = response.Body.Close() }()
		return nil, nil, describeErrorResponse(response)
	}

	return response.Body, response.Body, nil
}

func (r *RabbitmqDatabase) fetchVersion(
	ctx context.Context,
	encryptor encryption.FieldEncryptor,
) (string, error) {
	password, err := decryptPasswordIfNeeded(r.Password, encryptor)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt password: %w", err)
	}

	client := NewHTTPClient(r.IsHttps)

	response, err := r.Get(ctx, client, "/api/overview", password)
	if err != nil {
		return "", err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return "", describeErrorResponse(response)
	}

	var overview struct {
		RabbitmqVersion string `json:"rabbitmq_version"`
		ProductVersion  string `json:"product_version"`
	}
	if err := json.NewDecoder(response.Body).Decode(&overview); err != nil {
		return "", fmt.Errorf("failed to decode RabbitMQ overview: %w", err)
	}

	if overview.RabbitmqVersion != "" {
		return overview.RabbitmqVersion, nil
	}

	return overview.ProductVersion, nil
}

func describeErrorResponse(response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, errorBodyLimit))

	if response.StatusCode == http.StatusUnauthorized {
		return errors.New("authentication failed: invalid RabbitMQ username or password")
	}

	return fmt.Errorf(
		"RabbitMQ management API returned status %d: %s",
		response.StatusCode,
		string(body),
	)
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

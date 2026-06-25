package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"databasus-backend/internal/config"
	backups_core "databasus-backend/internal/features/backups/backups/core"
	backups_config "databasus-backend/internal/features/backups/config"
	"databasus-backend/internal/features/databases"
	pgtypes "databasus-backend/internal/features/databases/databases/postgresql"
	restores_core "databasus-backend/internal/features/restores/core"
	"databasus-backend/internal/features/storages"
	users_enums "databasus-backend/internal/features/users/enums"
	users_testing "databasus-backend/internal/features/users/testing"
	workspaces_testing "databasus-backend/internal/features/workspaces/testing"
	test_utils "databasus-backend/internal/util/testing"
)

func Test_BackupAndRestorePostgresqlSSL_Succeeds(t *testing.T) {
	port := config.GetEnv().TestPostgresSslPort
	if port == "" {
		t.Skip("TEST_POSTGRES_SSL_PORT not configured")
	}

	host := config.GetEnv().TestLocalhost
	portInt, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("failed to parse SSL port: %v", err)
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=testuser password=testpassword dbname=testdb sslmode=require",
		host, portInt,
	)
	originalDB, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to connect to SSL Postgres: %v", err)
	}
	defer originalDB.Close()

	tableName := "test_data_ssl"
	_, err = originalDB.Exec(createAndFillTableQuery(tableName))
	assert.NoError(t, err)

	router := createTestRouter()
	user := users_testing.CreateTestUser(users_enums.UserRoleMember)
	workspace := workspaces_testing.CreateTestWorkspace("Postgres SSL Workspace", user, router)
	storage := storages.CreateTestStorage(workspace.ID)

	dbName := "testdb"
	database := createPostgresqlSSLDatabaseViaAPI(
		t, router, "Postgres SSL DB", workspace.ID,
		host, portInt, "testuser", "testpassword", dbName, user.Token,
	)

	enableBackupsViaAPI(
		t, router, database.ID, storage.ID,
		backups_config.BackupEncryptionNone, user.Token,
	)
	createBackupViaAPI(t, router, database.ID, user.Token)

	backup := waitForBackupCompletion(t, router, database.ID, user.Token, 5*time.Minute)
	assert.Equal(t, backups_core.BackupStatusCompleted, backup.Status)

	newDBName := "restoreddb_pg_ssl"
	_, err = originalDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s;", newDBName))
	assert.NoError(t, err)
	_, err = originalDB.Exec(fmt.Sprintf("CREATE DATABASE %s;", newDBName))
	assert.NoError(t, err)

	newDSN := fmt.Sprintf(
		"host=%s port=%d user=testuser password=testpassword dbname=%s sslmode=require",
		host, portInt, newDBName,
	)
	newDB, err := sqlx.Connect("postgres", newDSN)
	assert.NoError(t, err)
	defer newDB.Close()

	createPostgresqlSSLRestoreViaAPI(
		t, router, backup.ID,
		host, portInt, "testuser", "testpassword", newDBName, user.Token,
	)

	restore := waitForRestoreCompletion(t, router, backup.ID, user.Token, 5*time.Minute)
	assert.Equal(t, restores_core.RestoreStatusCompleted, restore.Status)

	verifyDataIntegrity(t, originalDB, newDB, tableName)

	_ = os.Remove(filepath.Join(config.GetEnv().DataFolder, backup.ID.String()))
	test_utils.MakeDeleteRequest(
		t, router, "/api/v1/databases/"+database.ID.String(),
		"Bearer "+user.Token, http.StatusNoContent,
	)
	storages.RemoveTestStorage(storage.ID)
	workspaces_testing.RemoveTestWorkspace(workspace, router)
}

func createPostgresqlSSLDatabaseViaAPI(
	t *testing.T,
	router *gin.Engine,
	name string,
	workspaceID uuid.UUID,
	host string,
	port int,
	username, password, database, token string,
) *databases.Database {
	request := databases.Database{
		Name:        name,
		WorkspaceID: &workspaceID,
		Type:        databases.DatabaseTypePostgres,
		Postgresql: &pgtypes.PostgresqlDatabase{
			Host:     host,
			Port:     port,
			Username: username,
			Password: password,
			Database: &database,
			SslMode:  pgtypes.PostgresSslModeRequire,
			CpuCount: 1,
		},
	}

	return submitCreateDatabase(t, router, "Postgres SSL", request, token)
}

func createPostgresqlSSLRestoreViaAPI(
	t *testing.T,
	router *gin.Engine,
	backupID uuid.UUID,
	host string,
	port int,
	username, password, database, token string,
) {
	request := restores_core.RestoreBackupRequest{
		PostgresqlDatabase: &pgtypes.PostgresqlDatabase{
			Host:     host,
			Port:     port,
			Username: username,
			Password: password,
			Database: &database,
			SslMode:  pgtypes.PostgresSslModeRequire,
			CpuCount: 1,
		},
	}
	submitRestore(t, router, backupID, request, token)
}

func submitCreateDatabase(
	t *testing.T,
	router *gin.Engine,
	label string,
	request databases.Database,
	token string,
) *databases.Database {
	w := workspaces_testing.MakeAPIRequest(
		router, "POST", "/api/v1/databases/create", "Bearer "+token, request,
	)
	if w.Code != http.StatusCreated {
		t.Fatalf("Failed to create %s database. Status: %d, Body: %s",
			label, w.Code, w.Body.String())
	}

	var created databases.Database
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("Failed to unmarshal %s response: %v", label, err)
	}
	return &created
}

func submitRestore(
	t *testing.T,
	router *gin.Engine,
	backupID uuid.UUID,
	request restores_core.RestoreBackupRequest,
	token string,
) {
	test_utils.MakePostRequest(
		t, router,
		fmt.Sprintf("/api/v1/restores/%s/restore", backupID.String()),
		"Bearer "+token, request, http.StatusOK,
	)
}

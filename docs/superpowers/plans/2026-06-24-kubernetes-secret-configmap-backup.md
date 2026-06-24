# Kubernetes Secret & ConfigMap Backup — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `KUBERNETES` source type that backs up cluster Secrets and/or ConfigMaps as a sanitized multi-document YAML file streamed to any configured storage, using the backend's own in-cluster ServiceAccount, with read-only RBAC shipped in the Helm chart.

**Architecture:** Mirror the Redis/RabbitMQ "non-DB source" pattern already on `feature/redis-rabbitmq-backup-support`: a `DatabaseType` + a `kubernetes_databases` table + a server-side backup usecase that produces an `io.Reader` and hands it to the shared `streaming.Run` pipeline (encryption → counting → storage). Authentication is in-cluster only (`rest.InClusterConfig`). No external agent, no automated restore, no verify.

**Tech Stack:** Go 1.x, Gin, GORM, PostgreSQL (goose migrations), `k8s.io/client-go` + `k8s.io/api` + `k8s.io/apimachinery` (new deps) + `sigs.k8s.io/yaml` (transitive, for k8s-object YAML), React 19 + TypeScript + Ant Design + Tailwind, Helm.

## Global Constraints

- **Language:** English only in all code, comments, identifiers, log messages, API strings, test assertions, commit messages.
- **Base branch:** `feature/kubernetes-backup-support` (already checked out, branched off `feature/redis-rabbitmq-backup-support`). Do NOT branch off `main`.
- **Naming:** intent-revealing names; booleans prefixed `is/has/can/should`; getters `Get...`; no generic `Manager`/`data`/`helper`.
- **No backward-compat shims** and **no "how it was" comments** — current state only.
- **Lint/format after each change:** backend → `cd backend && make lint`; frontend → `cd frontend && pnpm lint && pnpm format`.
- **Tests:** prefer controller/package tests over micro-unit tests; CI has no live Kubernetes cluster, so all backend tests must pass without one (use `k8s.io/client-go/kubernetes/fake`).
- **Source type values (verbatim):** backend enum `KUBERNETES`; resource-type values `SECRET`, `CONFIGMAP`; namespace-scope values `ALL`, `SPECIFIC`.
- **Security:** read-only RBAC (`get`/`list` only); never log secret contents.

---

## File Structure

**Backend — new files**
- `backend/internal/features/databases/databases/kubernetes/enums.go` — resource-type + namespace-scope enums.
- `backend/internal/features/databases/databases/kubernetes/model.go` — `KubernetesDatabase` GORM model + `DatabaseConnector` methods + GORM hooks.
- `backend/internal/features/databases/databases/kubernetes/client.go` — in-cluster clientset, version detection, access check, namespace resolution.
- `backend/internal/features/databases/databases/kubernetes/sanitize.go` — pure sanitizer for `metav1.Object` + per-type TypeMeta setters.
- `backend/internal/features/databases/databases/kubernetes/sanitize_test.go` — sanitizer unit tests.
- `backend/internal/features/databases/databases/kubernetes/export.go` — `OpenExportStream` + injectable `streamExport`.
- `backend/internal/features/databases/databases/kubernetes/export_test.go` — fake-clientset export tests.
- `backend/internal/features/backups/backups/usecases/kubernetes/create_backup_uc.go` — backup usecase.
- `backend/internal/features/backups/backups/usecases/kubernetes/di.go` — usecase singleton.
- `backend/migrations/20260624120000_create_kubernetes_databases_table.sql` — table.

**Backend — modified**
- `backend/internal/features/databases/enums.go` — add `DatabaseTypeKubernetes`.
- `backend/internal/features/databases/model.go` — field + switch cases.
- `backend/internal/features/databases/repository.go` — Save/Preload/Delete cases.
- `backend/internal/features/databases/service.go` — `CopyDatabase` case.
- `backend/internal/features/databases/controller_test.go` — validation test + helper.
- `backend/internal/features/backups/backups/usecases/di.go` — register usecase.
- `backend/internal/features/backups/backups/usecases/create_backup_uc.go` — dispatch case.
- `backend/go.mod` / `backend/go.sum` — client-go deps.

**Frontend — new files**
- `frontend/src/entity/databases/model/kubernetes/KubernetesDatabase.ts`
- `frontend/src/entity/databases/model/kubernetes/KubernetesResourceType.ts`
- `frontend/public/icons/databases/kubernetes.svg`
- `frontend/src/features/databases/ui/edit/EditKubernetesSpecificDataComponent.tsx`
- `frontend/src/features/databases/ui/show/ShowKubernetesSpecificDataComponent.tsx`

**Frontend — modified**
- `frontend/src/entity/databases/model/DatabaseType.ts`
- `frontend/src/entity/databases/model/Database.ts`
- `frontend/src/entity/databases/model/getDatabaseLogoFromType.ts`
- `frontend/src/entity/databases/index.ts`
- `frontend/src/features/databases/ui/edit/EditDatabaseBaseInfoComponent.tsx`
- `frontend/src/features/databases/ui/edit/EditDatabaseSpecificDataComponent.tsx`
- `frontend/src/features/databases/ui/show/ShowDatabaseSpecificDataComponent.tsx`
- `frontend/src/features/backups/ui/BackupsComponent.tsx`

**Helm — new files**
- `deploy/helm/templates/serviceaccount.yaml`
- `deploy/helm/templates/clusterrole.yaml`
- `deploy/helm/templates/clusterrolebinding.yaml`

**Helm — modified**
- `deploy/helm/values.yaml` — `serviceAccount` + `rbac` blocks.
- `deploy/helm/templates/statefulset.yaml` — `serviceAccountName`.

---

## Task 1: Backend dependencies, enum, and migration

**Files:**
- Modify: `backend/go.mod`, `backend/go.sum`
- Modify: `backend/internal/features/databases/enums.go`
- Create: `backend/migrations/20260624120000_create_kubernetes_databases_table.sql`

**Interfaces:**
- Produces: `databases.DatabaseTypeKubernetes DatabaseType = "KUBERNETES"`; table `kubernetes_databases`.

- [ ] **Step 1: Add client-go dependencies**

Run (from `backend/`):
```bash
cd backend && go get k8s.io/client-go@v0.31.0 k8s.io/api@v0.31.0 k8s.io/apimachinery@v0.31.0 && go mod tidy
```
Expected: `go.mod` gains `k8s.io/client-go`, `k8s.io/api`, `k8s.io/apimachinery` (and `sigs.k8s.io/yaml` transitively). If `v0.31.0` is unavailable, use the latest `v0.31.x`/`v0.32.x` patch and keep the three versions identical.

- [ ] **Step 2: Add the enum constant**

In `backend/internal/features/databases/enums.go`, add to the `const` block:
```go
	DatabaseTypeKubernetes DatabaseType = "KUBERNETES"
```

- [ ] **Step 3: Create the migration**

Create `backend/migrations/20260624120000_create_kubernetes_databases_table.sql`:
```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE kubernetes_databases (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id     UUID REFERENCES databases (id) ON DELETE CASCADE,
    version         TEXT NOT NULL DEFAULT '',
    resource_types  TEXT NOT NULL DEFAULT '',
    namespace_scope TEXT NOT NULL DEFAULT 'ALL',
    namespaces      TEXT NOT NULL DEFAULT '',
    object_names    TEXT NOT NULL DEFAULT ''
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_kubernetes_databases_database_id ON kubernetes_databases (database_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_kubernetes_databases_database_id;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS kubernetes_databases;
-- +goose StatementEnd
```

- [ ] **Step 4: Verify build**

Run: `cd backend && go build ./...`
Expected: compiles (the new enum is unused for now — that is fine, constants do not trigger unused errors).

- [ ] **Step 5: Commit**

```bash
git add backend/go.mod backend/go.sum backend/internal/features/databases/enums.go backend/migrations/20260624120000_create_kubernetes_databases_table.sql
git commit -m "FEATURE (kubernetes): add client-go deps, KUBERNETES enum, and migration"
```

---

## Task 2: KubernetesDatabase model, enums, and validation

**Files:**
- Create: `backend/internal/features/databases/databases/kubernetes/enums.go`
- Create: `backend/internal/features/databases/databases/kubernetes/model.go`
- Test: (validation + GORM hook tests live in) `backend/internal/features/databases/databases/kubernetes/model_test.go`

**Interfaces:**
- Consumes: `encryption.FieldEncryptor` (from `databasus-backend/internal/util/encryption`).
- Produces:
  - `kubernetes.KubernetesResourceType` (`KubernetesResourceTypeSecret = "SECRET"`, `KubernetesResourceTypeConfigMap = "CONFIGMAP"`).
  - `kubernetes.KubernetesNamespaceScope` (`KubernetesNamespaceScopeAll = "ALL"`, `KubernetesNamespaceScopeSpecific = "SPECIFIC"`).
  - `kubernetes.KubernetesDatabase` struct with fields:
    `ID uuid.UUID`, `DatabaseID *uuid.UUID`, `Version string`,
    `ResourceTypes []string` (gorm:"-"), `ResourceTypesString string` (column `resource_types`),
    `NamespaceScope string` (column `namespace_scope`),
    `Namespaces []string` (gorm:"-"), `NamespacesString string` (column `namespaces`),
    `ObjectNames []string` (gorm:"-"), `ObjectNamesString string` (column `object_names`).
  - Methods: `TableName()`, `Validate()`, `Update(*KubernetesDatabase)`, `HideSensitiveData()`, `EncryptSensitiveFields(encryption.FieldEncryptor) error`, `GetRawDbSizeMb(context.Context, *slog.Logger, encryption.FieldEncryptor) (float64, error)`, `BeforeSave(*gorm.DB) error`, `AfterFind(*gorm.DB) error`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/features/databases/databases/kubernetes/model_test.go`:
```go
package kubernetes

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Validate(t *testing.T) {
	testCases := []struct {
		name      string
		db        KubernetesDatabase
		wantError bool
	}{
		{
			name:      "no resource types is invalid",
			db:        KubernetesDatabase{NamespaceScope: string(KubernetesNamespaceScopeAll)},
			wantError: true,
		},
		{
			name: "all-scope with one resource type is valid",
			db: KubernetesDatabase{
				ResourceTypes:  []string{string(KubernetesResourceTypeSecret)},
				NamespaceScope: string(KubernetesNamespaceScopeAll),
			},
			wantError: false,
		},
		{
			name: "specific-scope without namespaces is invalid",
			db: KubernetesDatabase{
				ResourceTypes:  []string{string(KubernetesResourceTypeConfigMap)},
				NamespaceScope: string(KubernetesNamespaceScopeSpecific),
			},
			wantError: true,
		},
		{
			name: "specific-scope with namespaces is valid",
			db: KubernetesDatabase{
				ResourceTypes:  []string{string(KubernetesResourceTypeConfigMap)},
				NamespaceScope: string(KubernetesNamespaceScopeSpecific),
				Namespaces:     []string{"prod"},
			},
			wantError: false,
		},
		{
			name: "unknown resource type is invalid",
			db: KubernetesDatabase{
				ResourceTypes:  []string{"PODS"},
				NamespaceScope: string(KubernetesNamespaceScopeAll),
			},
			wantError: true,
		},
		{
			name: "unknown namespace scope is invalid",
			db: KubernetesDatabase{
				ResourceTypes:  []string{string(KubernetesResourceTypeSecret)},
				NamespaceScope: "CLUSTER",
			},
			wantError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.db.Validate()
			if tc.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_BeforeSaveAfterFind_RoundTripsListColumns(t *testing.T) {
	db := KubernetesDatabase{
		ResourceTypes: []string{"SECRET", "CONFIGMAP"},
		Namespaces:    []string{"prod", "staging"},
		ObjectNames:   []string{"app-config"},
	}

	assert.NoError(t, db.BeforeSave(nil))
	assert.Equal(t, "SECRET,CONFIGMAP", db.ResourceTypesString)
	assert.Equal(t, "prod,staging", db.NamespacesString)
	assert.Equal(t, "app-config", db.ObjectNamesString)

	loaded := KubernetesDatabase{
		ResourceTypesString: "SECRET,CONFIGMAP",
		NamespacesString:    "prod,staging",
		ObjectNamesString:   "app-config",
	}
	assert.NoError(t, loaded.AfterFind(nil))
	assert.Equal(t, []string{"SECRET", "CONFIGMAP"}, loaded.ResourceTypes)
	assert.Equal(t, []string{"prod", "staging"}, loaded.Namespaces)
	assert.Equal(t, []string{"app-config"}, loaded.ObjectNames)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/features/databases/databases/kubernetes/ -run 'Test_Validate|Test_BeforeSaveAfterFind' -v`
Expected: FAIL — package/types undefined (build error).

- [ ] **Step 3: Create the enums**

Create `backend/internal/features/databases/databases/kubernetes/enums.go`:
```go
package kubernetes

type KubernetesResourceType string

const (
	KubernetesResourceTypeSecret    KubernetesResourceType = "SECRET"
	KubernetesResourceTypeConfigMap KubernetesResourceType = "CONFIGMAP"
)

type KubernetesNamespaceScope string

const (
	KubernetesNamespaceScopeAll      KubernetesNamespaceScope = "ALL"
	KubernetesNamespaceScopeSpecific KubernetesNamespaceScope = "SPECIFIC"
)
```

- [ ] **Step 4: Create the model**

Create `backend/internal/features/databases/databases/kubernetes/model.go`:
```go
package kubernetes

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"databasus-backend/internal/util/encryption"
)

type KubernetesDatabase struct {
	ID         uuid.UUID  `json:"id"         gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	DatabaseID *uuid.UUID `json:"databaseId" gorm:"type:uuid;column:database_id"`

	Version string `json:"version" gorm:"type:text;not null;default:''"`

	ResourceTypes       []string `json:"resourceTypes" gorm:"-"`
	ResourceTypesString string   `json:"-"             gorm:"column:resource_types;type:text;not null;default:''"`

	NamespaceScope string `json:"namespaceScope" gorm:"column:namespace_scope;type:text;not null;default:'ALL'"`

	Namespaces       []string `json:"namespaces" gorm:"-"`
	NamespacesString string   `json:"-"          gorm:"column:namespaces;type:text;not null;default:''"`

	ObjectNames       []string `json:"objectNames" gorm:"-"`
	ObjectNamesString string   `json:"-"           gorm:"column:object_names;type:text;not null;default:''"`
}

func (k *KubernetesDatabase) TableName() string {
	return "kubernetes_databases"
}

func (k *KubernetesDatabase) BeforeSave(_ *gorm.DB) error {
	k.ResourceTypesString = strings.Join(k.ResourceTypes, ",")
	k.NamespacesString = strings.Join(k.Namespaces, ",")
	k.ObjectNamesString = strings.Join(k.ObjectNames, ",")
	return nil
}

func (k *KubernetesDatabase) AfterFind(_ *gorm.DB) error {
	k.ResourceTypes = splitNonEmpty(k.ResourceTypesString)
	k.Namespaces = splitNonEmpty(k.NamespacesString)
	k.ObjectNames = splitNonEmpty(k.ObjectNamesString)
	return nil
}

func splitNonEmpty(value string) []string {
	if value == "" {
		return []string{}
	}
	return strings.Split(value, ",")
}

func (k *KubernetesDatabase) Validate() error {
	if len(k.ResourceTypes) == 0 {
		return errors.New("at least one resource type is required")
	}

	for _, resourceType := range k.ResourceTypes {
		switch KubernetesResourceType(resourceType) {
		case KubernetesResourceTypeSecret, KubernetesResourceTypeConfigMap:
		default:
			return errors.New("invalid resource type: " + resourceType)
		}
	}

	switch KubernetesNamespaceScope(k.NamespaceScope) {
	case KubernetesNamespaceScopeAll:
	case KubernetesNamespaceScopeSpecific:
		if len(k.Namespaces) == 0 {
			return errors.New("at least one namespace is required when namespace scope is SPECIFIC")
		}
	default:
		return errors.New("invalid namespace scope: " + k.NamespaceScope)
	}

	return nil
}

func (k *KubernetesDatabase) Update(incoming *KubernetesDatabase) {
	k.Version = incoming.Version
	k.ResourceTypes = incoming.ResourceTypes
	k.NamespaceScope = incoming.NamespaceScope
	k.Namespaces = incoming.Namespaces
	k.ObjectNames = incoming.ObjectNames
}

// HideSensitiveData is a no-op: the configuration holds no credentials
// (authentication is the backend's in-cluster ServiceAccount).
func (k *KubernetesDatabase) HideSensitiveData() {}

// EncryptSensitiveFields is a no-op for the same reason as HideSensitiveData.
func (k *KubernetesDatabase) EncryptSensitiveFields(_ encryption.FieldEncryptor) error {
	return nil
}

// GetRawDbSizeMb returns 0: Secrets/ConfigMaps are configuration, not a dataset.
// The real artifact size is recorded by the streaming counting writer.
func (k *KubernetesDatabase) GetRawDbSizeMb(
	_ context.Context,
	_ *slog.Logger,
	_ encryption.FieldEncryptor,
) (float64, error) {
	return 0, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd backend && go test ./internal/features/databases/databases/kubernetes/ -run 'Test_Validate|Test_BeforeSaveAfterFind' -v`
Expected: PASS.

- [ ] **Step 6: Lint and commit**

```bash
cd backend && make lint && cd ..
git add backend/internal/features/databases/databases/kubernetes/enums.go backend/internal/features/databases/databases/kubernetes/model.go backend/internal/features/databases/databases/kubernetes/model_test.go
git commit -m "FEATURE (kubernetes): add KubernetesDatabase model with validation and list-column hooks"
```

---

## Task 3: In-cluster client, version detection, and connection test

**Files:**
- Create: `backend/internal/features/databases/databases/kubernetes/client.go`
- Modify: `backend/internal/features/databases/databases/kubernetes/model.go` (add `TestConnection`, `PopulateDbData`, `PopulateVersion`)

**Interfaces:**
- Consumes: `k8s.io/client-go/kubernetes.Interface`, `k8s.io/client-go/rest`.
- Produces:
  - `buildInClusterClientset() (kubernetes.Interface, error)`
  - `detectServerVersion(ctx context.Context, clientset kubernetes.Interface) (string, error)`
  - `verifyReadAccess(ctx context.Context, clientset kubernetes.Interface, db *KubernetesDatabase) error`
  - `resolveNamespaces(ctx context.Context, clientset kubernetes.Interface, db *KubernetesDatabase) ([]string, error)`
  - `(*KubernetesDatabase).TestConnection(logger, encryptor) error`
  - `(*KubernetesDatabase).PopulateDbData(logger, encryptor) error`

- [ ] **Step 1: Create the client helpers**

Create `backend/internal/features/databases/databases/kubernetes/client.go`:
```go
package kubernetes

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func buildInClusterClientset() (kubernetes.Interface, error) {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build in-cluster config (is Databasus running inside Kubernetes?): %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build Kubernetes client: %w", err)
	}

	return clientset, nil
}

func detectServerVersion(_ context.Context, clientset kubernetes.Interface) (string, error) {
	versionInfo, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to read Kubernetes server version: %w", err)
	}
	return versionInfo.GitVersion, nil
}

// verifyReadAccess confirms the ServiceAccount can list each selected resource
// type within the configured namespace scope by issuing a 1-item list.
func verifyReadAccess(
	ctx context.Context,
	clientset kubernetes.Interface,
	db *KubernetesDatabase,
) error {
	namespaces, err := resolveNamespaces(ctx, clientset, db)
	if err != nil {
		return err
	}

	listOptions := metav1.ListOptions{Limit: 1}

	for _, namespace := range namespaces {
		for _, resourceType := range db.ResourceTypes {
			switch KubernetesResourceType(resourceType) {
			case KubernetesResourceTypeSecret:
				if _, err := clientset.CoreV1().Secrets(namespace).List(ctx, listOptions); err != nil {
					return fmt.Errorf("cannot list secrets (namespace %q): %w", namespace, err)
				}
			case KubernetesResourceTypeConfigMap:
				if _, err := clientset.CoreV1().ConfigMaps(namespace).List(ctx, listOptions); err != nil {
					return fmt.Errorf("cannot list configmaps (namespace %q): %w", namespace, err)
				}
			}
		}
	}

	return nil
}

// resolveNamespaces returns the namespaces to operate on. For ALL scope it
// returns a single empty string, which client-go treats as "all namespaces"
// (requires the cluster-wide ClusterRole). For SPECIFIC scope it returns the
// configured list.
func resolveNamespaces(
	_ context.Context,
	_ kubernetes.Interface,
	db *KubernetesDatabase,
) ([]string, error) {
	switch KubernetesNamespaceScope(db.NamespaceScope) {
	case KubernetesNamespaceScopeAll:
		return []string{metav1.NamespaceAll}, nil
	case KubernetesNamespaceScopeSpecific:
		if len(db.Namespaces) == 0 {
			return nil, errors.New("no namespaces configured for SPECIFIC scope")
		}
		return db.Namespaces, nil
	default:
		return nil, errors.New("invalid namespace scope: " + db.NamespaceScope)
	}
}
```

- [ ] **Step 2: Add TestConnection and PopulateDbData to the model**

Append to `backend/internal/features/databases/databases/kubernetes/model.go` (add `"context"` and `"time"` to imports if not present — `context` is already imported; add `"time"`):
```go
func (k *KubernetesDatabase) TestConnection(
	logger *slog.Logger,
	_ encryption.FieldEncryptor,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	clientset, err := buildInClusterClientset()
	if err != nil {
		return err
	}

	version, err := detectServerVersion(ctx, clientset)
	if err != nil {
		return err
	}
	k.Version = version

	if err := verifyReadAccess(ctx, clientset, k); err != nil {
		return err
	}

	logger.Info("Kubernetes connection test passed", "server_version", version)
	return nil
}

// PopulateDbData detects the server version on a best-effort basis. Failure to
// reach the API (e.g. when the backend is run outside a cluster during local
// development) is logged but does not block database creation; TestConnection
// is the authoritative access check.
func (k *KubernetesDatabase) PopulateDbData(
	logger *slog.Logger,
	_ encryption.FieldEncryptor,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	clientset, err := buildInClusterClientset()
	if err != nil {
		logger.Warn("skipping Kubernetes version detection", "error", err)
		return nil
	}

	version, err := detectServerVersion(ctx, clientset)
	if err != nil {
		logger.Warn("failed to detect Kubernetes version", "error", err)
		return nil
	}
	k.Version = version

	return nil
}
```

- [ ] **Step 3: Verify build**

Run: `cd backend && go build ./... && go test ./internal/features/databases/databases/kubernetes/ -run 'Test_Validate|Test_BeforeSaveAfterFind' -v`
Expected: compiles; existing tests still PASS.

- [ ] **Step 4: Lint and commit**

```bash
cd backend && make lint && cd ..
git add backend/internal/features/databases/databases/kubernetes/client.go backend/internal/features/databases/databases/kubernetes/model.go
git commit -m "FEATURE (kubernetes): add in-cluster client, version detection, and connection test"
```

---

## Task 4: Sanitizer (pure function)

**Files:**
- Create: `backend/internal/features/databases/databases/kubernetes/sanitize.go`
- Test: `backend/internal/features/databases/databases/kubernetes/sanitize_test.go`

**Interfaces:**
- Produces:
  - `sanitizeObjectMeta(obj metav1.Object)` — strips server-managed metadata in place.
  - `toYAMLDocument(obj runtime.Object) ([]byte, error)` — sets TypeMeta + marshals one object to YAML.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/features/databases/databases/kubernetes/sanitize_test.go`:
```go
package kubernetes

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_SanitizeObjectMeta_StripsServerFields(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "tls-cert",
			Namespace:         "prod",
			ResourceVersion:   "12345",
			UID:               "abc-uid",
			Generation:        7,
			CreationTimestamp: metav1.NewTime(time.Unix(1000, 0)),
			ManagedFields:     []metav1.ManagedFieldsEntry{{Manager: "kubectl"}},
			OwnerReferences:   []metav1.OwnerReference{{Name: "owner"}},
			SelfLink:          "/api/v1/...",
			Labels:            map[string]string{"app": "web"},
			Annotations: map[string]string{
				"kubectl.kubernetes.io/last-applied-configuration": "{...}",
				"team": "payments",
			},
		},
	}

	sanitizeObjectMeta(secret)

	assert.Equal(t, "tls-cert", secret.Name)
	assert.Equal(t, "prod", secret.Namespace)
	assert.Equal(t, map[string]string{"app": "web"}, secret.Labels)
	assert.Equal(t, map[string]string{"team": "payments"}, secret.Annotations)
	assert.Empty(t, secret.ResourceVersion)
	assert.Empty(t, secret.UID)
	assert.Zero(t, secret.Generation)
	assert.True(t, secret.CreationTimestamp.IsZero())
	assert.Nil(t, secret.ManagedFields)
	assert.Nil(t, secret.OwnerReferences)
	assert.Empty(t, secret.SelfLink)
}

func Test_ToYAMLDocument_SetsTypeMetaForSecret(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "tls-cert", Namespace: "prod"},
		Data:       map[string][]byte{"tls.crt": []byte("xyz")},
		Type:       corev1.SecretTypeTLS,
	}

	doc, err := toYAMLDocument(secret)
	assert.NoError(t, err)

	text := string(doc)
	assert.True(t, strings.Contains(text, "apiVersion: v1"))
	assert.True(t, strings.Contains(text, "kind: Secret"))
	assert.True(t, strings.Contains(text, "name: tls-cert"))
}

func Test_ToYAMLDocument_SetsTypeMetaForConfigMap(t *testing.T) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "app-config", Namespace: "prod"},
		Data:       map[string]string{"key": "value"},
	}

	doc, err := toYAMLDocument(configMap)
	assert.NoError(t, err)

	text := string(doc)
	assert.True(t, strings.Contains(text, "kind: ConfigMap"))
	assert.True(t, strings.Contains(text, "name: app-config"))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/features/databases/databases/kubernetes/ -run 'Test_Sanitize|Test_ToYAML' -v`
Expected: FAIL — `sanitizeObjectMeta`/`toYAMLDocument` undefined.

- [ ] **Step 3: Implement the sanitizer**

Create `backend/internal/features/databases/databases/kubernetes/sanitize.go`:
```go
package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

const lastAppliedConfigAnnotation = "kubectl.kubernetes.io/last-applied-configuration"

// sanitizeObjectMeta removes server-managed metadata so the exported manifest
// is portable and re-appliable via `kubectl apply -f`.
func sanitizeObjectMeta(obj metav1.Object) {
	obj.SetResourceVersion("")
	obj.SetUID("")
	obj.SetGeneration(0)
	obj.SetCreationTimestamp(metav1.Time{})
	obj.SetDeletionTimestamp(nil)
	obj.SetManagedFields(nil)
	obj.SetOwnerReferences(nil)
	obj.SetSelfLink("")
	obj.SetGenerateName("")

	annotations := obj.GetAnnotations()
	delete(annotations, lastAppliedConfigAnnotation)
	if len(annotations) == 0 {
		annotations = nil
	}
	obj.SetAnnotations(annotations)
}

// toYAMLDocument sets the TypeMeta (List responses omit it) and marshals one
// object to YAML via sigs.k8s.io/yaml, which honours the JSON field tags.
func toYAMLDocument(obj runtime.Object) ([]byte, error) {
	switch typed := obj.(type) {
	case *corev1.Secret:
		typed.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"}
	case *corev1.ConfigMap:
		typed.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"}
	}

	return yaml.Marshal(obj)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/features/databases/databases/kubernetes/ -run 'Test_Sanitize|Test_ToYAML' -v`
Expected: PASS.

- [ ] **Step 5: Lint and commit**

```bash
cd backend && make lint && cd ..
git add backend/internal/features/databases/databases/kubernetes/sanitize.go backend/internal/features/databases/databases/kubernetes/sanitize_test.go
git commit -m "FEATURE (kubernetes): add manifest sanitizer and YAML document marshaller"
```

---

## Task 5: Export streaming (with fake-clientset test)

**Files:**
- Create: `backend/internal/features/databases/databases/kubernetes/export.go`
- Test: `backend/internal/features/databases/databases/kubernetes/export_test.go`

**Interfaces:**
- Consumes: `kubernetes.Interface`, `sanitizeObjectMeta`, `toYAMLDocument`, `resolveNamespaces`.
- Produces:
  - `(*KubernetesDatabase).OpenExportStream(ctx context.Context) (io.Reader, io.Closer, error)` — builds the in-cluster clientset and returns a streaming reader.
  - `streamExport(ctx context.Context, clientset kubernetes.Interface, db *KubernetesDatabase, writer io.Writer) error` — the testable core.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/features/databases/databases/kubernetes/export_test.go`:
```go
package kubernetes

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_StreamExport_AllNamespacesBothТypes(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: "prod", ResourceVersion: "9"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "staging"}},
	)
	db := &KubernetesDatabase{
		ResourceTypes:  []string{"SECRET", "CONFIGMAP"},
		NamespaceScope: "ALL",
	}

	var buf bytes.Buffer
	err := streamExport(context.Background(), clientset, db, &buf)
	assert.NoError(t, err)

	out := buf.String()
	assert.True(t, strings.Contains(out, "kind: Secret"))
	assert.True(t, strings.Contains(out, "name: s1"))
	assert.True(t, strings.Contains(out, "kind: ConfigMap"))
	assert.True(t, strings.Contains(out, "name: c1"))
	assert.True(t, strings.Contains(out, "---"))
	assert.False(t, strings.Contains(out, "resourceVersion: \"9\""))
}

func Test_StreamExport_ObjectNameFilter(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "keep", Namespace: "prod"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "drop", Namespace: "prod"}},
	)
	db := &KubernetesDatabase{
		ResourceTypes:  []string{"SECRET"},
		NamespaceScope: "SPECIFIC",
		Namespaces:     []string{"prod"},
		ObjectNames:    []string{"keep"},
	}

	var buf bytes.Buffer
	err := streamExport(context.Background(), clientset, db, &buf)
	assert.NoError(t, err)

	out := buf.String()
	assert.True(t, strings.Contains(out, "name: keep"))
	assert.False(t, strings.Contains(out, "name: drop"))
}
```
> Note: rename `Test_StreamExport_AllNamespacesBothТypes` to ASCII `Test_StreamExport_AllNamespacesBothTypes` when typing (avoid the accidental Cyrillic character).

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/features/databases/databases/kubernetes/ -run Test_StreamExport -v`
Expected: FAIL — `streamExport` undefined.

- [ ] **Step 3: Implement the export**

Create `backend/internal/features/databases/databases/kubernetes/export.go`:
```go
package kubernetes

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

// OpenExportStream builds the in-cluster client and returns a reader over the
// sanitized multi-document YAML export. A goroutine streams documents into a
// pipe; the returned Closer is the read side, whose closure unblocks the writer.
func (k *KubernetesDatabase) OpenExportStream(
	ctx context.Context,
) (io.Reader, io.Closer, error) {
	clientset, err := buildInClusterClientset()
	if err != nil {
		return nil, nil, err
	}

	reader, writer := io.Pipe()

	go func() {
		err := streamExport(ctx, clientset, k, writer)
		_ = writer.CloseWithError(err)
	}()

	return reader, reader, nil
}

func streamExport(
	ctx context.Context,
	clientset kubernetes.Interface,
	db *KubernetesDatabase,
	writer io.Writer,
) error {
	namespaces, err := resolveNamespaces(ctx, clientset, db)
	if err != nil {
		return err
	}

	nameFilter := toNameFilter(db.ObjectNames)
	isFirstDocument := true

	for _, namespace := range namespaces {
		for _, resourceType := range db.ResourceTypes {
			objects, listErr := listObjects(ctx, clientset, KubernetesResourceType(resourceType), namespace)
			if listErr != nil {
				return listErr
			}

			for _, object := range objects {
				metaObject, ok := object.(metav1.Object)
				if !ok {
					continue
				}
				if nameFilter != nil {
					if _, isWanted := nameFilter[metaObject.GetName()]; !isWanted {
						continue
					}
				}

				sanitizeObjectMeta(metaObject)

				document, marshalErr := toYAMLDocument(object)
				if marshalErr != nil {
					return fmt.Errorf("failed to marshal %s/%s: %w", metaObject.GetNamespace(), metaObject.GetName(), marshalErr)
				}

				if !isFirstDocument {
					if _, writeErr := io.WriteString(writer, "---\n"); writeErr != nil {
						return writeErr
					}
				}
				isFirstDocument = false

				if _, writeErr := writer.Write(document); writeErr != nil {
					return writeErr
				}
			}
		}
	}

	return nil
}

func listObjects(
	ctx context.Context,
	clientset kubernetes.Interface,
	resourceType KubernetesResourceType,
	namespace string,
) ([]runtime.Object, error) {
	switch resourceType {
	case KubernetesResourceTypeSecret:
		list, err := clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list secrets (namespace %q): %w", namespace, err)
		}
		objects := make([]runtime.Object, 0, len(list.Items))
		for i := range list.Items {
			objects = append(objects, &list.Items[i])
		}
		return objects, nil
	case KubernetesResourceTypeConfigMap:
		list, err := clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list configmaps (namespace %q): %w", namespace, err)
		}
		objects := make([]runtime.Object, 0, len(list.Items))
		for i := range list.Items {
			objects = append(objects, &list.Items[i])
		}
		return objects, nil
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

func toNameFilter(names []string) map[string]struct{} {
	if len(names) == 0 {
		return nil
	}
	filter := make(map[string]struct{}, len(names))
	for _, name := range names {
		filter[name] = struct{}{}
	}
	return filter
}

var _ = corev1.Secret{} // keep corev1 import if referenced only in tests
```
> Remove the trailing `var _ = corev1.Secret{}` line if `corev1` ends up referenced elsewhere; it exists only to avoid an unused-import error and `make lint` will flag it if redundant.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/features/databases/databases/kubernetes/ -v`
Expected: PASS (all kubernetes-package tests).

- [ ] **Step 5: Lint and commit**

```bash
cd backend && make lint && cd ..
git add backend/internal/features/databases/databases/kubernetes/export.go backend/internal/features/databases/databases/kubernetes/export_test.go
git commit -m "FEATURE (kubernetes): stream sanitized secret/configmap export to a reader"
```

---

## Task 6: Wire KubernetesDatabase into the base Database aggregate

**Files:**
- Modify: `backend/internal/features/databases/model.go`
- Modify: `backend/internal/features/databases/repository.go`
- Modify: `backend/internal/features/databases/service.go`

**Interfaces:**
- Consumes: `kubernetes.KubernetesDatabase` and its methods (Task 2–5).
- Produces: `Database.Kubernetes *kubernetes.KubernetesDatabase`; full CRUD persistence for the `KUBERNETES` type.

- [ ] **Step 1: Add the import and field in model.go**

In `backend/internal/features/databases/model.go`, add to the import block:
```go
	"databasus-backend/internal/features/databases/databases/kubernetes"
```
Add the struct field after the `Rabbitmq` field:
```go
	Kubernetes *kubernetes.KubernetesDatabase `json:"kubernetes,omitzero" gorm:"foreignKey:DatabaseID"`
```

- [ ] **Step 2: Add the switch cases in model.go**

In `Validate()` add before `default:`:
```go
	case DatabaseTypeKubernetes:
		if d.Kubernetes == nil {
			return errors.New("kubernetes database is required")
		}
		return d.Kubernetes.Validate()
```
In `EncryptSensitiveFields()` add before `return nil`:
```go
	if d.Kubernetes != nil {
		return d.Kubernetes.EncryptSensitiveFields(encryptor)
	}
```
In `PopulateDbData()` add before `return nil`:
```go
	if d.Kubernetes != nil {
		return d.Kubernetes.PopulateDbData(logger, encryptor)
	}
```
In `Update()` add a case:
```go
	case DatabaseTypeKubernetes:
		if d.Kubernetes != nil && incoming.Kubernetes != nil {
			d.Kubernetes.Update(incoming.Kubernetes)
		}
```
In `getSpecificDatabase()` add a case:
```go
	case DatabaseTypeKubernetes:
		return d.Kubernetes
```
> `IsUserReadOnly()` is left unchanged: its `default` branch already returns "read-only check not supported for this database type", which is correct for Kubernetes.

- [ ] **Step 3: Add repository.go cases**

In `backend/internal/features/databases/repository.go`:

Add the import:
```go
	"databasus-backend/internal/features/databases/databases/kubernetes"
```
In `Save`, first `switch` (the FK-assignment block), add:
```go
		case DatabaseTypeKubernetes:
			if database.Kubernetes == nil {
				return errors.New("kubernetes configuration is required for Kubernetes database")
			}
			database.Kubernetes.DatabaseID = &database.ID
```
Update BOTH `Omit(...)` calls to include `"Kubernetes"`:
```go
				Omit("Postgresql", "Mysql", "Mariadb", "Mongodb", "Redis", "Rabbitmq", "Kubernetes", "Notifiers").
```
In `Save`, the second `switch` (per-type create/save block), add:
```go
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
```
In `FindByID`, `FindByWorkspaceID`, and `GetAllDatabases`, add `.Preload("Kubernetes")` next to the other preloads.
In `Delete`'s `switch`, add:
```go
		case DatabaseTypeKubernetes:
			if err := tx.
				Where("database_id = ?", id).
				Delete(&kubernetes.KubernetesDatabase{}).Error; err != nil {
				return err
			}
```

- [ ] **Step 4: Add service.go CopyDatabase case**

In `backend/internal/features/databases/service.go`, add the import:
```go
	"databasus-backend/internal/features/databases/databases/kubernetes"
```
In `CopyDatabase`'s `switch`, add:
```go
	case DatabaseTypeKubernetes:
		if existingDatabase.Kubernetes != nil {
			newDatabase.Kubernetes = &kubernetes.KubernetesDatabase{
				ID:             uuid.Nil,
				DatabaseID:     nil,
				Version:        existingDatabase.Kubernetes.Version,
				ResourceTypes:  existingDatabase.Kubernetes.ResourceTypes,
				NamespaceScope: existingDatabase.Kubernetes.NamespaceScope,
				Namespaces:     existingDatabase.Kubernetes.Namespaces,
				ObjectNames:    existingDatabase.Kubernetes.ObjectNames,
			}
		}
```

- [ ] **Step 5: Verify build and existing tests**

Run: `cd backend && go build ./... && go test ./internal/features/databases/... 2>&1 | tail -20`
Expected: compiles; existing database tests PASS (the live-service tests for redis/postgres may require docker services as before — run the kubernetes package test specifically to confirm no regression: `go test ./internal/features/databases/databases/kubernetes/ -v`).

- [ ] **Step 6: Lint and commit**

```bash
cd backend && make lint && cd ..
git add backend/internal/features/databases/model.go backend/internal/features/databases/repository.go backend/internal/features/databases/service.go
git commit -m "FEATURE (kubernetes): wire KubernetesDatabase into the database aggregate and persistence"
```

---

## Task 7: Backup usecase and dispatch

**Files:**
- Create: `backend/internal/features/backups/backups/usecases/kubernetes/create_backup_uc.go`
- Create: `backend/internal/features/backups/backups/usecases/kubernetes/di.go`
- Modify: `backend/internal/features/backups/backups/usecases/di.go`
- Modify: `backend/internal/features/backups/backups/usecases/create_backup_uc.go`

**Interfaces:**
- Consumes: `streaming.Run`, `(*kubernetes.KubernetesDatabase).OpenExportStream`, `databases.Database`.
- Produces:
  - `usecases_kubernetes.CreateKubernetesBackupUsecase` with `Execute(ctx, *backups_core.Backup, *backups_config.BackupConfig, *databases.Database, *storages.Storage, func(float64)) (*common.BackupMetadata, error)`.
  - `usecases_kubernetes.GetCreateKubernetesBackupUsecase() *CreateKubernetesBackupUsecase`.

- [ ] **Step 1: Create the usecase**

Create `backend/internal/features/backups/backups/usecases/kubernetes/create_backup_uc.go`:
```go
package usecases_kubernetes

import (
	"context"
	"errors"
	"log/slog"

	common "databasus-backend/internal/features/backups/backups/common"
	backups_core "databasus-backend/internal/features/backups/backups/core"
	"databasus-backend/internal/features/backups/backups/streaming"
	backups_config "databasus-backend/internal/features/backups/config"
	"databasus-backend/internal/features/databases"
	encryption_secrets "databasus-backend/internal/features/encryption/secrets"
	"databasus-backend/internal/features/storages"
	"databasus-backend/internal/util/encryption"
)

type CreateKubernetesBackupUsecase struct {
	logger           *slog.Logger
	secretKeyService *encryption_secrets.SecretKeyService
	fieldEncryptor   encryption.FieldEncryptor
}

func (uc *CreateKubernetesBackupUsecase) Execute(
	ctx context.Context,
	backup *backups_core.Backup,
	backupConfig *backups_config.BackupConfig,
	db *databases.Database,
	storage *storages.Storage,
	backupProgressListener func(completedMBs float64),
) (*common.BackupMetadata, error) {
	logger := uc.logger.With("database_id", db.ID, "backup_id", backup.ID, "storage_id", storage.ID)
	logger.Info("creating Kubernetes backup via resource export")

	kubernetesDb := db.Kubernetes
	if kubernetesDb == nil {
		return nil, errors.New("kubernetes database configuration is required")
	}

	source, closer, err := kubernetesDb.OpenExportStream(ctx)
	if err != nil {
		return nil, err
	}

	metadata, _, err := streaming.Run(ctx, streaming.Spec{
		BackupID:         backup.ID,
		FileName:         backup.FileName,
		BackupConfig:     backupConfig,
		Source:           source,
		SourceCloser:     closer,
		Storage:          storage,
		ProgressListener: backupProgressListener,
		Logger:           logger,
		Encryptor:        uc.fieldEncryptor,
		SecretKeyService: uc.secretKeyService,
	})
	if err != nil {
		return nil, err
	}

	return metadata, nil
}
```

- [ ] **Step 2: Create the DI singleton**

Create `backend/internal/features/backups/backups/usecases/kubernetes/di.go`:
```go
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
```

- [ ] **Step 3: Register in usecases/di.go**

In `backend/internal/features/backups/backups/usecases/di.go`, add the import:
```go
	usecases_kubernetes "databasus-backend/internal/features/backups/backups/usecases/kubernetes"
```
Add to the `&CreateBackupUsecase{...}` literal (after the rabbitmq entry):
```go
	usecases_kubernetes.GetCreateKubernetesBackupUsecase(),
```

- [ ] **Step 4: Wire dispatch in create_backup_uc.go**

In `backend/internal/features/backups/backups/usecases/create_backup_uc.go`:

Add the import:
```go
	usecases_kubernetes "databasus-backend/internal/features/backups/backups/usecases/kubernetes"
```
Add the struct field (after `CreateRabbitmqBackupUsecase`):
```go
	CreateKubernetesBackupUsecase *usecases_kubernetes.CreateKubernetesBackupUsecase
```
Add the dispatch case before `default:`:
```go
	case databases.DatabaseTypeKubernetes:
		return uc.CreateKubernetesBackupUsecase.Execute(
			ctx,
			backup,
			backupConfig,
			database,
			storage,
			backupProgressListener,
		)
```

- [ ] **Step 5: Verify build**

Run: `cd backend && go build ./...`
Expected: compiles. (The positional struct literal in `di.go` now has the matching field order — confirm the new field is appended last in BOTH the struct definition and the literal.)

- [ ] **Step 6: Lint and commit**

```bash
cd backend && make lint && cd ..
git add backend/internal/features/backups/backups/usecases/kubernetes/ backend/internal/features/backups/backups/usecases/di.go backend/internal/features/backups/backups/usecases/create_backup_uc.go
git commit -m "FEATURE (kubernetes): add backup usecase and wire it into backup dispatch"
```

---

## Task 8: Controller validation test

**Files:**
- Modify: `backend/internal/features/databases/controller_test.go`

**Interfaces:**
- Consumes: existing test harness in `controller_test.go` (the `setupTestController`/request helpers used by the other database tests).

**Context:** CI has no live cluster, so the happy-path create flow (which calls `PopulateDbData` → cluster) is not exercised here. This test asserts the validation-rejection path, which fails at `Validate()` before any cluster call. The export/sanitize happy paths are covered by the fake-clientset tests in Task 5.

- [ ] **Step 1: Inspect the existing test helpers**

Run: `cd backend && grep -n "func setupTest\|func performRequest\|httptest\|CreateDatabase\b" internal/features/databases/controller_test.go | head -20`
Expected: identifies the helper used to issue an authenticated create request (e.g. a `performAuthenticatedRequest`/`setupDatabaseControllerTest` helper). Use the SAME helper the redis/postgres cases use in this file.

- [ ] **Step 2: Write the validation test**

Add to `backend/internal/features/databases/controller_test.go` (adapt the helper calls to match what Step 1 found; this mirrors the structure of the existing create tests in the file):
```go
func Test_CreateKubernetesDatabase_RejectsEmptyResourceTypes(t *testing.T) {
	database := &Database{
		Name: "Invalid Kubernetes Database",
		Type: DatabaseTypeKubernetes,
		Kubernetes: &kubernetes.KubernetesDatabase{
			NamespaceScope: string(kubernetes.KubernetesNamespaceScopeAll),
			// ResourceTypes deliberately empty -> Validate() must reject.
		},
	}

	err := database.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one resource type is required")
}

func Test_CreateKubernetesDatabase_RejectsSpecificScopeWithoutNamespaces(t *testing.T) {
	database := &Database{
		Name: "Invalid Kubernetes Database",
		Type: DatabaseTypeKubernetes,
		Kubernetes: &kubernetes.KubernetesDatabase{
			ResourceTypes:  []string{string(kubernetes.KubernetesResourceTypeSecret)},
			NamespaceScope: string(kubernetes.KubernetesNamespaceScopeSpecific),
		},
	}

	err := database.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one namespace is required")
}
```
Add the import to the test file if not present:
```go
	"databasus-backend/internal/features/databases/databases/kubernetes"
```

- [ ] **Step 3: Run the tests**

Run: `cd backend && go test ./internal/features/databases/ -run Test_CreateKubernetesDatabase -v`
Expected: PASS.

- [ ] **Step 4: Lint and commit**

```bash
cd backend && make lint && cd ..
git add backend/internal/features/databases/controller_test.go
git commit -m "TEST (kubernetes): validate KUBERNETES database creation rules"
```

---

## Task 9: Frontend entity model, type, icon

**Files:**
- Create: `frontend/src/entity/databases/model/kubernetes/KubernetesDatabase.ts`
- Create: `frontend/src/entity/databases/model/kubernetes/KubernetesResourceType.ts`
- Create: `frontend/public/icons/databases/kubernetes.svg`
- Modify: `frontend/src/entity/databases/model/DatabaseType.ts`
- Modify: `frontend/src/entity/databases/model/Database.ts`
- Modify: `frontend/src/entity/databases/model/getDatabaseLogoFromType.ts`
- Modify: `frontend/src/entity/databases/index.ts`

**Interfaces:**
- Produces: `DatabaseType.KUBERNETES`, `KubernetesDatabase` interface, `KubernetesResourceType` enum, `Database.kubernetes?`.

- [ ] **Step 1: Add the enum value**

In `frontend/src/entity/databases/model/DatabaseType.ts`, add:
```ts
  KUBERNETES = 'KUBERNETES',
```

- [ ] **Step 2: Create the resource-type enum**

Create `frontend/src/entity/databases/model/kubernetes/KubernetesResourceType.ts`:
```ts
export enum KubernetesResourceType {
  SECRET = 'SECRET',
  CONFIGMAP = 'CONFIGMAP',
}

export type KubernetesNamespaceScope = 'ALL' | 'SPECIFIC';
```

- [ ] **Step 3: Create the entity interface**

Create `frontend/src/entity/databases/model/kubernetes/KubernetesDatabase.ts`:
```ts
import type { KubernetesNamespaceScope } from './KubernetesResourceType';

export interface KubernetesDatabase {
  id: string;
  version: string;
  resourceTypes: string[];
  namespaceScope: KubernetesNamespaceScope;
  namespaces: string[];
  objectNames: string[];
}
```

- [ ] **Step 4: Add the field to Database.ts**

In `frontend/src/entity/databases/model/Database.ts`, add the import and field:
```ts
import type { KubernetesDatabase } from './kubernetes/KubernetesDatabase';
```
```ts
  kubernetes?: KubernetesDatabase;
```
(place the field next to `rabbitmq?`).

- [ ] **Step 5: Add the logo case**

In `frontend/src/entity/databases/model/getDatabaseLogoFromType.ts`, add before `default:`:
```ts
    case DatabaseType.KUBERNETES:
      return '/icons/databases/kubernetes.svg';
```

- [ ] **Step 6: Create the icon**

Create `frontend/public/icons/databases/kubernetes.svg` (official Kubernetes mark, single-color simplified):
```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="32" height="32"><path fill="#326CE5" d="M16 1.6 4 7.3v9.4c0 6.1 5 11.8 12 13.7 7-1.9 12-7.6 12-13.7V7.3z"/><path fill="#fff" d="M16 6.8a9.2 9.2 0 1 0 0 18.4 9.2 9.2 0 0 0 0-18.4zm0 2.1 1.1 4.6-2.2 0zm-5.6 2.6 3.8 2.8-1.7 1.4zm11.2 0-2.1 4.2-1.7-1.4zM12 17.4h2.1l-.7 4.5zm5.9 0H20l-2.8 4.5z"/></svg>
```
> If a brand-accurate asset is preferred, replace with the official SVG from the Kubernetes brand kit; keep the filename `kubernetes.svg`.

- [ ] **Step 7: Export from the entity index**

In `frontend/src/entity/databases/index.ts`, add:
```ts
export { type KubernetesDatabase } from './model/kubernetes/KubernetesDatabase';
export { KubernetesResourceType } from './model/kubernetes/KubernetesResourceType';
export { type KubernetesNamespaceScope } from './model/kubernetes/KubernetesResourceType';
```

- [ ] **Step 8: Verify and commit**

Run: `cd frontend && pnpm lint && pnpm format && npx tsc --noEmit`
Expected: no type errors.
```bash
cd .. && git add frontend/src/entity/databases/ frontend/public/icons/databases/kubernetes.svg
git commit -m "FEATURE (kubernetes): add frontend entity model, type, and icon"
```

---

## Task 10: Frontend edit form + create wiring

**Files:**
- Create: `frontend/src/features/databases/ui/edit/EditKubernetesSpecificDataComponent.tsx`
- Modify: `frontend/src/features/databases/ui/edit/EditDatabaseBaseInfoComponent.tsx`
- Modify: `frontend/src/features/databases/ui/edit/EditDatabaseSpecificDataComponent.tsx`

**Interfaces:**
- Consumes: `Database`, `databaseApi`, `KubernetesResourceType`. Same `Props` shape as `EditRedisSpecificDataComponent`.
- Produces: `EditKubernetesSpecificDataComponent`.

- [ ] **Step 1: Add the type-dropdown option and init**

In `frontend/src/features/databases/ui/edit/EditDatabaseBaseInfoComponent.tsx`:

Add the import:
```ts
  type KubernetesDatabase,
```
(add inside the existing `entity/databases` import group).
Add to `databaseTypeOptions`:
```ts
  { value: DatabaseType.KUBERNETES, label: 'Kubernetes' },
```
Add to the reset object (the block setting `redis: undefined,` etc.):
```ts
      kubernetes: undefined,
```
Add a case to the `switch (newType)`:
```ts
      case DatabaseType.KUBERNETES:
        updatedDatabase.kubernetes =
          editingDatabase.kubernetes ??
          ({
            resourceTypes: ['SECRET'],
            namespaceScope: 'ALL',
            namespaces: [],
            objectNames: [],
          } as KubernetesDatabase);
        break;
```

- [ ] **Step 2: Create the edit form**

Create `frontend/src/features/databases/ui/edit/EditKubernetesSpecificDataComponent.tsx`:
```tsx
import { Alert, Button, Checkbox, Radio, Select } from 'antd';
import { useEffect, useState } from 'react';

import { type Database, KubernetesResourceType, databaseApi } from '../../../../entity/databases';
import { ToastHelper } from '../../../../shared/toast';

interface Props {
  database: Database;

  isShowCancelButton?: boolean;
  onCancel: () => void;

  isShowBackButton: boolean;
  onBack: () => void;

  saveButtonText?: string;
  isSaveToApi: boolean;
  onSaved: (database: Database) => void;
}

export const EditKubernetesSpecificDataComponent = ({
  database,

  isShowCancelButton,
  onCancel,

  isShowBackButton,
  onBack,

  saveButtonText,
  isSaveToApi,
  onSaved,
}: Props) => {
  const [editingDatabase, setEditingDatabase] = useState<Database>();
  const [isSaving, setIsSaving] = useState(false);

  const [isConnectionTested, setIsConnectionTested] = useState(false);
  const [isTestingConnection, setIsTestingConnection] = useState(false);

  useEffect(() => {
    setIsSaving(false);
    setIsConnectionTested(false);
    setIsTestingConnection(false);
    setEditingDatabase({ ...database });
  }, [database]);

  const updateKubernetes = (changes: Partial<NonNullable<Database['kubernetes']>>) => {
    if (!editingDatabase?.kubernetes) return;
    setEditingDatabase({
      ...editingDatabase,
      kubernetes: { ...editingDatabase.kubernetes, ...changes },
    });
    setIsConnectionTested(false);
  };

  const testConnection = async () => {
    if (!editingDatabase?.kubernetes) return;

    setIsTestingConnection(true);
    try {
      await databaseApi.testDatabaseConnectionDirect(editingDatabase);
      setIsConnectionTested(true);
      ToastHelper.showToast({
        title: 'Connection test passed',
        description: 'You can continue with the next step',
      });
    } catch (e) {
      alert((e as Error).message);
    }
    setIsTestingConnection(false);
  };

  const saveDatabase = async () => {
    if (!editingDatabase?.kubernetes) return;

    if (isSaveToApi) {
      setIsSaving(true);
      try {
        await databaseApi.updateDatabase(editingDatabase);
      } catch (e) {
        alert((e as Error).message);
      }
      setIsSaving(false);
    }

    onSaved(editingDatabase);
  };

  if (!editingDatabase?.kubernetes) return null;

  const kubernetes = editingDatabase.kubernetes;
  const isSecretSelected = kubernetes.resourceTypes.includes(KubernetesResourceType.SECRET);
  const isSpecificScope = kubernetes.namespaceScope === 'SPECIFIC';

  const isAllFieldsFilled =
    kubernetes.resourceTypes.length > 0 && (!isSpecificScope || kubernetes.namespaces.length > 0);

  return (
    <div>
      <div className="mb-3 flex w-full items-start">
        <div className="min-w-[150px] pt-1">Resource types</div>
        <Checkbox.Group
          value={kubernetes.resourceTypes}
          onChange={(values) => updateKubernetes({ resourceTypes: values as string[] })}
          options={[
            { label: 'Secrets', value: KubernetesResourceType.SECRET },
            { label: 'ConfigMaps', value: KubernetesResourceType.CONFIGMAP },
          ]}
        />
      </div>

      {isSecretSelected && (
        <Alert
          className="mb-3"
          type="warning"
          showMessage
          message="Secrets contain sensitive data. Enable backup encryption in the next step so the exported values are not stored in plain base64."
        />
      )}

      <div className="mb-3 flex w-full items-start">
        <div className="min-w-[150px] pt-1">Namespaces</div>
        <Radio.Group
          value={kubernetes.namespaceScope}
          onChange={(e) => updateKubernetes({ namespaceScope: e.target.value })}
        >
          <Radio value="ALL">All namespaces</Radio>
          <Radio value="SPECIFIC">Specific namespaces</Radio>
        </Radio.Group>
      </div>

      {isSpecificScope && (
        <div className="mb-3 flex w-full items-center">
          <div className="min-w-[150px]">Namespace list</div>
          <Select
            mode="tags"
            value={kubernetes.namespaces}
            onChange={(values) => updateKubernetes({ namespaces: values })}
            size="small"
            className="max-w-[280px] grow"
            placeholder="Type a namespace and press Enter"
            tokenSeparators={[',', ' ']}
          />
        </div>
      )}

      <div className="mb-5 flex w-full items-center">
        <div className="min-w-[150px]">Object names</div>
        <Select
          mode="tags"
          value={kubernetes.objectNames}
          onChange={(values) => updateKubernetes({ objectNames: values })}
          size="small"
          className="max-w-[280px] grow"
          placeholder="Optional - leave empty for all objects"
          tokenSeparators={[',', ' ']}
        />
      </div>

      <div className="mt-5 flex">
        {isShowCancelButton && (
          <Button className="mr-1" danger ghost onClick={() => onCancel()}>
            Cancel
          </Button>
        )}

        {isShowBackButton && (
          <Button className="mr-auto" type="primary" ghost onClick={() => onBack()}>
            Back
          </Button>
        )}

        {!isConnectionTested && (
          <Button
            type="primary"
            onClick={() => testConnection()}
            loading={isTestingConnection}
            disabled={!isAllFieldsFilled}
            className="mr-5"
          >
            Test connection
          </Button>
        )}

        {isConnectionTested && (
          <Button
            type="primary"
            onClick={() => saveDatabase()}
            loading={isSaving}
            disabled={!isAllFieldsFilled}
            className="mr-5"
          >
            {saveButtonText || 'Save'}
          </Button>
        )}
      </div>
    </div>
  );
};
```
> The `Alert` prop in Ant Design is `message` (not `showMessage`); remove the stray `showMessage` line if `pnpm lint`/tsc flags it — it is included above only as a visual cue and is not a valid prop.

- [ ] **Step 3: Add the dispatcher case**

In `frontend/src/features/databases/ui/edit/EditDatabaseSpecificDataComponent.tsx`:

Add the import:
```ts
import { EditKubernetesSpecificDataComponent } from './EditKubernetesSpecificDataComponent';
```
Extend the `isReadOnlyUserNotSupported` check to include Kubernetes:
```ts
    const isReadOnlyUserNotSupported =
      databaseToSave.type === DatabaseType.REDIS ||
      databaseToSave.type === DatabaseType.RABBITMQ ||
      databaseToSave.type === DatabaseType.KUBERNETES;
```
Add the `switch` case:
```ts
    case DatabaseType.KUBERNETES:
      return <EditKubernetesSpecificDataComponent {...commonProps} />;
```

- [ ] **Step 4: Verify and commit**

Run: `cd frontend && pnpm lint && pnpm format && npx tsc --noEmit`
Expected: no type errors.
```bash
cd .. && git add frontend/src/features/databases/ui/edit/
git commit -m "FEATURE (kubernetes): add edit form and create wiring for Kubernetes source"
```

---

## Task 11: Frontend show view + backups list behavior

**Files:**
- Create: `frontend/src/features/databases/ui/show/ShowKubernetesSpecificDataComponent.tsx`
- Modify: `frontend/src/features/databases/ui/show/ShowDatabaseSpecificDataComponent.tsx`
- Modify: `frontend/src/features/backups/ui/BackupsComponent.tsx`

**Interfaces:**
- Consumes: `Database`, `DatabaseType`.
- Produces: `ShowKubernetesSpecificDataComponent`.

- [ ] **Step 1: Create the show component**

Create `frontend/src/features/databases/ui/show/ShowKubernetesSpecificDataComponent.tsx`:
```tsx
import { type Database } from '../../../../entity/databases';

interface Props {
  database: Database;
}

export const ShowKubernetesSpecificDataComponent = ({ database }: Props) => {
  const kubernetes = database.kubernetes;

  return (
    <div>
      <div className="mb-1 flex w-full items-center">
        <div className="min-w-[150px]">Resource types</div>
        <div>{kubernetes?.resourceTypes?.join(', ') || ''}</div>
      </div>

      <div className="mb-1 flex w-full items-center">
        <div className="min-w-[150px]">Namespace scope</div>
        <div>{kubernetes?.namespaceScope === 'SPECIFIC' ? 'Specific namespaces' : 'All namespaces'}</div>
      </div>

      {kubernetes?.namespaceScope === 'SPECIFIC' && (
        <div className="mb-1 flex w-full items-center">
          <div className="min-w-[150px]">Namespaces</div>
          <div>{kubernetes?.namespaces?.join(', ') || ''}</div>
        </div>
      )}

      <div className="mb-1 flex w-full items-center">
        <div className="min-w-[150px]">Object names</div>
        <div>{kubernetes?.objectNames?.length ? kubernetes.objectNames.join(', ') : 'All objects'}</div>
      </div>
    </div>
  );
};
```

- [ ] **Step 2: Add the show dispatcher case**

In `frontend/src/features/databases/ui/show/ShowDatabaseSpecificDataComponent.tsx`:

Add the import:
```ts
import { ShowKubernetesSpecificDataComponent } from './ShowKubernetesSpecificDataComponent';
```
Add the case:
```ts
    case DatabaseType.KUBERNETES:
      return <ShowKubernetesSpecificDataComponent database={database} />;
```

- [ ] **Step 3: Hide Restore for Kubernetes in BackupsComponent**

In `frontend/src/features/backups/ui/BackupsComponent.tsx`, extend the Restore-button guard to include Kubernetes:
```tsx
                {database.type !== DatabaseType.REDIS &&
                  database.type !== DatabaseType.RABBITMQ &&
                  database.type !== DatabaseType.KUBERNETES && (
                    <Tooltip title="Restore from backup">
```
(the closing `)}` of that block is unchanged).

- [ ] **Step 4: Add the Kubernetes download tooltip**

In the same file, extend the nested download-tooltip ternary. Replace the final `: 'Download backup file'` fallback with a Kubernetes branch:
```tsx
                            : database.type === DatabaseType.KUBERNETES
                              ? 'Download backup file. It is a sanitized multi-document YAML - restore manually via kubectl apply -f <file>'
                              : 'Download backup file'
```
> If Task 11 runs on a tree where the Redis/RabbitMQ branches from `feature/redis-rabbitmq-backup-support` are present (they are), insert the Kubernetes branch immediately before the existing `: 'Download backup file'` terminal fallback, preserving the existing Redis/RabbitMQ branches.

- [ ] **Step 5: Verify and commit**

Run: `cd frontend && pnpm lint && pnpm format && npx tsc --noEmit`
Expected: no type errors.
```bash
cd .. && git add frontend/src/features/databases/ui/show/ frontend/src/features/backups/ui/BackupsComponent.tsx
git commit -m "FEATURE (kubernetes): add show view and hide restore for Kubernetes backups"
```

---

## Task 12: Helm RBAC and ServiceAccount

**Files:**
- Create: `deploy/helm/templates/serviceaccount.yaml`
- Create: `deploy/helm/templates/clusterrole.yaml`
- Create: `deploy/helm/templates/clusterrolebinding.yaml`
- Modify: `deploy/helm/values.yaml`
- Modify: `deploy/helm/templates/statefulset.yaml`

**Interfaces:**
- Consumes: existing helpers `databasus.serviceAccountName`, `databasus.fullname`, `databasus.labels`, `databasus.namespace`.
- Produces: a ServiceAccount + cluster-wide read-only ClusterRole + binding, mounted into the StatefulSet pod.

- [ ] **Step 1: Add values blocks**

In `deploy/helm/values.yaml`, add near the top (after `replicaCount: 1`):
```yaml

# ServiceAccount used by the Databasus pod. Required for Kubernetes
# secret/configmap backups (in-cluster API access).
serviceAccount:
  create: true
  name: ""

# Cluster-wide read-only RBAC for Kubernetes secret/configmap backups.
# Grants get/list on secrets, configmaps and namespaces across the cluster.
# Set create: false to disable the feature and drop the ClusterRole.
rbac:
  create: true
```

- [ ] **Step 2: Create the ServiceAccount template**

Create `deploy/helm/templates/serviceaccount.yaml`:
```yaml
{{- if .Values.serviceAccount.create }}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "databasus.serviceAccountName" . }}
  namespace: {{ include "databasus.namespace" . }}
  labels:
    {{- include "databasus.labels" . | nindent 4 }}
{{- end }}
```

- [ ] **Step 3: Create the ClusterRole template**

Create `deploy/helm/templates/clusterrole.yaml`:
```yaml
{{- if .Values.rbac.create }}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ include "databasus.fullname" . }}-backup-reader
  labels:
    {{- include "databasus.labels" . | nindent 4 }}
rules:
  - apiGroups: [""]
    resources: ["secrets", "configmaps", "namespaces"]
    verbs: ["get", "list"]
{{- end }}
```

- [ ] **Step 4: Create the ClusterRoleBinding template**

Create `deploy/helm/templates/clusterrolebinding.yaml`:
```yaml
{{- if .Values.rbac.create }}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ include "databasus.fullname" . }}-backup-reader
  labels:
    {{- include "databasus.labels" . | nindent 4 }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{ include "databasus.fullname" . }}-backup-reader
subjects:
  - kind: ServiceAccount
    name: {{ include "databasus.serviceAccountName" . }}
    namespace: {{ include "databasus.namespace" . }}
{{- end }}
```

- [ ] **Step 5: Mount the ServiceAccount on the pod**

In `deploy/helm/templates/statefulset.yaml`, inside the pod `template.spec` (the block starting at the `spec:` on line 25, right before `imagePullSecrets`), add:
```yaml
      serviceAccountName: {{ include "databasus.serviceAccountName" . }}
```

- [ ] **Step 6: Render and verify with helm template**

Run:
```bash
helm template databasus ./deploy/helm --namespace databasus | grep -E "kind: (ServiceAccount|ClusterRole|ClusterRoleBinding)|serviceAccountName:"
```
Expected: shows `kind: ServiceAccount`, `kind: ClusterRole`, `kind: ClusterRoleBinding`, and a `serviceAccountName:` line.

Run the disabled case:
```bash
helm template databasus ./deploy/helm --namespace databasus --set rbac.create=false --set serviceAccount.create=false | grep -E "kind: ClusterRole|kind: ServiceAccount" || echo "RBAC omitted as expected"
```
Expected: prints `RBAC omitted as expected` (no RBAC objects rendered). The `serviceAccountName` line falls back to `default` via the helper.

- [ ] **Step 7: Lint and commit**

Run: `helm lint ./deploy/helm`
Expected: `1 chart(s) linted, 0 chart(s) failed`.
```bash
git add deploy/helm/values.yaml deploy/helm/templates/serviceaccount.yaml deploy/helm/templates/clusterrole.yaml deploy/helm/templates/clusterrolebinding.yaml deploy/helm/templates/statefulset.yaml
git commit -m "FEATURE (kubernetes): add ServiceAccount and read-only RBAC to Helm chart"
```

---

## Task 13: Full-suite verification and README note

**Files:**
- Modify: `README.md` (security section — document the cluster-wide secret-read grant)

- [ ] **Step 1: Run the backend suite that does not need live services**

Run: `cd backend && go build ./... && go test ./internal/features/databases/databases/kubernetes/... -v && make lint && cd ..`
Expected: build + kubernetes package tests PASS; lint clean.

- [ ] **Step 2: Run the frontend checks**

Run: `cd frontend && pnpm lint && pnpm format && npx tsc --noEmit && cd ..`
Expected: clean.

- [ ] **Step 3: Document the RBAC grant in the README**

In `README.md`, under the `🛡️ Security & reliability engineering` section, add a bullet (match the surrounding bullet style):
```markdown
- **Kubernetes backups use least-privilege, read-only RBAC.** The Helm chart provisions a ServiceAccount with a ClusterRole granting only `get`/`list` on `secrets`, `configmaps` and `namespaces`. This lets Databasus back up Secrets/ConfigMaps cluster-wide; because it can read every Secret in the cluster, restrict who can create Kubernetes sources and enable backup encryption for Secret exports. Disable entirely with `rbac.create=false` and `serviceAccount.create=false`.
```

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "DOCS (kubernetes): document read-only RBAC grant for cluster backups"
```

- [ ] **Step 5: Push the branch**

Run: `git push -u origin feature/kubernetes-backup-support`
Expected: branch pushed; open a PR targeting `feature/redis-rabbitmq-backup-support` (or `develop`/`main` per the team's merge flow for this stack of branches).

---

## Self-Review

**Spec coverage:**
- Enum (`KUBERNETES`) — Task 1 (backend), Task 9 (frontend). ✓
- `kubernetes_databases` model + migration + list columns — Tasks 1, 2. ✓
- DatabaseConnector methods (TestConnection/GetRawDbSizeMb=0/HideSensitiveData no-op) + Encrypt/Populate/Update no-ops — Tasks 2, 3. ✓
- In-cluster auth, version detection, access check — Task 3. ✓
- Sanitized multi-doc YAML export via streaming pipeline — Tasks 4, 5, 7. ✓
- Selection semantics (resource types / namespace scope / object names) — Tasks 2 (validate), 5 (export filter), 10 (UI). ✓
- Restore/verify omitted — no restore wiring added; BackupsComponent hides Restore (Task 11); read-only check returns unsupported via default (Task 6). ✓
- Persistence wiring (repository/service/model switches) — Task 6. ✓
- Helm ServiceAccount + read-only ClusterRole/binding, enabled by default, `serviceAccountName` on pod — Task 12. ✓
- Frontend forms/show/dispatchers/icon/tooltip — Tasks 9–11. ✓
- Security note — Tasks 10 (UI warning), 13 (README). ✓
- Testing (sanitizer unit, fake-clientset export, validation, helm smoke) — Tasks 4, 5, 8, 12. ✓

**Placeholder scan:** No "TBD"/"handle edge cases"/"similar to". Each code step contains full code. Two inline caveats are flagged explicitly (the Ant `Alert` `message` prop; the accidental non-ASCII test name in Task 5 Step 1) so the implementer corrects them rather than copying blindly.

**Type consistency:** `KubernetesDatabase` field set is identical across model (Task 2), CopyDatabase (Task 6), and entity TS (Task 9: `resourceTypes`/`namespaceScope`/`namespaces`/`objectNames`/`version`). `streamExport`/`OpenExportStream` signatures match between Task 5 (definition) and Task 7 (caller). Usecase method `Execute(...)` signature matches the sibling redis/rabbitmq usecases and the dispatch call in Task 7. Enum string values (`SECRET`/`CONFIGMAP`/`ALL`/`SPECIFIC`) are consistent across backend, frontend, and Helm-irrelevant layers.

**Known CI limitation (called out, not a gap):** the happy-path controller create flow is not unit-tested because it requires a live cluster (`PopulateDbData`); coverage is provided by the fake-clientset export tests and validation tests instead. `PopulateDbData` is intentionally best-effort so creation does not hard-fail outside a cluster.

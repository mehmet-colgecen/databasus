# Kubernetes Secret & ConfigMap Backup Support — Design Spec

**Date:** 2026-06-24
**Status:** Approved (pending spec review)
**Base branch:** `feature/kubernetes-backup-support` (off `feature/redis-rabbitmq-backup-support`)

---

## Problem

Databasus runs inside a Kubernetes cluster. Operators want scheduled, encrypted
backups of cluster **Secrets** and **ConfigMaps** pushed to S3 (or any configured
storage). They do not need automated restore — download + manual `kubectl apply`
is sufficient. Backups must be able to target any namespace in the cluster.

---

## Approach

Add a new `KUBERNETES` source type that follows the **Redis/RabbitMQ pattern**:
a `DatabaseType` with its own model + migration and a **server-side backup
usecase that streams a single file directly to storage** through the existing
`io.Pipe → encryption → counting → storage.SaveFile` pipeline. No external agent.
No automated restore, no verify.

Authentication is **in-cluster only**: the backend uses its own mounted
ServiceAccount token (`rest.InClusterConfig()`). The Helm chart gains a
ServiceAccount + a read-only ClusterRole + ClusterRoleBinding so the backend can
read `secrets`/`configmaps` across all namespaces.

---

## Architecture

### 1. Database type enum

**Backend** (`backend/internal/features/databases/enums.go`):

```go
DatabaseTypeKubernetes DatabaseType = "KUBERNETES"
```

**Frontend** (`frontend/src/entity/databases/model/DatabaseType.ts`):

```ts
KUBERNETES = 'KUBERNETES',
```

---

### 2. Data model — `kubernetes_databases` table

There are **no connection credentials** — auth is in-cluster. The row *is* the
selection of what to back up.

| Column            | Type          | Notes                                                              |
| ----------------- | ------------- | ------------------------------------------------------------------ |
| `id`            | uuid PK       |                                                                    |
| `database_id`   | uuid FK       | → `databases(id)` ON DELETE CASCADE                              |
| `resource_types`| text NOT NULL | joined list, e.g. `SECRET,CONFIGMAP` (≥1 required)               |
| `namespace_scope`| text NOT NULL | `ALL` or `SPECIFIC` (default `ALL`)                            |
| `namespaces`    | text          | joined list; required and non-empty when scope = `SPECIFIC`      |
| `object_names`  | text          | joined list; optional; empty = all objects of the selected types   |
| `version`       | text NOT NULL DEFAULT '' | k8s server version detected at TestConnection           |

GORM model: `backend/internal/features/databases/databases/kubernetes/model.go`.
Following the existing `include_schemas` convention, the list columns are stored
as joined strings with `gorm:"-"` slice fields plus a backing `...String` column
(e.g. `ResourceTypes []string gorm:"-"` + `ResourceTypesString string gorm:"column:resource_types"`).

Resource-type values use an enum:

```go
type KubernetesResourceType string
const (
    KubernetesResourceTypeSecret    KubernetesResourceType = "SECRET"
    KubernetesResourceTypeConfigMap KubernetesResourceType = "CONFIGMAP"
)
```

#### Selection semantics

- `namespace_scope = ALL` → enumerate all namespaces via the API.
- `namespace_scope = SPECIFIC` → use the configured `namespaces` list.
- `object_names` is a filter applied **within** the resolved namespace scope.
  Empty means "all objects of the selected types". If names are provided with
  `ALL` scope, any object matching one of those names in any namespace is
  backed up; a name absent from a namespace is simply skipped (not an error).

#### `DatabaseConnector` interface

The model implements the same interface as every other type, with k8s-specific
behaviour:

- `Validate()` — ≥1 resource type; `namespaces` non-empty when scope = `SPECIFIC`.
- `TestConnection()` — build the in-cluster client; run a `SelfSubjectAccessReview`
  ("can I `list` secrets/configmaps in scope") and read the server version. No
  dial/auth against a remote host.
- `GetRawDbSizeMb()` — returns `0` (config, not a dataset — same rationale as
  RabbitMQ; the real artifact size is still recorded by the counting writer).
- `HideSensitiveData()` — **no-op** (the config holds no secrets).
- `EncryptSensitiveFields()` / decrypt helper — **no-op** (no credentials).
- `PopulateDbData()` / `PopulateVersion()` — detect and store the server version.
- `Update()` — standard field merge.
- `IsUserReadOnly()` returns "not supported"; no `CreateReadOnlyUser`.

**`Database` struct** (`databases/model.go`) gains:

```go
Kubernetes *kubernetes.KubernetesDatabase `json:"kubernetes,omitzero" gorm:"foreignKey:DatabaseID"`
```

All switch statements in `model.go` (`Validate`, `Update`, `getSpecificDatabase`,
`EncryptSensitiveFields`, `PopulateDbData`, `TestConnection`, etc.) gain a
`DatabaseTypeKubernetes` case.

---

### 3. Migration

One timestamped SQL migration, next in sequence after the Redis/RabbitMQ ones:

```sql
-- create_kubernetes_databases_table.sql
CREATE TABLE kubernetes_databases (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  database_id     UUID REFERENCES databases(id) ON DELETE CASCADE,
  resource_types  TEXT NOT NULL DEFAULT '',
  namespace_scope TEXT NOT NULL DEFAULT 'ALL',
  namespaces      TEXT NOT NULL DEFAULT '',
  object_names    TEXT NOT NULL DEFAULT '',
  version         TEXT NOT NULL DEFAULT ''
);
```

---

### 4. Backup engine — native Go, client-go

`backend/internal/features/backups/backups/usecases/kubernetes/create_backup_uc.go`

> **Dependency note:** this adds `k8s.io/client-go`, `k8s.io/api`, and
> `k8s.io/apimachinery` to the backend `go.mod`. Heavyweight but standard and
> well-maintained. No CLI binaries are bundled.

1. Build a clientset from `rest.InClusterConfig()`.
2. Resolve target namespaces (`CoreV1().Namespaces().List` when scope = `ALL`,
   else the configured list).
3. For each selected resource type, `List` objects per namespace; filter to
   `object_names` when provided.
4. **Sanitize** each object for restore-friendliness — strip
   `metadata.resourceVersion`, `uid`, `creationTimestamp`, `generation`,
   `selfLink`, `managedFields`, `ownerReferences`, the whole `status`, and the
   `kubectl.kubernetes.io/last-applied-configuration` annotation. Keep `name`,
   `namespace`, `labels`, remaining annotations, `data` (and `type` for Secrets).
   This is a **pure function** — its own unit test.
5. Marshal each sanitized object to YAML, separated by `---`, streamed into the
   existing `io.Pipe → encryption writer → counting writer → storage.SaveFile`
   pipeline. Track progress via `backupProgressListener`.
6. Filename: `<backup-id>.yaml` (or `.yaml.enc` when encrypted).
7. Reuses the MongoDB-path constants (`backupTimeout`, `shutdownCheckInterval`,
   `copyBufferSize`).

**Registration:**

- `usecases/di.go` — add `CreateKubernetesBackupUsecase *usecases_kubernetes.CreateKubernetesBackupUsecase`.
- `create_backup_uc.go` — add a `DatabaseTypeKubernetes` case to the dispatch switch.

---

### 5. Restore & verify — unchanged

The restore subsystem (`restores/usecases/di.go`, `RestoreBackupUsecase` switch)
is **not modified**. A restore request for a Kubernetes backup returns
"database type not supported" — unreachable from the UI. Verify is Postgres-only
and needs no change.

---

### 6. Helm RBAC — `deploy/helm/templates/`

The chart currently creates **no** ServiceAccount and **no** RBAC. Add:

- `serviceaccount.yaml` — gated on `.Values.serviceAccount.create` (default `true`).
- `clusterrole.yaml` — gated on `.Values.rbac.create` (default `true`):

  ```yaml
  rules:
    - apiGroups: [""]
      resources: ["secrets", "configmaps", "namespaces"]
      verbs: ["get", "list"]   # read-only — least privilege
  ```

- `clusterrolebinding.yaml` — binds the ServiceAccount to the ClusterRole.
- `statefulset.yaml` — set `serviceAccountName: {{ include "databasus.serviceAccountName" . }}`
  (the helper already exists in `_helpers.tpl`).

New `values.yaml` block (**RBAC ships enabled by default — no opt-in flag**):

```yaml
serviceAccount:
  create: true
  name: ""        # defaults to the release fullname
rbac:
  create: true    # ClusterRole + ClusterRoleBinding: read-only get/list on
                  # secrets, configmaps, namespaces (cluster-wide)
```

`namespaces` is included in the ClusterRole because `ALL`-scope backups must
enumerate namespaces. A ClusterRole (not a namespaced Role) is required because
the source's namespace selection is chosen at runtime by users, not known at
Helm install time, and "any namespace" is an explicit requirement.

---

### 7. Frontend (mirrors the Redis/RabbitMQ files)

- **Icon:** `frontend/public/icons/databases/kubernetes.svg`;
  `getDatabaseLogoFromType.ts` gains a `KUBERNETES` case.
- **Entity model:** `frontend/src/entity/databases/model/kubernetes/KubernetesDatabase.ts`
  (`resourceTypes: string[]`, `namespaceScope: 'ALL' | 'SPECIFIC'`,
  `namespaces: string[]`, `objectNames: string[]`, `version: string`), wired into
  `Database.ts` (`kubernetes?: KubernetesDatabase`) and exported from
  `entity/databases/index.ts`.
- **Create:** `CreateDatabaseComponent.tsx` adds `KUBERNETES` to the type dropdown
  and its init case.
- **Edit form:** `EditKubernetesSpecificDataComponent.tsx` — resource-type
  checkboxes (Secrets / ConfigMaps), a namespace-scope radio (`All` / `Specific`)
  revealing a tag-input for namespaces, and an optional tag-input for object names.
  Shows a **warning banner** when Secrets are selected and the backup config's
  encryption is `NONE`, recommending `ENCRYPTED`.
- **Show:** `ShowKubernetesSpecificDataComponent.tsx` (read-only view).
- **Dispatchers:** `EditDatabaseSpecificDataComponent.tsx` and
  `ShowDatabaseSpecificDataComponent.tsx` gain `KUBERNETES` cases.
- **Backups list** (`BackupsComponent.tsx`): hide Restore and Verify for
  `KUBERNETES`; download tooltip: "Download the YAML export. Restore manually via
  `kubectl apply -f <file>`."

---

## Security considerations

Databasus handles sensitive data; these are real, not theoretical, concerns:

1. **Cluster-wide secret read is a powerful grant.** With RBAC enabled by default,
   every install gets a ServiceAccount that can read **every Secret in the
   cluster**. Anyone who can create a Kubernetes source and download its backup
   can therefore read all cluster secrets. This is acceptable for single-tenant
   self-hosting but is a privilege-escalation concern in multi-tenant
   deployments. Documented in the chart values and README; operators who don't
   need the feature can set `rbac.create: false` / `serviceAccount.create: false`.
2. **Secrets are base64-only in the YAML unless backup encryption is on.** The UI
   warns when Secrets are selected and the backup config's encryption is `NONE`,
   recommending `ENCRYPTED`. Encryption uses the existing backup-encryption path.
3. **Read-only RBAC by design.** The ClusterRole grants only `get`/`list`. No
   write verbs — no automated restore — so the credential cannot mutate cluster
   state.

---

## Testing

Per project preference (controller tests over unit tests, plus targeted units for
pure logic):

- `databases/controller_test.go` — create/validate a Kubernetes database entry
  (resource-type required; namespaces required when scope = `SPECIFIC`).
- `backups/.../controllers/controller_test.go` — backup dispatch selects the
  Kubernetes usecase for `KUBERNETES`.
- **Sanitizer unit test** — pure function: asserts server-managed fields are
  stripped and `data`/`type`/`labels` preserved across a Secret and a ConfigMap.
- `helm template` smoke check — renders the ServiceAccount/ClusterRole/binding
  with defaults, and omits them when the toggles are `false`.

No E2E tests (require a live cluster; existing E2E is Postgres-only).

---

## Decisions

| Decision              | Choice                                   | Rationale                                                            |
| --------------------- | ---------------------------------------- | ------------------------------------------------------------------- |
| Cluster auth          | In-cluster ServiceAccount only           | Matches "install into the cluster"; simplest; least credential mgmt |
| Source scope          | resource types + namespace scope + names | Flexible; satisfies "any namespace" and "secret or configmap"       |
| Object selection      | Explicit names (not label selectors)     | User preference                                                     |
| Output format         | Sanitized multi-doc YAML, one file       | `kubectl apply`-ready; mirrors the single-stream Redis/RabbitMQ path |
| Restore / verify      | Omitted (download-only)                  | User requirement; keeps RBAC read-only                              |
| Helm RBAC default     | Enabled by default                       | User requirement; operator can disable via `rbac.create: false`     |
| Raw DB size           | Returns 0 MB                             | Config, not a dataset; artifact size still recorded                 |
| Backup acquisition    | Native Go via client-go                  | No bundled binaries; standard k8s client                            |

---

## Out of scope

- Automated restore back into the cluster (write RBAC, conflict handling, restore UI).
- Remote/external clusters via kubeconfig (in-cluster only).
- Resource types beyond Secrets and ConfigMaps.
- Label-selector-based selection (explicit names only).
- Namespaced `Role` alternative to the ClusterRole.

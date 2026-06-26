# Clone storage config + disable cloud notices - Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a one-click "clone storage" feature (backend endpoint + UI button) and disable all cloud-promo notices in the self-hosted deployment.

**Architecture:** Cloning runs server-side because storage secrets are masked on read and required on create, so only the backend holds the encrypted ciphertext. A new `POST /storages/:id/clone` endpoint deep-copies the source storage (including encrypted secrets, without re-encrypting) into a new row, then the frontend exposes it as a button that auto-selects the copy. Disabling notices is config-only: the existing `IS_DISABLE_CLOUD_NOTICE` flag already gates every banner app-wide.

**Tech Stack:** Go + Gin + GORM + PostgreSQL (backend); React 19 + TypeScript + Vite + Ant Design (frontend); Helm (deployment).

## Global Constraints

- **English only** in code, comments, identifiers, log messages, API strings, test assertions, commit messages.
- **Backend:** controller tests preferred over unit tests; unit tests only for model logic with no API surface. Run `make lint` (from `backend/`) after changes. Test naming: `Test_WhatWeDo_WhatWeExpect`. Time: `time.Now().UTC()`. DI structs use positional fields.
- **Frontend:** AntD 5 + `@ant-design/icons` only. Component structure order: states -> plain functions -> hooks -> calculated values -> return. User-facing copy uses plain hyphen `-`, never em/en dashes. Run `pnpm lint` and `pnpm format` (from `frontend/`) after changes. FSD layering: consumers import from a slice's `index.ts`.
- **No backward-compat shims**; no "how it was" comments.
- **Secrets never logged.** Never re-encrypt already-encrypted fields.

---

## File Structure

**Backend (clone):**
- Modify `backend/internal/features/storages/model.go` - add `Storage.Clone()` dispatcher.
- Modify each sub-model `model.go` (8 files: `local`, `s3`, `google_drive`, `nas`, `azure_blob`, `ftp`, `sftp`, `rclone`) - add a concrete `Clone()` method.
- Modify `backend/internal/features/storages/service.go` - add `StorageService.CloneStorage`.
- Modify `backend/internal/features/storages/controller.go` - add route + `CloneStorage` handler.
- Modify `backend/internal/features/storages/model_test.go` - add `Clone()` unit test.
- Modify `backend/internal/features/storages/controller_test.go` - add clone controller tests.

**Frontend (clone):**
- Modify `frontend/src/entity/storages/api/storageApi.ts` - add `cloneStorage`.
- Modify `frontend/src/features/storages/ui/StorageComponent.tsx` - clone button + handler + `onStorageCloned` prop.
- Modify `frontend/src/features/storages/ui/StoragesComponent.tsx` - wire `onStorageCloned`.

**Config (notices):**
- Modify `deploy/helm/values.yaml` - set `extraEnv` with `IS_DISABLE_CLOUD_NOTICE`.
- Modify `.env` and `.env.example` - `VITE_IS_DISABLE_CLOUD_NOTICE=true`.

---

## Task 1: Disable cloud-promo notices (config)

Config-only. Enabling `IS_DISABLE_CLOUD_NOTICE` hides every notice (storages list, storage edit form, verification agents, auth navbar, sidebar, main-screen footer) with no component code changes. Production reads the **plain** `IS_DISABLE_CLOUD_NOTICE` env at container startup (`Dockerfile` line 269); local dev reads the `VITE_` build-time variant.

**Files:**
- Modify: `deploy/helm/values.yaml:37`
- Modify: `.env:84`
- Modify: `.env.example:84`

- [ ] **Step 1: Set the Helm production env var**

In `deploy/helm/values.yaml`, replace the empty `extraEnv: []` (line 37) with:

```yaml
extraEnv:
  - name: IS_DISABLE_CLOUD_NOTICE
    value: "true"
```

Leave the surrounding comments intact.

- [ ] **Step 2: Set the local-dev build var**

In both `.env` and `.env.example`, change line 84 from:

```
VITE_IS_DISABLE_CLOUD_NOTICE=false
```

to:

```
VITE_IS_DISABLE_CLOUD_NOTICE=true
```

- [ ] **Step 3: Verify the Helm chart renders the env var**

Run: `helm template deploy/helm | grep -A1 IS_DISABLE_CLOUD_NOTICE`
Expected: output shows
```
            - name: IS_DISABLE_CLOUD_NOTICE
              value: "true"
```

(If `helm` is not installed, instead run `grep -n IS_DISABLE_CLOUD_NOTICE deploy/helm/values.yaml` and confirm the value is `"true"`.)

- [ ] **Step 4: Commit**

```bash
git add deploy/helm/values.yaml .env .env.example
git commit -m "CONFIG (storages): disable cloud-promo notices via IS_DISABLE_CLOUD_NOTICE

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Backend - `Clone()` on storage models

Pure model logic (no API surface), so this is unit-tested per the backend convention. `Clone()` is a concrete per-type method mirroring the existing `Update` methods (it is NOT part of the `StorageFileSaver` interface, because it returns a concrete type). Each sub-model struct is value-only, so a struct value copy is a sufficient deep copy; resetting `StorageID` to `uuid.Nil` lets the repository assign a fresh ID on insert.

**Files:**
- Modify: `backend/internal/features/storages/models/local/model.go`
- Modify: `backend/internal/features/storages/models/s3/model.go`
- Modify: `backend/internal/features/storages/models/google_drive/model.go`
- Modify: `backend/internal/features/storages/models/nas/model.go`
- Modify: `backend/internal/features/storages/models/azure_blob/model.go`
- Modify: `backend/internal/features/storages/models/ftp/model.go`
- Modify: `backend/internal/features/storages/models/sftp/model.go`
- Modify: `backend/internal/features/storages/models/rclone/model.go`
- Modify: `backend/internal/features/storages/model.go`
- Test: `backend/internal/features/storages/model_test.go`

**Interfaces:**
- Produces:
  - `func (s *S3Storage) Clone() *S3Storage` (and the analogous `Clone()` on `LocalStorage`, `GoogleDriveStorage`, `NASStorage`, `AzureBlobStorage`, `FTPStorage`, `SFTPStorage`, `RcloneStorage`) - returns a value copy with `StorageID = uuid.Nil`.
  - `func (s *Storage) Clone() *Storage` - returns a new `Storage` with `ID = uuid.Nil`, `Name = s.Name + " (copy)"`, `LastSaveError = nil`, copying `WorkspaceID`/`Type`/`IsSystem` and deep-copying the active sub-model.

- [ ] **Step 1: Write the failing unit test**

Append to `backend/internal/features/storages/model_test.go`:

```go
func Test_StorageClone_ResetsIdsAndCopiesSecrets(t *testing.T) {
	sourceID := uuid.New()
	workspaceID := uuid.New()
	lastError := "previous failure"

	source := &Storage{
		ID:            sourceID,
		WorkspaceID:   workspaceID,
		Type:          StorageTypeS3,
		Name:          "Prod S3",
		LastSaveError: &lastError,
		IsSystem:      true,
		S3Storage: &s3_storage.S3Storage{
			StorageID:   sourceID,
			S3Bucket:    "prod-bucket",
			S3Region:    "us-east-1",
			S3AccessKey: "encrypted-access-key",
			S3SecretKey: "encrypted-secret-key",
			S3Endpoint:  "https://s3.example.com",
			S3Prefix:    "prod",
		},
	}

	clone := source.Clone()

	assert.Equal(t, uuid.Nil, clone.ID)
	assert.Equal(t, "Prod S3 (copy)", clone.Name)
	assert.Nil(t, clone.LastSaveError)
	assert.Equal(t, workspaceID, clone.WorkspaceID)
	assert.Equal(t, StorageTypeS3, clone.Type)
	assert.True(t, clone.IsSystem)

	require.NotNil(t, clone.S3Storage)
	assert.NotSame(t, source.S3Storage, clone.S3Storage)
	assert.Equal(t, uuid.Nil, clone.S3Storage.StorageID)
	assert.Equal(t, "prod-bucket", clone.S3Storage.S3Bucket)
	assert.Equal(t, "prod", clone.S3Storage.S3Prefix)
	assert.Equal(t, "encrypted-access-key", clone.S3Storage.S3AccessKey)
	assert.Equal(t, "encrypted-secret-key", clone.S3Storage.S3SecretKey)

	// Mutating the clone must not affect the source (deep copy).
	clone.S3Storage.S3Bucket = "changed"
	assert.Equal(t, "prod-bucket", source.S3Storage.S3Bucket)
}
```

`require` is already imported in `model_test.go`.

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd backend && go test ./internal/features/storages/ -run Test_StorageClone_ResetsIdsAndCopiesSecrets`
Expected: FAIL - compile error `source.Clone undefined (type *Storage has no field or method Clone)`.

- [ ] **Step 3: Add `Clone()` to each sub-model**

In `backend/internal/features/storages/models/s3/model.go`, add after the `Update` method:

```go
func (s *S3Storage) Clone() *S3Storage {
	clone := *s
	clone.StorageID = uuid.Nil

	return &clone
}
```

Add the identical pattern (only the type name and method receiver change) to the other seven sub-models:

`models/local/model.go`:
```go
func (s *LocalStorage) Clone() *LocalStorage {
	clone := *s
	clone.StorageID = uuid.Nil

	return &clone
}
```

`models/google_drive/model.go`:
```go
func (s *GoogleDriveStorage) Clone() *GoogleDriveStorage {
	clone := *s
	clone.StorageID = uuid.Nil

	return &clone
}
```

`models/nas/model.go`:
```go
func (s *NASStorage) Clone() *NASStorage {
	clone := *s
	clone.StorageID = uuid.Nil

	return &clone
}
```

`models/azure_blob/model.go`:
```go
func (s *AzureBlobStorage) Clone() *AzureBlobStorage {
	clone := *s
	clone.StorageID = uuid.Nil

	return &clone
}
```

`models/ftp/model.go`:
```go
func (s *FTPStorage) Clone() *FTPStorage {
	clone := *s
	clone.StorageID = uuid.Nil

	return &clone
}
```

`models/sftp/model.go`:
```go
func (s *SFTPStorage) Clone() *SFTPStorage {
	clone := *s
	clone.StorageID = uuid.Nil

	return &clone
}
```

`models/rclone/model.go`:
```go
func (s *RcloneStorage) Clone() *RcloneStorage {
	clone := *s
	clone.StorageID = uuid.Nil

	return &clone
}
```

Each of these files already imports `github.com/google/uuid` (the `StorageID uuid.UUID` field). If any does not, add it to that file's import block.

- [ ] **Step 4: Add the `Storage.Clone()` dispatcher**

In `backend/internal/features/storages/model.go`, add after the `Update` method (around line 161):

```go
func (s *Storage) Clone() *Storage {
	clone := &Storage{
		WorkspaceID: s.WorkspaceID,
		Type:        s.Type,
		Name:        s.Name + " (copy)",
		IsSystem:    s.IsSystem,
	}

	switch s.Type {
	case StorageTypeLocal:
		if s.LocalStorage != nil {
			clone.LocalStorage = s.LocalStorage.Clone()
		}
	case StorageTypeS3:
		if s.S3Storage != nil {
			clone.S3Storage = s.S3Storage.Clone()
		}
	case StorageTypeGoogleDrive:
		if s.GoogleDriveStorage != nil {
			clone.GoogleDriveStorage = s.GoogleDriveStorage.Clone()
		}
	case StorageTypeNAS:
		if s.NASStorage != nil {
			clone.NASStorage = s.NASStorage.Clone()
		}
	case StorageTypeAzureBlob:
		if s.AzureBlobStorage != nil {
			clone.AzureBlobStorage = s.AzureBlobStorage.Clone()
		}
	case StorageTypeFTP:
		if s.FTPStorage != nil {
			clone.FTPStorage = s.FTPStorage.Clone()
		}
	case StorageTypeSFTP:
		if s.SFTPStorage != nil {
			clone.SFTPStorage = s.SFTPStorage.Clone()
		}
	case StorageTypeRclone:
		if s.RcloneStorage != nil {
			clone.RcloneStorage = s.RcloneStorage.Clone()
		}
	}

	return clone
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `cd backend && go test ./internal/features/storages/ -run Test_StorageClone_ResetsIdsAndCopiesSecrets`
Expected: PASS.

- [ ] **Step 6: Lint and commit**

```bash
cd backend && make lint
cd .. && git add backend/internal/features/storages/
git commit -m "FEATURE (storages): add Clone() to storage models

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Backend - clone service, controller, and route

Adds the `CloneStorage` service method (reusing the create-path permission guards), the controller handler, and the route. Controller tests prove the end-to-end behavior, including that encrypted secrets survive the clone (clone an S3 storage against the test MinIO, then test the clone's connection).

**Files:**
- Modify: `backend/internal/features/storages/service.go`
- Modify: `backend/internal/features/storages/controller.go`
- Test: `backend/internal/features/storages/controller_test.go`

**Interfaces:**
- Consumes: `Storage.Clone()` (Task 2); `storageRepository.FindByID`, `storageRepository.Save`; `workspaceService.CanUserManageDBs`; sentinel errors `ErrInsufficientPermissionsToManageStorage`, `ErrLocalStorageNotAllowedInCloudMode`, `ErrRcloneStorageRequiresAdmin`.
- Produces:
  - `func (s *StorageService) CloneStorage(user *users_models.User, id uuid.UUID) (*Storage, error)`
  - `POST /api/v1/storages/:id/clone` -> `200` with the cloned `Storage` (secrets hidden), `403` on permission errors, `400` otherwise.

- [ ] **Step 1: Write the failing controller tests**

Append to `backend/internal/features/storages/controller_test.go`:

```go
func Test_CloneStorage_WhenUserCanManage_CloneCreatedWithCopyName(t *testing.T) {
	owner := users_testing.CreateTestUser(users_enums.UserRoleMember)
	router := createRouter()
	workspace := workspaces_testing.CreateTestWorkspace("Test Workspace", owner, router)
	storage := createNewStorage(workspace.ID)

	var savedStorage Storage
	test_utils.MakePostRequestAndUnmarshal(
		t, router, "/api/v1/storages", "Bearer "+owner.Token,
		*storage, http.StatusOK, &savedStorage,
	)

	var clonedStorage Storage
	test_utils.MakePostRequestAndUnmarshal(
		t, router,
		fmt.Sprintf("/api/v1/storages/%s/clone", savedStorage.ID.String()),
		"Bearer "+owner.Token, nil, http.StatusOK, &clonedStorage,
	)

	assert.NotEqual(t, savedStorage.ID, clonedStorage.ID)
	assert.NotEmpty(t, clonedStorage.ID)
	assert.Equal(t, savedStorage.Name+" (copy)", clonedStorage.Name)
	assert.Equal(t, workspace.ID, clonedStorage.WorkspaceID)
	assert.Equal(t, StorageTypeLocal, clonedStorage.Type)

	deleteStorage(t, router, clonedStorage.ID, owner.Token)
	deleteStorage(t, router, savedStorage.ID, owner.Token)
	workspaces_testing.RemoveTestWorkspace(workspace, router)
}

func Test_CloneStorage_WhenUserCannotManage_ReturnsForbidden(t *testing.T) {
	owner := users_testing.CreateTestUser(users_enums.UserRoleMember)
	outsider := users_testing.CreateTestUser(users_enums.UserRoleMember)
	router := createRouter()
	workspace := workspaces_testing.CreateTestWorkspace("Test Workspace", owner, router)
	storage := createNewStorage(workspace.ID)

	var savedStorage Storage
	test_utils.MakePostRequestAndUnmarshal(
		t, router, "/api/v1/storages", "Bearer "+owner.Token,
		*storage, http.StatusOK, &savedStorage,
	)

	response := test_utils.MakePostRequest(
		t, router,
		fmt.Sprintf("/api/v1/storages/%s/clone", savedStorage.ID.String()),
		"Bearer "+outsider.Token, nil, http.StatusForbidden,
	)
	assert.Contains(t, string(response.Body), "error")

	deleteStorage(t, router, savedStorage.ID, owner.Token)
	workspaces_testing.RemoveTestWorkspace(workspace, router)
}

func Test_CloneStorage_S3SecretsCopied_CloneTestConnectionSucceeds(t *testing.T) {
	validateEnvVariables(t)
	s3Container, err := setupS3Container(t.Context())
	require.NoError(t, err, "Failed to setup S3 container")

	owner := users_testing.CreateTestUser(users_enums.UserRoleMember)
	router := createRouter()
	workspace := workspaces_testing.CreateTestWorkspace("Test Workspace", owner, router)

	storage := &Storage{
		WorkspaceID: workspace.ID,
		Type:        StorageTypeS3,
		Name:        "S3 source " + uuid.New().String(),
		S3Storage: &s3_storage.S3Storage{
			S3Bucket:    s3Container.bucketName,
			S3Region:    s3Container.region,
			S3AccessKey: s3Container.accessKey,
			S3SecretKey: s3Container.secretKey,
			S3Endpoint:  "http://" + s3Container.endpoint,
		},
	}

	var savedStorage Storage
	test_utils.MakePostRequestAndUnmarshal(
		t, router, "/api/v1/storages", "Bearer "+owner.Token,
		*storage, http.StatusOK, &savedStorage,
	)

	var clonedStorage Storage
	test_utils.MakePostRequestAndUnmarshal(
		t, router,
		fmt.Sprintf("/api/v1/storages/%s/clone", savedStorage.ID.String()),
		"Bearer "+owner.Token, nil, http.StatusOK, &clonedStorage,
	)

	// Empty access/secret keys in the response prove they were hidden, not lost.
	assert.Empty(t, clonedStorage.S3Storage.S3AccessKey)

	// Testing the clone's connection proves the encrypted secrets were copied.
	response := test_utils.MakePostRequest(
		t, router,
		fmt.Sprintf("/api/v1/storages/%s/test", clonedStorage.ID.String()),
		"Bearer "+owner.Token, nil, http.StatusOK,
	)
	assert.Contains(t, string(response.Body), "successful")

	deleteStorage(t, router, clonedStorage.ID, owner.Token)
	deleteStorage(t, router, savedStorage.ID, owner.Token)
	workspaces_testing.RemoveTestWorkspace(workspace, router)
}
```

(`setupS3Container`, `validateEnvVariables`, `require`, and `s3_storage` are already defined/imported in the `storages` test package.)

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/features/storages/ -run Test_CloneStorage`
Expected: FAIL - the clone requests return `404` (route not registered), so the unmarshal/status assertions fail.

- [ ] **Step 3: Add the `CloneStorage` service method**

In `backend/internal/features/storages/service.go`, add after `SaveStorage` (around line 151):

```go
func (s *StorageService) CloneStorage(
	user *users_models.User,
	id uuid.UUID,
) (*Storage, error) {
	source, err := s.storageRepository.FindByID(id)
	if err != nil {
		return nil, err
	}

	canManage, err := s.workspaceService.CanUserManageDBs(source.WorkspaceID, user)
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, ErrInsufficientPermissionsToManageStorage
	}

	if config.GetEnv().IsCloud && source.Type == StorageTypeLocal &&
		user.Role != users_enums.UserRoleAdmin {
		return nil, ErrLocalStorageNotAllowedInCloudMode
	}

	if source.Type == StorageTypeRclone && user.Role != users_enums.UserRoleAdmin {
		return nil, ErrRcloneStorageRequiresAdmin
	}

	if source.IsSystem && user.Role != users_enums.UserRoleAdmin {
		return nil, ErrInsufficientPermissionsToManageStorage
	}

	clone := source.Clone()

	if _, err := s.storageRepository.Save(clone); err != nil {
		return nil, err
	}

	s.auditLogService.WriteAuditLog(
		fmt.Sprintf("Storage cloned: %s from %s", clone.Name, source.Name),
		&user.ID,
		&source.WorkspaceID,
	)

	clone.HideSensitiveData()

	return clone, nil
}
```

(`config`, `users_models`, `users_enums`, `fmt`, `uuid` are already imported in `service.go`.)

- [ ] **Step 4: Add the controller route and handler**

In `backend/internal/features/storages/controller.go`, add the route inside `RegisterRoutes` (after the `/transfer` line, around line 25):

```go
	router.POST("/storages/:id/clone", c.CloneStorage)
```

Then add the handler (place it after `SaveStorage`):

```go
// CloneStorage
// @Summary Clone a storage
// @Description Duplicate an existing storage, including its credentials, within the same workspace
// @Tags storages
// @Produce json
// @Param Authorization header string true "JWT token"
// @Param id path string true "Storage ID"
// @Success 200 {object} Storage
// @Failure 400
// @Failure 401
// @Failure 403
// @Router /storages/{id}/clone [post]
func (c *StorageController) CloneStorage(ctx *gin.Context) {
	user, ok := users_middleware.GetUserFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid storage ID"})
		return
	}

	clone, err := c.storageService.CloneStorage(user, id)
	if err != nil {
		if errors.Is(err, ErrInsufficientPermissionsToManageStorage) ||
			errors.Is(err, ErrLocalStorageNotAllowedInCloudMode) ||
			errors.Is(err, ErrRcloneStorageRequiresAdmin) {
			ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, clone)
}
```

(`errors`, `net/http`, `gin`, `uuid`, `users_middleware` are already imported in `controller.go`.)

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/features/storages/ -run Test_CloneStorage`
Expected: PASS (all three). If the S3 test errors with a MinIO connection failure, ensure the test stack is up (`docker compose up -d` for the test MinIO) and re-run.

- [ ] **Step 6: Lint and commit**

```bash
cd backend && make lint
cd .. && git add backend/internal/features/storages/
git commit -m "FEATURE (storages): add clone storage endpoint

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Frontend - clone API client, button, and wiring

Adds the API call, a clone button in the storage detail action row, and the parent wiring to reload + auto-select the new copy.

**Files:**
- Modify: `frontend/src/entity/storages/api/storageApi.ts`
- Modify: `frontend/src/features/storages/ui/StorageComponent.tsx`
- Modify: `frontend/src/features/storages/ui/StoragesComponent.tsx`

**Interfaces:**
- Consumes: `POST /storages/:id/clone` (Task 3).
- Produces: `storageApi.cloneStorage(id: string): Promise<Storage>`; new `StorageComponent` prop `onStorageCloned: (newStorage: Storage) => void`.

- [ ] **Step 1: Add the API method**

In `frontend/src/entity/storages/api/storageApi.ts`, add inside the `storageApi` object (after `transferStorage`):

```ts
  async cloneStorage(id: string) {
    const requestOptions: RequestOptions = new RequestOptions();
    return apiHelper.fetchPostJson<Storage>(
      `${getApplicationServer()}/api/v1/storages/${id}/clone`,
      requestOptions,
    );
  },
```

- [ ] **Step 2: Add the clone handler and button in `StorageComponent.tsx`**

Add `CopyOutlined` to the icon import at the top of `frontend/src/features/storages/ui/StorageComponent.tsx`:

```ts
import {
  ArrowRightOutlined,
  CloseOutlined,
  CopyOutlined,
  DeleteOutlined,
  InfoCircleOutlined,
} from '@ant-design/icons';
```

Add `onStorageCloned` to the `Props` interface:

```ts
interface Props {
  storageId: string;
  onStorageChanged: (storage: Storage) => void;
  onStorageDeleted: () => void;
  onStorageTransferred: () => void;
  onStorageCloned: (newStorage: Storage) => void;
  isCanManageStorages: boolean;
  user: UserProfile;
}
```

Destructure it in the component signature:

```ts
export const StorageComponent = ({
  storageId,
  onStorageChanged,
  onStorageDeleted,
  onStorageTransferred,
  onStorageCloned,
  isCanManageStorages,
  user,
}: Props) => {
```

Add an `isCloning` state next to the other states (after `isRemoving`):

```ts
  const [isCloning, setIsCloning] = useState(false);
```

Add the `clone` handler next to the other plain functions (after `remove`):

```ts
  const clone = () => {
    if (!storage) return;

    setIsCloning(true);
    storageApi
      .cloneStorage(storage.id)
      .then((clonedStorage: Storage) => {
        ToastHelper.showToast({
          title: 'Storage cloned',
          description: 'A copy of the storage was created',
        });
        onStorageCloned(clonedStorage);
      })
      .catch((e: Error) => {
        alert(e.message);
      })
      .finally(() => {
        setIsCloning(false);
      });
  };
```

In the action-button row, add the clone button inside the `isCanManageStorages` block, before the transfer button (around line 280):

```tsx
                {isCanManageStorages && (
                  <>
                    {!storage.isSystem && (
                      <Button
                        type="primary"
                        ghost
                        icon={<CopyOutlined />}
                        onClick={clone}
                        loading={isCloning}
                        disabled={isCloning}
                        className="mr-1"
                      />
                    )}

                    {!storage.isSystem && (
                      <Button
                        type="primary"
                        ghost
                        icon={<ArrowRightOutlined />}
                        onClick={() => setIsShowTransferDialog(true)}
                        className="mr-1"
                      />
                    )}
```

(Keep the rest of the block - the delete button - unchanged.)

- [ ] **Step 3: Wire `onStorageCloned` in `StoragesComponent.tsx`**

In `frontend/src/features/storages/ui/StoragesComponent.tsx`, add the prop to the `<StorageComponent>` usage (after `onStorageTransferred`, around line 173):

```tsx
              onStorageCloned={(newStorage) => {
                loadStorages(false, newStorage.id);
              }}
```

- [ ] **Step 4: Lint, format, and build**

Run:
```bash
cd frontend && pnpm format && pnpm lint && pnpm build
```
Expected: format and lint pass with no errors; build succeeds.

- [ ] **Step 5: Manual verification**

Start the app, open a workspace with a non-system storage, select it, and confirm:
- A copy icon button appears in the action row (for users who can manage storages).
- Clicking it creates `<name> (copy)`, shows the "Storage cloned" toast, and auto-selects the new copy in the list.
- Testing the clone's connection succeeds (secrets were copied).

- [ ] **Step 6: Commit**

```bash
git add frontend/src/entity/storages/api/storageApi.ts \
  frontend/src/features/storages/ui/StorageComponent.tsx \
  frontend/src/features/storages/ui/StoragesComponent.tsx
git commit -m "FEATURE (storages): add clone storage button

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:**
- Feature A (disable notices, Helm + .env) -> Task 1. ✓
- Feature B backend (endpoint, service, model `Clone()`, no re-encrypt, audit log, hidden secrets in response) -> Tasks 2 & 3. ✓
- Feature B frontend (API method, clone button, auto-select) -> Task 4. ✓
- Testing (controller test cloning S3 then testing connection; permission 403; cleanup) -> Task 3. ✓

**Type consistency:** `Storage.Clone()` and per-model `Clone()` signatures match between Task 2 (definition) and Task 3 (consumption). `CloneStorage(user, id)` signature matches between service (Task 3 Step 3) and controller (Task 3 Step 4). Frontend `cloneStorage(id)` and `onStorageCloned(newStorage)` match between Task 4 steps.

**Placeholder scan:** No TBD/TODO/"handle errors" placeholders; every code step shows complete code.

**Notes for the implementer:**
- The S3 clone test (Task 3) requires the test MinIO from the dev compose stack to be running.
- Cloning a system storage is intentionally restricted to admins and the UI hides the clone button for system storages (Task 4 Step 2), consistent with the transfer button.

# Clone storage config + disable cloud notices - design

Date: 2026-06-26

## Overview

Two independent changes to the storages area, shipped together:

- **A. Disable cloud-promo notices** - config only. Flip the existing
  `IS_DISABLE_CLOUD_NOTICE` flag on, which hides every cloud upsell across the
  app. No component code changes.
- **B. Clone a storage** - a new backend `POST /storages/:id/clone` endpoint
  that server-side duplicates a storage including its encrypted secrets, plus a
  clone button in the storage detail panel.

The two changes share no code. They may be one PR with two commits, or two PRs.

## Feature A - Disable cloud notices

### Background

`IS_DISABLE_CLOUD_NOTICE` already exists. Every notice block is gated on
`!IS_CLOUD && !IS_DISABLE_CLOUD_NOTICE`, so enabling the flag removes all of
them at once. Current notice locations:

- `frontend/src/features/storages/ui/StoragesComponent.tsx` (storages list)
- `frontend/src/features/storages/ui/edit/EditStorageComponent.tsx` (storage edit form)
- `frontend/src/features/verification/agents/ui/VerificationAgentsComponent.tsx`
- `frontend/src/features/users/ui/AuthNavbarComponent.tsx`
- `frontend/src/widgets/main/SidebarComponent.tsx`
- `frontend/src/widgets/main/MainScreenComponent.tsx`

The flag is read at container startup into `runtime-config.js`
(`Dockerfile` line 269) from the plain `IS_DISABLE_CLOUD_NOTICE` env var
(not the `VITE_` build-time variant). Runtime config works regardless of how
the image was built.

### Decision

Disable all cloud-promo notices app-wide. This is the intended mechanism and
keeps the codebase identical to upstream (clean future merges).

### Changes

- **Primary (production, Helm):** add to `deploy/helm/values.yaml` `extraEnv`:

  ```yaml
  extraEnv:
    - name: IS_DISABLE_CLOUD_NOTICE
      value: "true"
  ```

  The chart already supports `extraEnv` and threads it into the StatefulSet
  container env (`deploy/helm/templates/statefulset.yaml`).

- **Optional (local dev):** set `VITE_IS_DISABLE_CLOUD_NOTICE=true` in `.env`
  and `.env.example` so developers do not see the banners locally.

No `.tsx` file is touched.

## Feature B - Clone storage

### Why this must be server-side

`StorageService.GetStorage` calls `HideSensitiveData()`, which blanks secrets
(e.g. S3 access/secret key, passwords) before returning. `Validate()` requires
those secrets to be non-empty on create. The frontend therefore never holds the
real secrets and cannot reconstruct them. Cloning must run on the backend where
the encrypted ciphertext is available in the DB.

### Backend

**Endpoint:** `POST /api/v1/storages/:id/clone`

- Registered in `StorageController.RegisterRoutes`.
- Returns the created `Storage` with secrets hidden (same shape as `GetStorage`).
- Error mapping mirrors `SaveStorage`: permission errors -> `403`, others -> `400`.

**Service:** `StorageService.CloneStorage(user, id) (*Storage, error)`

1. `repository.FindByID(id)` - loads the source with encrypted secrets and the
   type-specific sub-model preloaded.
2. Permission and guard checks identical to the create path in `SaveStorage`:
   - `workspaceService.CanUserManageDBs(source.WorkspaceID, user)` else
     `ErrInsufficientPermissionsToManageStorage`.
   - In cloud mode, local storage requires admin
     (`ErrLocalStorageNotAllowedInCloudMode`).
   - Rclone storage requires admin (`ErrRcloneStorageRequiresAdmin`).
   - System storage requires admin (`ErrInsufficientPermissionsToManageStorage`).
   No new permission concept is introduced.
3. Build the clone via a new `Storage.Clone()` method (see below).
4. `repository.Save(clone)` - the existing nil-ID branch creates fresh rows for
   the storage and its sub-model in one transaction.
5. Audit log: `"Storage cloned: <newName> from <sourceName>"` via
   `auditLogService.WriteAuditLog`.
6. Return `clone` with `HideSensitiveData()` applied.

**Model:** `Storage.Clone() *Storage`

- Returns a new `Storage` with:
  - `ID = uuid.Nil` (Postgres assigns a fresh UUID via `gen_random_uuid()` on insert).
  - `Name = "<source name> (copy)"`.
  - `LastSaveError = nil`.
  - `WorkspaceID`, `Type`, `IsSystem` copied from source.
  - The type-specific sub-struct deep-copied with `StorageID` reset to
    `uuid.Nil`. Encrypted secret fields are copied verbatim - **no call to
    `EncryptSensitiveData`** (re-encrypting already-encrypted ciphertext would
    corrupt it). The S3 prefix and all other config fields are copied as-is.
- Add a `Clone()` method to each storage sub-model (`s3`, `local`,
  `google_drive`, `nas`, `azure_blob`, `ftp`, `sftp`, `rclone`), mirroring the
  existing per-type `Update` / `HideSensitiveData` / `EncryptSensitiveData`
  methods, plus a dispatcher `Storage.Clone()` that selects by `Type`. Each
  sub-model `Clone()` returns a value copy with `StorageID` zeroed.
  - Implementation note: sub-model structs are value-field-only (strings,
    bools, ints), so a struct value copy is a sufficient deep copy. If any
    sub-model gains a reference-type field (slice/map/pointer) later, its
    `Clone()` must copy that field explicitly.

### Frontend

- `storageApi.cloneStorage(id: string)` in
  `frontend/src/entity/storages/api/storageApi.ts` - `POST /storages/:id/clone`,
  returns `Storage`.
- `StorageComponent.tsx`:
  - Add an `isCloning` state and a `clone` handler that calls the API, shows a
    success toast (`"Storage cloned"`), and invokes a new `onStorageCloned`
    prop.
  - Add a clone button (`CopyOutlined`) to the action row next to Test
    connection / transfer / delete, gated by `isCanManageStorages`. No
    confirmation dialog (cloning is non-destructive).
- `StoragesComponent.tsx`: pass `onStorageCloned={(newStorage) =>
  loadStorages(false, newStorage.id)}` so the list reloads and auto-selects the
  new copy (same pattern as the Add flow).

## Testing

Backend, controller tests against real containers (repo convention):

- `Test_CloneStorage_WhenUserCanManage_StorageCloned` - create an S3 storage
  against the test MinIO, clone it via the API, then `POST
  /storages/:cloneId/test` succeeds. This proves the encrypted secrets were
  copied correctly end-to-end. Assert the clone has a new id, the
  `"<name> (copy)"` name, and the same workspace.
- Permission-failure test - a user without manage rights gets `403`.
- Not-found / cross-workspace source - returns an error.
- Clean up created storages via the DELETE endpoint.

Frontend: manual verification that the clone button appears for users who can
manage storages, creates a copy, and auto-selects it.

## Out of scope

- Per-page scoping of the notice flag - it is global by design.
- Cloning does not copy attached databases or backups; it duplicates the
  storage config only.
- Two storages may point at the same S3 bucket/prefix after cloning. This is
  expected (a clone is a config starting point), and the prefix is immutable
  after creation anyway.

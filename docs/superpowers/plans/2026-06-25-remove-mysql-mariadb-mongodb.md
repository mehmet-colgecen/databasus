# Remove MySQL, MariaDB, MongoDB Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove MySQL, MariaDB, and MongoDB as supported database types end to end, leaving PostgreSQL, Redis, RabbitMQ, and Kubernetes fully working.

**Architecture:** The database type is a plain `TEXT` enum (`DatabaseType` in Go, `DatabaseType` in TS) with no DB CHECK constraint. Every feature dispatches off it with a `switch`/`Preload`/struct-field-per-type pattern, and each type has self-contained per-type packages/folders. Removal = delete the per-type packages, strip the three cases from each shared dispatch site, drop the three Postgres tables via one forward goose migration, and remove the bundled client binaries + test infra.

**Tech Stack:** Go (Gin + GORM + goose migrations), React 19 + TypeScript (Vite + AntD), Docker, GitHub Actions.

## Global Constraints

- **English only** in code, comments, identifiers, log messages, commit messages.
- **No backward-compat shims, deprecation aliases, or data export** — delete outright.
- **Keep the four remaining types intact**: `POSTGRES`, `REDIS`, `RABBITMQ`, `KUBERNETES`. In every shared file, remove only the `MYSQL`/`MARIADB`/`MONGODB` branches.
- **Frontend verification uses a real build** (`pnpm build`), never `tsc --noEmit`.
- **Migrations are PostgreSQL + goose**; create stubs with `make migration-create name=...`, never hand-name the timestamp.
- **Backend lint/build gate:** `cd backend && make lint && go build ./...`.
- This is a removal, so the TDD cycle is adapted to: **edit → verify build/lint/tests green and grep clean → commit**. There is no new behavior to test first.

---

### Task 1: Remove MySQL/MariaDB/MongoDB from the Go backend + agent

**Files:**

- Delete (whole folders/files):
  - `backend/internal/features/databases/databases/mysql/`
  - `backend/internal/features/databases/databases/mariadb/`
  - `backend/internal/features/databases/databases/mongodb/`
  - `backend/internal/features/backups/backups/usecases/mysql/`
  - `backend/internal/features/backups/backups/usecases/mariadb/`
  - `backend/internal/features/backups/backups/usecases/mongodb/`
  - `backend/internal/features/restores/usecases/mysql/`
  - `backend/internal/features/restores/usecases/mariadb/`
  - `backend/internal/features/restores/usecases/mongodb/`
  - `backend/internal/util/tools/mysql.go`
  - `backend/internal/util/tools/mariadb.go`
  - `backend/internal/util/tools/mongodb.go`
  - `backend/internal/features/tests/mysql_backup_restore_test.go`
  - `backend/internal/features/tests/mariadb_backup_restore_test.go`
  - `backend/internal/features/tests/mongodb_backup_restore_test.go`
- Modify (remove only the three branches/fields/imports):
  - `backend/internal/features/databases/enums.go`
  - `backend/internal/features/databases/model.go`
  - `backend/internal/features/databases/repository.go`
  - `backend/internal/features/databases/service.go`
  - `backend/internal/features/databases/testing.go`
  - `backend/internal/features/databases/controller_test.go`
  - `backend/internal/features/backups/backups/usecases/create_backup_uc.go`
  - `backend/internal/features/backups/backups/usecases/di.go`
  - `backend/internal/features/backups/backups/streaming/stream.go`
  - `backend/internal/features/backups/backups/backuping/scheduler_test.go`
  - `backend/internal/features/restores/restoring/restorer.go`
  - `backend/internal/features/restores/restoring/scheduler.go`
  - `backend/internal/features/restores/restoring/dto.go`
  - `backend/internal/features/restores/service.go`
  - `backend/internal/features/restores/usecases/di.go`
  - `backend/internal/features/restores/usecases/restore_backup_uc.go`
  - `backend/internal/features/restores/core/dto.go`
  - `backend/internal/features/restores/core/model.go`
  - `backend/internal/features/restores/core/repository.go`
  - `backend/internal/features/restores/controller_test.go`
  - `backend/internal/features/healthcheck/attempt/check_database_health_uc.go`
  - `backend/internal/features/telemetry/service.go`
  - `backend/internal/features/telemetry/service_test.go`
  - `backend/internal/util/tools/common.go`
  - `backend/internal/config/config.go`
  - `backend/internal/features/tests/ssl_backup_restore_test.go`
  - `backend/internal/features/tests/backup_hang_edge_cases_test.go`
  - `agent/verification/internal/features/runner/runner_test.go`

**Interfaces:**

- Consumes: nothing (first task).
- Produces: a `DatabaseType` enum (`backend/internal/features/databases/enums.go`) whose only values are `DatabaseTypePostgres`, `DatabaseTypeRedis`, `DatabaseTypeRabbitmq`, `DatabaseTypeKubernetes`. Task 2's migration deletes rows matching the removed values.

**Note on ordering:** `go build ./...` will be red mid-task (deleting the `Database.Mysql` field breaks the still-present `usecases/mysql` package, etc.). That is expected — make all edits and deletions, then build once at the end.

- [ ] **Step 1: Delete the self-contained per-type packages and test files**

```bash
cd backend
git rm -r \
  internal/features/databases/databases/mysql \
  internal/features/databases/databases/mariadb \
  internal/features/databases/databases/mongodb \
  internal/features/backups/backups/usecases/mysql \
  internal/features/backups/backups/usecases/mariadb \
  internal/features/backups/backups/usecases/mongodb \
  internal/features/restores/usecases/mysql \
  internal/features/restores/usecases/mariadb \
  internal/features/restores/usecases/mongodb \
  internal/util/tools/mysql.go \
  internal/util/tools/mariadb.go \
  internal/util/tools/mongodb.go \
  internal/features/tests/mysql_backup_restore_test.go \
  internal/features/tests/mariadb_backup_restore_test.go \
  internal/features/tests/mongodb_backup_restore_test.go
```

- [ ] **Step 2: Remove the three enum constants**

In `backend/internal/features/databases/enums.go`, delete the `DatabaseTypeMysql`, `DatabaseTypeMariadb`, and `DatabaseTypeMongodb` lines. Result:

```go
const (
	DatabaseTypePostgres   DatabaseType = "POSTGRES"
	DatabaseTypeRedis      DatabaseType = "REDIS"
	DatabaseTypeRabbitmq   DatabaseType = "RABBITMQ"
	DatabaseTypeKubernetes DatabaseType = "KUBERNETES"
)
```

- [ ] **Step 3: Strip the three branches from the databases aggregate**

In `backend/internal/features/databases/model.go`:
- Remove the three imports (`databases/databases/mariadb`, `.../mongodb`, `.../mysql`).
- Remove the `Mysql`, `Mariadb`, `Mongodb` struct fields from `Database`.
- Remove the `DatabaseTypeMysql`/`DatabaseTypeMariadb`/`DatabaseTypeMongodb` `case` blocks from `Validate`, `IsUserReadOnly`, `Update`, and `getSpecificDatabase`.
- Remove the three `if d.Mysql != nil { ... }` / `Mariadb` / `Mongodb` blocks from `EncryptSensitiveFields` and `PopulateDbData`.

In `backend/internal/features/databases/repository.go`:
- Remove the three imports (`.../mysql`, `.../mariadb`, `.../mongodb`).
- Remove the `DatabaseTypeMysql`/`Mariadb`/`Mongodb` `case` blocks from both `switch` statements in `Save` and from the `switch` in `Delete`.
- Remove `"Mysql"`, `"Mariadb"`, `"Mongodb"` from both `.Omit(...)` lists.
- Remove the three `Preload("Mysql")` / `Preload("Mariadb")` / `Preload("Mongodb")` calls in `FindByID`, `FindByWorkspaceID`, and `GetAllDatabases`.

In `backend/internal/features/databases/service.go`, `testing.go`, and `controller_test.go`: remove every `mysql`/`mariadb`/`mongodb` reference (helper builders, test cases, validation branches) for the three types, keeping the kept-type equivalents.

- [ ] **Step 4: Strip the three branches from backup + restore dispatch**

In `backend/internal/features/backups/backups/usecases/create_backup_uc.go`: remove the three `usecases_mysql`/`usecases_mariadb`/`usecases_mongodb` imports, the `CreateMysqlBackupUsecase`/`CreateMariadbBackupUsecase`/`CreateMongodbBackupUsecase` struct fields, and the three `case` blocks in `Execute`.

In `backend/internal/features/backups/backups/usecases/di.go`: remove the construction/wiring of the three removed usecases.

In `backend/internal/features/backups/backups/streaming/stream.go` and `backuping/scheduler_test.go`: remove the three-type references.

In the restores feature (`restoring/restorer.go`, `restoring/scheduler.go`, `restoring/dto.go`, `service.go`, `usecases/di.go`, `usecases/restore_backup_uc.go`, `core/dto.go`, `core/model.go`, `core/repository.go`, `controller_test.go`): remove the three imports, struct fields, `case` blocks, and test cases for MySQL/MariaDB/MongoDB.

- [ ] **Step 5: Strip the three branches from healthcheck, telemetry, and tools**

In `backend/internal/features/healthcheck/attempt/check_database_health_uc.go`: remove the three `case` blocks.

In `backend/internal/features/telemetry/service.go`, remove the three `case` blocks in `buildDatabaseEntry`. Result:

```go
func buildDatabaseEntry(db *databases.Database) (DatabaseEntry, bool) {
	switch db.Type {
	case databases.DatabaseTypePostgres:
		if db.Postgresql == nil {
			return DatabaseEntry{}, false
		}
		return DatabaseEntry{Type: string(db.Type), Version: string(db.Postgresql.Version)}, true
	}

	return DatabaseEntry{}, false
}
```

In `backend/internal/features/telemetry/service_test.go`: remove assertions/fixtures for the three types.

In `backend/internal/util/tools/common.go`: remove the `checkMysql()`, `checkMariadb()`, and `checkMongodb()` calls, and update the doc comment on line ~24 so it no longer lists MySQL/MariaDB/MongoDB.

- [ ] **Step 6: Remove the test-port config fields**

In `backend/internal/config/config.go`: delete the `TestMysql*Port`, `TestMariadb*Port`, `TestMongodb*Port`, `TestMysqlSslPort`, `TestMariadbSslPort`, and `TestMongodbSslPort` fields (lines ~87-116).

- [ ] **Step 7: Strip the three branches from the SSL + edge-case tests and the agent test**

In `backend/internal/features/tests/ssl_backup_restore_test.go` and `backup_hang_edge_cases_test.go`: keep the PostgreSQL path, remove the MySQL/MariaDB/MongoDB test cases and helpers.

In `agent/verification/internal/features/runner/runner_test.go`: remove the MySQL/MongoDB references.

- [ ] **Step 8: Build and lint**

Run:

```bash
cd backend && make lint && go build ./...
cd ../agent/verification && go build ./...
```

Expected: both succeed with no errors. Fix any remaining references the compiler flags (missing import, undefined `DatabaseTypeMysql`, etc.) until green.

- [ ] **Step 9: Run the type-relevant unit tests**

Run (container-independent tests):

```bash
cd backend && go test ./internal/features/telemetry/... ./internal/features/databases/... 2>&1 | tail -20
```

Expected: PASS (or, for tests needing a live Postgres/agent, they are skipped/connection-gated — no compile failures). Note any test that requires a running DB so the executor can run it with infra up.

- [ ] **Step 10: Grep sweep for backend code**

Run:

```bash
grep -rniE 'mysql|mariadb|mongo' backend/internal backend/cmd agent
```

Expected: **zero matches**. (Historical migration files under `backend/migrations/` are intentionally untouched and excluded here.)

- [ ] **Step 11: Commit**

```bash
git add -A
git commit -m "FEATURE (databases): remove MySQL, MariaDB, MongoDB from the backend"
```

---

### Task 2: Add the cleanup migration

**Files:**
- Create: `backend/migrations/<goose-timestamp>_drop_mysql_mariadb_mongodb_databases.sql`

**Interfaces:**
- Consumes: the four-value enum from Task 1 (the deleted string values `'MYSQL'`, `'MARIADB'`, `'MONGODB'` are now orphaned data).
- Produces: a database with no `mysql_databases` / `mariadb_databases` / `mongodb_databases` tables and no `databases` rows of those types.

- [ ] **Step 1: Generate the migration stub**

```bash
cd backend && make migration-create name=drop_mysql_mariadb_mongodb_databases
```

Expected: prints the path of a new file under `migrations/` named `<timestamp>_drop_mysql_mariadb_mongodb_databases.sql`.

- [ ] **Step 2: Fill in the migration**

Replace the stub contents with:

```sql
-- +goose Up
-- +goose StatementBegin
-- Remove all sources of the dropped types. ON DELETE CASCADE on database_id
-- (per-type tables, backups, healthcheck_configs/attempts, restores, notifiers)
-- cleans up every dependent row.
DELETE FROM databases WHERE type IN ('MYSQL', 'MARIADB', 'MONGODB');
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS mysql_databases;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS mariadb_databases;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS mongodb_databases;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- No-op: the dropped tables and deleted rows are unrecoverable. MySQL, MariaDB,
-- and MongoDB support has been removed from the application entirely.
SELECT 1;
-- +goose StatementEnd
```

- [ ] **Step 3: Verify each dependent table cascades**

Confirm the `database_id` foreign keys on `backups`, `healthcheck_configs`, `healthcheck_attempts`, `restores`/restore-verification tables, and `database_notifiers` are `ON DELETE CASCADE` (init migration shows this pattern). If any child table references `databases(id)` without cascade, add an explicit `DELETE FROM <child> WHERE database_id IN (SELECT id FROM databases WHERE type IN (...))` before the `databases` delete.

Run:

```bash
grep -rniE 'references databases|database_id' backend/migrations/*.sql | grep -iE 'fk_|references databases'
```

Expected: every FK to `databases(id)` is followed by `ON DELETE CASCADE`. Record any exception and patch the migration.

- [ ] **Step 4: Apply the migration to a fresh test DB**

Run (with Postgres up and `GOOSE_TEST_DBSTRING`/`GOOSE_DBSTRING` set per the repo's dev setup):

```bash
cd backend && make migration-up-test
```

Expected: the new migration applies without error. Then verify:

```bash
psql "$GOOSE_TEST_DBSTRING" -c "\dt mysql_databases mariadb_databases mongodb_databases"
psql "$GOOSE_TEST_DBSTRING" -c "SELECT DISTINCT type FROM databases;"
```

Expected: the three tables do **not** exist; `type` contains only kept values.

- [ ] **Step 5: Commit**

```bash
git add backend/migrations/
git commit -m "FEATURE (databases): drop mysql/mariadb/mongodb tables and rows"
```

---

### Task 3: Remove MySQL/MariaDB/MongoDB from the frontend

**Files:**
- Delete (whole folders/files):
  - `frontend/src/entity/databases/model/mysql/`
  - `frontend/src/entity/databases/model/mariadb/`
  - `frontend/src/entity/databases/model/mongodb/`
  - `frontend/src/features/databases/ui/edit/EditMySqlSpecificDataComponent.tsx`
  - `frontend/src/features/databases/ui/edit/EditMariaDbSpecificDataComponent.tsx`
  - `frontend/src/features/databases/ui/edit/EditMongoDbSpecificDataComponent.tsx`
  - `frontend/src/features/databases/ui/show/ShowMySqlSpecificDataComponent.tsx`
  - `frontend/src/features/databases/ui/show/ShowMariaDbSpecificDataComponent.tsx`
  - `frontend/src/features/databases/ui/show/ShowMongoDbSpecificDataComponent.tsx`
- Modify (remove only the three branches/members/imports):
  - `frontend/src/entity/databases/model/DatabaseType.ts`
  - `frontend/src/entity/databases/model/Database.ts`
  - `frontend/src/entity/databases/model/getDatabaseLogoFromType.ts`
  - `frontend/src/entity/databases/index.ts`
  - `frontend/src/features/databases/ui/CreateDatabaseComponent.tsx`
  - `frontend/src/features/databases/ui/edit/EditDatabaseBaseInfoComponent.tsx`
  - `frontend/src/features/databases/ui/edit/EditDatabaseSpecificDataComponent.tsx`
  - `frontend/src/features/databases/ui/edit/CreateReadOnlyComponent.tsx`
  - `frontend/src/features/databases/ui/show/ShowDatabaseBaseInfoComponent.tsx`
  - `frontend/src/features/databases/ui/show/ShowDatabaseSpecificDataComponent.tsx`
  - `frontend/src/features/billing/models/purchaseUtils.ts`
  - `frontend/src/entity/restores/api/restoreApi.ts`
  - `frontend/src/features/backups/ui/BackupsComponent.tsx`
  - `frontend/src/features/restores/ui/RestoresComponent.tsx`

**Interfaces:**
- Consumes: nothing from the frontend's perspective; the backend no longer accepts these types (Task 1).
- Produces: a `DatabaseType` enum (`frontend/src/entity/databases/model/DatabaseType.ts`) with only `POSTGRES`, `REDIS`, `RABBITMQ`, `KUBERNETES`.

- [ ] **Step 1: Delete per-type model folders and components**

```bash
cd frontend
git rm -r \
  src/entity/databases/model/mysql \
  src/entity/databases/model/mariadb \
  src/entity/databases/model/mongodb \
  src/features/databases/ui/edit/EditMySqlSpecificDataComponent.tsx \
  src/features/databases/ui/edit/EditMariaDbSpecificDataComponent.tsx \
  src/features/databases/ui/edit/EditMongoDbSpecificDataComponent.tsx \
  src/features/databases/ui/show/ShowMySqlSpecificDataComponent.tsx \
  src/features/databases/ui/show/ShowMariaDbSpecificDataComponent.tsx \
  src/features/databases/ui/show/ShowMongoDbSpecificDataComponent.tsx
```

- [ ] **Step 2: Remove the three enum members**

In `frontend/src/entity/databases/model/DatabaseType.ts`, delete the `MYSQL`, `MARIADB`, `MONGODB` members. Result:

```ts
export enum DatabaseType {
  POSTGRES = 'POSTGRES',
  REDIS = 'REDIS',
  RABBITMQ = 'RABBITMQ',
  KUBERNETES = 'KUBERNETES',
}
```

- [ ] **Step 3: Strip the three branches from the entity model**

In `frontend/src/entity/databases/model/Database.ts`: remove the `mysql`/`mariadb`/`mongodb` optional fields and their type imports.

In `frontend/src/entity/databases/model/getDatabaseLogoFromType.ts`: remove the three `case`/mapping entries and their logo imports.

In `frontend/src/entity/databases/index.ts`: remove the re-exports of the three deleted model folders/types.

- [ ] **Step 4: Strip the three branches from the create/edit/show UI**

In `frontend/src/features/databases/ui/CreateDatabaseComponent.tsx`: remove the MySQL/MariaDB/MongoDB dropdown `Select.Option`s and the three branches in the type-specific form switch.

In `EditDatabaseBaseInfoComponent.tsx`, `EditDatabaseSpecificDataComponent.tsx`, `CreateReadOnlyComponent.tsx`, `ShowDatabaseBaseInfoComponent.tsx`, `ShowDatabaseSpecificDataComponent.tsx`: remove the three `case`/conditional branches (which referenced the now-deleted `Edit*`/`Show*SpecificDataComponent`s) and their imports.

- [ ] **Step 5: Strip the three branches from billing, restores, and backups**

In `frontend/src/features/billing/models/purchaseUtils.ts`: remove the `MySQL / MariaDB` and `MongoDB` entries from `DB_SIZE_COMMANDS`, keeping the PostgreSQL entry.

In `frontend/src/entity/restores/api/restoreApi.ts`, `frontend/src/features/backups/ui/BackupsComponent.tsx`, `frontend/src/features/restores/ui/RestoresComponent.tsx`: remove the three-type references (labels, type guards, switch branches).

- [ ] **Step 6: Build, lint, format**

Run:

```bash
cd frontend && pnpm build && pnpm lint && pnpm format
```

Expected: `pnpm build` succeeds (no TS errors about missing enum members or deleted imports); lint/format clean. Fix any flagged references until green.

- [ ] **Step 7: Grep sweep for frontend code**

Run:

```bash
grep -rniE 'mysql|mariadb|mongo' frontend/src
```

Expected: **zero matches**.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "FEATURE (databases): remove MySQL, MariaDB, MongoDB from the frontend"
```

---

### Task 4: Remove client binaries, test infra, CI, and docs

**Files:**
- Delete:
  - `assets/tools/x64/mysql/`, `assets/tools/x64/mariadb/`, `assets/tools/x64/mongodb/`
  - `assets/tools/win-x64/mysql/`, `assets/tools/win-x64/mariadb/`, `assets/tools/win-x64/mongodb/`
  - `assets/tools/arm/mysql/`, `assets/tools/arm/mariadb/`, `assets/tools/arm/mongodb/`
- Modify:
  - `Dockerfile`
  - `docker-compose.yml`
  - `.github/workflows/ci-release.yml`
  - `README.md`
  - any `.env` / `.env.example` referencing `TEST_MYSQL*` / `TEST_MARIADB*` / `TEST_MONGODB*`

**Interfaces:**
- Consumes: the backend (Task 1) no longer invokes `mysqldump` / `mariadb-dump` / `mongodump`, so the binaries and test DBs are dead.
- Produces: nothing downstream.

- [ ] **Step 1: Delete the bundled client binaries**

```bash
git rm -r \
  assets/tools/x64/mysql assets/tools/x64/mariadb assets/tools/x64/mongodb \
  assets/tools/win-x64/mysql assets/tools/win-x64/mariadb assets/tools/win-x64/mongodb \
  assets/tools/arm/mysql assets/tools/arm/mariadb assets/tools/arm/mongodb
```

- [ ] **Step 2: Clean the Dockerfile**

In `Dockerfile`: remove the three `COPY` lines bringing in `/app/assets/tools/*/mysql/*/bin/*`, `.../mariadb/*/bin/*`, and `.../mongodb/bin/*` (around lines 175-177), remove `libmariadb3` from the apt install (line ~149), and update the comment on line ~162 so it lists only the kept clients (PostgreSQL).

- [ ] **Step 3: Remove the test DB containers from docker-compose**

In `docker-compose.yml`: delete every `test-mysql-*`, `test-mariadb-*`, and `test-mongodb-*` service (including the `-ssl` variants), and any `${TEST_MYSQL_*}` / `${TEST_MARIADB_*}` / `${TEST_MONGODB_*}` env references and their `*data/` volume mounts.

- [ ] **Step 4: Remove the CI steps**

In `.github/workflows/ci-release.yml`: remove the "Wait for MySQL/MariaDB/MongoDB containers" blocks and any matrix entries / test invocations specific to these three types. Keep the PostgreSQL/Redis/RabbitMQ/Kubernetes steps.

- [ ] **Step 5: Clean env files**

```bash
grep -rniE 'TEST_MYSQL|TEST_MARIADB|TEST_MONGODB' . --include='*.env' --include='.env*' --include='*.yml' --include='*.yaml' | grep -v node_modules
```

Remove each matched line from any `.env` / `.env.example` (the `docker-compose.yml` ones were handled in Step 3).

- [ ] **Step 6: Update the README**

In `README.md`: remove the MySQL/MariaDB/MongoDB mentions in the title/subtitle (line ~4), the three badges (lines ~9-11), and the supported-versions list entries (lines ~43-45). Re-read the surrounding copy so the remaining sentence about supported databases reads naturally with only PostgreSQL + Redis/RabbitMQ/Kubernetes.

- [ ] **Step 7: Final repo-wide grep sweep**

Run:

```bash
grep -rniE 'mysql|mariadb|mongo' . \
  --exclude-dir=node_modules --exclude-dir=.git \
  | grep -vE 'backend/migrations/|docs/superpowers/'
```

Expected: **zero matches** outside the intentionally-kept historical migrations and design/plan docs.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "CHORE (databases): drop mysql/mariadb/mongodb client binaries, test infra, CI, docs"
```

---

## Self-Review

**Spec coverage:**
- Enum spine → Task 1 Step 2, Task 3 Step 2. ✓
- Per-type package deletion (backend) → Task 1 Step 1. ✓
- Backend hub edits → Task 1 Steps 3-7. ✓
- Cleanup migration → Task 2. ✓
- Per-type frontend folder deletion + hub edits → Task 3. ✓
- Dockerfile / assets binaries / docker-compose / CI / README → Task 4. ✓
- Verification (build, lint, real frontend build, migration apply, grep sweeps) → embedded in each task + Task 4 Step 7 final sweep. ✓
- Risk: shared-file edits keep PostgreSQL → called out in each "remove only the three branches" step. ✓
- Risk: migration cascade verification → Task 2 Step 3. ✓

**Placeholder scan:** No "TBD"/"handle edge cases" left. The migration timestamp is generated by `make migration-create` (Task 2 Step 1), not hand-written — intentional, not a placeholder.

**Type consistency:** The four kept enum values are spelled identically in Task 1 Step 2 (Go) and Task 3 Step 2 (TS): `POSTGRES`, `REDIS`, `RABBITMQ`, `KUBERNETES`. The deleted string literals `'MYSQL'`/`'MARIADB'`/`'MONGODB'` in Task 2's migration match the removed Go consts.

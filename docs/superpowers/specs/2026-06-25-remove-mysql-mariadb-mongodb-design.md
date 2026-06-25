# Remove MySQL, MariaDB, and MongoDB backup support

**Date:** 2026-06-25
**Status:** Approved (design)
**Branch base:** `feature/kubernetes-backup-support`

## Objective

Remove MySQL, MariaDB, and MongoDB as supported database types from Databasus,
leaving the remaining four — **PostgreSQL, Redis, RabbitMQ, Kubernetes** — fully
working. This is a permanent, full removal: code, schema, bundled client
binaries, test infrastructure, and docs.

## Decisions (settled with the user)

- **Full hard removal**, not a UI-only hide. Matches the repo rule "delete the
  old code outright — no compat shims, no fallbacks."
- **Clean slate / delete freely.** Only test deployments exist; there is no
  production data of these types to preserve. The cleanup migration may delete
  rows and drop tables without guards.
- **Forward "drop" migration**, not a history rewrite. All historical goose
  migrations stay untouched (rewriting them would require hand-auditing every
  file to separate type-specific changes from ones that also touch shared tables
  such as `move_cpu_count_to_databases` — error-prone for no benefit).

## Approaches considered

- **A — Full hard removal (chosen).** Delete all three types end to end.
- **B — Soft hide (UI dropdown only).** Rejected: leaves thousands of lines of
  dead backend code, three dead tables, and ~100 MB of bundled client binaries;
  contradicts house rules.
- **C — Feature flag.** Rejected: overkill for a permanent removal.

## Architecture context (why this is safe and mechanical)

- `databases.type` is a plain `TEXT` column with **no CHECK constraint**, so
  removing types needs no change to the `databases` table schema itself.
- The enum is two declarations — `backend/.../databases/enums.go`
  (`DatabaseType` consts) and `frontend/.../model/DatabaseType.ts`. Everything
  else dispatches off these.
- Dispatch is a consistent `switch` / `Preload` / struct-field-per-type pattern
  in ~20 backend hub files and ~12 frontend files. Each type also has
  self-contained per-type packages/folders that are deleted wholesale.
- Each per-type table (`mysql_databases`, `mariadb_databases`,
  `mongodb_databases`) is referenced by `database_id ... ON DELETE CASCADE`.

## Plan of work

### 1. Enum spine

Remove `MYSQL`, `MARIADB`, `MONGODB` from:

- `backend/internal/features/databases/enums.go`
- `frontend/src/entity/databases/model/DatabaseType.ts`

### 2. Delete self-contained per-type packages (whole folders/files)

**Backend:**

- `internal/features/databases/databases/{mysql,mariadb,mongodb}/`
- `internal/features/backups/backups/usecases/{mysql,mariadb,mongodb}/`
- `internal/features/restores/usecases/{mysql,mariadb,mongodb}/`
- `internal/util/tools/{mysql,mariadb,mongodb}.go`
- `internal/features/tests/{mysql,mariadb,mongodb}_backup_restore_test.go`

**Frontend:**

- `src/entity/databases/model/{mysql,mariadb,mongodb}/`
- `src/features/databases/ui/edit/Edit{MySql,MariaDb,MongoDb}SpecificDataComponent.tsx`
- `src/features/databases/ui/show/Show{MySql,MariaDb,MongoDb}SpecificDataComponent.tsx`

### 3. Edit hub files — remove the three cases/fields/imports, keep the four

**Backend:**

- `databases/model.go` — three struct fields + cases in `Validate`,
  `IsUserReadOnly`, `EncryptSensitiveFields`, `PopulateDbData`, `Update`,
  `getSpecificDatabase`; drop the three imports.
- `databases/repository.go` — `Save` (two switches), `Delete` switch, the three
  `Preload` calls across `FindByID` / `FindByWorkspaceID` / `GetAllDatabases`,
  and the `Omit(...)` lists; drop the three imports.
- `databases/service.go`, `databases/testing.go`, `databases/controller_test.go`
- `backups/backups/usecases/create_backup_uc.go` (struct fields + switch cases +
  imports) and `backups/backups/usecases/di.go` (wiring)
- `restores/*` dispatch: `restoring/restorer.go`, `restoring/scheduler.go`,
  `restoring/dto.go`, `service.go`, `usecases/di.go`,
  `usecases/restore_backup_uc.go`, `core/dto.go`, `core/model.go`,
  `core/repository.go`, `controller_test.go`
- `healthcheck/attempt/check_database_health_uc.go`
- `telemetry/service.go` (`buildDatabaseEntry`) + `telemetry/service_test.go`
- `util/tools/common.go` — drop `checkMysql()` / `checkMariadb()` /
  `checkMongodb()` calls and the doc comment listing them
- `backups/backups/streaming/stream.go`,
  `backups/backups/backuping/scheduler_test.go`,
  `tests/backup_hang_edge_cases_test.go`, `tests/ssl_backup_restore_test.go`
  (keep the PostgreSQL SSL path; remove only the three types)
- `config/config.go` — remove the ~24 `Test*Port` fields for these types
- `agent/verification/internal/features/runner/runner_test.go`

**Frontend:**

- `entity/databases/model/Database.ts`, `getDatabaseLogoFromType.ts`,
  `entity/databases/index.ts`
- `features/databases/ui/CreateDatabaseComponent.tsx` — dropdown options + form
  switch
- `features/databases/ui/edit/EditDatabaseBaseInfoComponent.tsx`,
  `EditDatabaseSpecificDataComponent.tsx`, `CreateReadOnlyComponent.tsx`
- `features/databases/ui/show/ShowDatabaseBaseInfoComponent.tsx`,
  `ShowDatabaseSpecificDataComponent.tsx`
- `features/billing/models/purchaseUtils.ts` — drop the `MySQL / MariaDB` and
  `MongoDB` entries from `DB_SIZE_COMMANDS`
- `entity/restores/api/restoreApi.ts`, `features/backups/ui/BackupsComponent.tsx`,
  `features/restores/ui/RestoresComponent.tsx`

### 4. Cleanup migration (single new goose file)

`backend/migrations/2026XXXX_drop_mysql_mariadb_mongodb_databases.sql`

- **Up:**
  - `DELETE FROM databases WHERE type IN ('MYSQL', 'MARIADB', 'MONGODB');`
    (relies on `ON DELETE CASCADE` to remove per-type rows, backups,
    healthcheck configs/attempts, restores, etc.)
  - `DROP TABLE IF EXISTS mysql_databases;`
  - `DROP TABLE IF EXISTS mariadb_databases;`
  - `DROP TABLE IF EXISTS mongodb_databases;`
- **Down:** documented no-op — the dropped data is unrecoverable; follows the
  existing no-op-Down precedent (`20260512000000_disable_healthcheck_for_agent_dbs.sql`).
- During implementation, verify each child table's `database_id` FK is
  `ON DELETE CASCADE`. If any child lacks cascade, delete from it explicitly
  before the `databases` delete.

### 5. Infra / build / docs

- `Dockerfile` — drop the three client-binary `COPY` lines
  (`mysql`/`mariadb`/`mongodb` under `/app/assets/tools/...`) and the
  `libmariadb3` apt package.
- `assets/tools/{x64,win-x64,arm}/{mysql,mariadb,mongodb}/` — delete the bundled
  `mysqldump` / `mariadb-dump` / `mongodump` binaries for all three arches.
- `docker-compose.yml` — remove the ~25 test containers (mysql 5.7/8.0/8.4/9.5,
  mariadb 5.5–12.0, mongodb 4.0–8.2, plus the SSL variants).
- `.github/workflows/ci-release.yml` — remove the corresponding wait-for and
  test steps.
- `README.md` — title, badges, and the supported-versions list (keep
  PostgreSQL; Redis/RabbitMQ/Kubernetes as already documented).
- Leave the two historical specs in `docs/superpowers/specs/` as-is (historical
  record).

## Verification

- **Backend:** `make lint` + `go build ./...` + `go test` for the touched
  features (databases, backups, restores, telemetry, healthcheck) — all green.
- **Frontend:** `pnpm build` (real build, **not** `tsc --noEmit`) + `pnpm lint`
  + `pnpm format`.
- **Migration:** apply against a fresh DB (`goose up`); confirm the three tables
  are gone and the kept four types create/list/back-up unaffected.
- **Grep sweep:** zero case-insensitive `mysql | mariadb | mongo` matches remain
  in `backend/`, `frontend/src/`, and `agent/` (README and historical specs
  excepted only where intentionally kept — here README is also cleaned).

## Risks and edge cases

- **Missed reference → compile error.** Caught by the full build + lint + grep
  sweep.
- **Shared-file edits** (telemetry, SSL test, billing `DB_SIZE_COMMANDS`,
  `config.go`): touch only the three cases; keep PostgreSQL and the other kept
  types intact.
- **Migration cascade:** the single `DELETE` is sufficient only if every child
  FK is `ON DELETE CASCADE`; verify during implementation.
- **Edit ordering:** removing the enum consts first will cascade compile breaks;
  delete per-type packages and apply the hub edits per layer, then build, to
  keep error noise manageable.

## Out of scope

- No backward-compatibility shims, deprecation aliases, or data export.
- No changes to the remaining four database types beyond removing the three
  cases alongside them.

# Redis & RabbitMQ Backup Support — Design Spec

**Date:** 2026-06-03
**Author:** Copilot (brainstorming session)
**Status:** Approved

---

## Problem

Databasus supports PostgreSQL, MySQL, MariaDB, and MongoDB. Users also run Redis and RabbitMQ and want scheduled, encrypted backups of those services pushed to S3 (or any configured storage). They do not need automated restore — delete and download are sufficient.

---

## Approach

Both Redis and RabbitMQ follow the **MongoDB streaming pattern**: connect remotely → stream a dump directly to storage with the existing encryption/counting/io.Pipe scaffolding. No CLI binaries are bundled; both backup engines are implemented in native Go. The restore subsystem is untouched.

---

## Architecture

### 1. Database type enum

**Backend** (`backend/internal/features/databases/enums.go`):

```go
DatabaseTypeRedis    DatabaseType = "REDIS"
DatabaseTypeRabbitmq DatabaseType = "RABBITMQ"
```

**Frontend** (`frontend/src/entity/databases/model/DatabaseType.ts`):

```ts
REDIS    = 'REDIS',
RABBITMQ = 'RABBITMQ',
```

---

### 2. Data models

#### Redis — `redis_databases` table

| Column          | Type                      | Notes                      |
| --------------- | ------------------------- | -------------------------- |
| `id`          | uuid PK                   |                            |
| `database_id` | uuid FK → databases      |                            |
| `host`        | text NOT NULL             |                            |
| `port`        | int NOT NULL default 6379 |                            |
| `username`    | text (nullable)           | ACL, Redis 6+              |
| `password`    | text NOT NULL             | AES-encrypted at rest      |
| `is_tls`      | boolean default false     |                            |
| `version`     | text                      | detected at TestConnection |

GORM model: `backend/internal/features/databases/databases/redis/model.go`

#### RabbitMQ — `rabbitmq_databases` table

| Column              | Type                       | Notes                      |
| ------------------- | -------------------------- | -------------------------- |
| `id`              | uuid PK                    |                            |
| `database_id`     | uuid FK → databases       |                            |
| `host`            | text NOT NULL              |                            |
| `management_port` | int NOT NULL default 15672 | Management HTTP API        |
| `username`        | text NOT NULL              |                            |
| `password`        | text NOT NULL              | AES-encrypted at rest      |
| `is_https`        | boolean default false      |                            |
| `version`         | text                       | detected at TestConnection |

GORM model: `backend/internal/features/databases/databases/rabbitmq/model.go`

Both models implement the `DatabaseConnector` interface:

- `Validate()` — required fields, port range
- `TestConnection()` — dial + auth + version detection
- `GetRawDbSizeMb()` — Redis: `INFO memory` → `used_memory_human`; RabbitMQ: returns `0` (definitions are config, not dataset — keeps billing semantics honest)
- `HideSensitiveData()` — blank password
- `EncryptSensitiveFields()` / decrypt helper
- `PopulateDbData()` / `PopulateVersion()` — detect and store version string
- `Update()` — standard field merge, skip password if empty

**`Database` struct** (`databases/model.go`) gets:

```go
Redis    *redis.RedisDatabase       `json:"redis,omitzero"    gorm:"foreignKey:DatabaseID"`
Rabbitmq *rabbitmq.RabbitmqDatabase `json:"rabbitmq,omitzero" gorm:"foreignKey:DatabaseID"`
```

All switch statements in `model.go` (`Validate`, `Update`, `getSpecificDatabase`, etc.) get new cases. `IsUserReadOnly` returns "not supported" for both. No `CreateReadOnlyUser`.

---

### 3. Migrations

Two SQL migration files (timestamped, next in sequence):

```sql
-- create_redis_databases_table.sql
CREATE TABLE redis_databases (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  database_id   UUID REFERENCES databases(id) ON DELETE CASCADE,
  host          TEXT NOT NULL,
  port          INT  NOT NULL DEFAULT 6379,
  username      TEXT,
  password      TEXT NOT NULL,
  is_tls        BOOLEAN NOT NULL DEFAULT FALSE,
  version       TEXT NOT NULL DEFAULT ''
);
```

```sql
-- create_rabbitmq_databases_table.sql
CREATE TABLE rabbitmq_databases (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  database_id      UUID REFERENCES databases(id) ON DELETE CASCADE,
  host             TEXT NOT NULL,
  management_port  INT  NOT NULL DEFAULT 15672,
  username         TEXT NOT NULL,
  password         TEXT NOT NULL,
  is_https         BOOLEAN NOT NULL DEFAULT FALSE,
  version          TEXT NOT NULL DEFAULT ''
);
```

---

### 4. Backup engines (native Go, no bundled binaries)

#### Redis (`backend/internal/features/backups/backups/usecases/redis/create_backup_uc.go`)

1. Resolve TLS dialer if `is_tls`; TCP dial `host:port`.
2. Send `AUTH [username] password` (or `AUTH password` for password-only; skip if no password).
3. Send `SYNC` command.
4. Read the inline response: `+CONTINUE\r\n` (for a replica-capable server) is treated as a pass-through; wait for `$<len>\r\n` bulk string header.
5. Stream exactly `<len>` bytes into the existing `io.Pipe` → encryption writer → counting writer → `storage.SaveFile` pipeline. This is identical to the MongoDB streaming path.
6. Track progress via `backupProgressListener`.
7. Filename: `<backup-id>.rdb` (or `.rdb.enc` when encrypted — same as current convention).

> Note: `SYNC` produces a full RDB snapshot universally supported across Redis 2.8+. It is simpler and more reliable than `PSYNC` for a one-shot backup.

#### RabbitMQ (`backend/internal/features/backups/backups/usecases/rabbitmq/create_backup_uc.go`)

1. Build base URL: `http(s)://host:management_port`.
2. `GET /api/definitions` with HTTP Basic Auth (username + decrypted password). If `is_https`, use a TLS-aware `http.Client` (accept server cert; if self-signed, add `InsecureSkipVerify` option behind a flag — document this risk).
3. Stream the JSON response body through the same pipe/encryption/counting/storage pipeline.
4. Filename: `<backup-id>.definitions.json` (or `.definitions.json.enc`).

Both use the same `backupTimeout` (23 h), `shutdownCheckInterval`, `copyBufferSize` constants from MongoDB.

**Registration:**

`backend/internal/features/backups/backups/usecases/di.go`:

```go
CreateRedisBackupUsecase    *usecases_redis.CreateRedisBackupUsecase
CreateRabbitmqBackupUsecase *usecases_rabbitmq.CreateRabbitmqBackupUsecase
```

`create_backup_uc.go` switch gets `DatabaseTypeRedis` and `DatabaseTypeRabbitmq` cases.

---

### 5. Restore subsystem — unchanged

`backend/internal/features/restores/usecases/di.go` and the `RestoreBackupUsecase` switch are **not modified**. Requests to restore a Redis/RabbitMQ backup return `"database type not supported"` — this path is unreachable from the UI.

---

### 6. Frontend

#### Icons

- `frontend/public/icons/databases/redis.svg`
- `frontend/public/icons/databases/rabbitmq.svg`
- `getDatabaseLogoFromType.ts` — add cases for `REDIS`, `RABBITMQ`

#### Entity models

```
frontend/src/entity/databases/model/redis/
  RedisDatabase.ts          (TS interface matching backend JSON)
  RedisVersion.ts           (version string, no enum needed)
frontend/src/entity/databases/model/rabbitmq/
  RabbitmqDatabase.ts
  RabbitmqVersion.ts
```

Both wired into `Database.ts` (`redis?: RedisDatabase; rabbitmq?: RabbitmqDatabase`) and exported from `entity/databases/index.ts`.

#### Create / Edit forms

- `CreateDatabaseComponent.tsx` — add `REDIS` and `RABBITMQ` to the type dropdown.
- `EditRedisSpecificDataComponent.tsx` — host, port, username (optional), password, TLS toggle.
- `EditRabbitmqSpecificDataComponent.tsx` — host, management port, username, password, HTTPS toggle.
- `EditDatabaseSpecificDataComponent.tsx` — add cases to render the above.

#### Show (read-only view)

- `ShowRedisSpecificDataComponent.tsx`
- `ShowRabbitmqSpecificDataComponent.tsx`
- `ShowDatabaseSpecificDataComponent.tsx` — add cases.

#### Backups list (`BackupsComponent.tsx`)

- **Restore button** (line ~468): hide when `database.type === DatabaseType.REDIS || database.type === DatabaseType.RABBITMQ`.
- **Download tooltip**: add Redis (`'Download backup file. It can be restored manually via redis-cli --rdb <file> ...'`) and RabbitMQ (`'Download backup file. It is a RabbitMQ definitions JSON — import via Management UI or rabbitmqadmin'`).
- Verify button is already Postgres-only — no change needed.

---

### 7. Testing

Following the existing `controller_test.go` pattern (preferred over unit tests):

- `backend/internal/features/databases/controller_test.go` — create/validate Redis and RabbitMQ database entries.
- `backend/internal/features/backups/backups/controllers/controller_test.go` — trigger backup dispatch for both types, assert correct use case is invoked.

No E2E tests (those require live instances; existing E2E tests are Postgres-only).

---

## Decisions

| Decision             | Choice                                       | Rationale                                                                      |
| -------------------- | -------------------------------------------- | ------------------------------------------------------------------------------ |
| Redis acquisition    | Native Go `SYNC`                           | No binary bundle; universally supported; mirrors MongoDB streaming             |
| RabbitMQ acquisition | Management HTTP API `GET /api/definitions` | No CLI needed; standard approach                                               |
| RabbitMQ size        | Returns 0 MB                                 | Definitions are config (~KB), not dataset size; keeps billing semantics honest |
| Vhost filter         | None — always all vhosts                    | User confirmed: export all vhosts                                              |
| Restore support      | Omitted for both types                       | User requirement: delete + download only                                       |
| Verify support       | Omitted for both types                       | Only meaningful for Postgres WAL; no temp-instance restore logic               |

---

## Out of scope

- Redis cluster / Sentinel mode (standard single-instance only for now)
- RabbitMQ per-vhost export (all vhosts via `/api/definitions`)
- RabbitMQ message data / queue contents (definitions only)
- Automated restore for either type

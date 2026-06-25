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

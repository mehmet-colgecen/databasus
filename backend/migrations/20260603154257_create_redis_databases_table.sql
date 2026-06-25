-- +goose Up
-- +goose StatementBegin
CREATE TABLE redis_databases (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id UUID REFERENCES databases (id) ON DELETE CASCADE,
    version     TEXT NOT NULL DEFAULT '',
    host        TEXT NOT NULL,
    port        INT NOT NULL DEFAULT 6379,
    username    TEXT NOT NULL DEFAULT '',
    password    TEXT NOT NULL,
    is_tls      BOOLEAN NOT NULL DEFAULT FALSE
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_redis_databases_database_id ON redis_databases (database_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_redis_databases_database_id;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS redis_databases;
-- +goose StatementEnd

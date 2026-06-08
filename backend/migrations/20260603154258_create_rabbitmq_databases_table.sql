-- +goose Up
-- +goose StatementBegin
CREATE TABLE rabbitmq_databases (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id     UUID REFERENCES databases (id) ON DELETE CASCADE,
    version         TEXT NOT NULL DEFAULT '',
    host            TEXT NOT NULL,
    management_port INT NOT NULL DEFAULT 15672,
    username        TEXT NOT NULL,
    password        TEXT NOT NULL,
    is_https        BOOLEAN NOT NULL DEFAULT FALSE
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_rabbitmq_databases_database_id ON rabbitmq_databases (database_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_rabbitmq_databases_database_id;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS rabbitmq_databases;
-- +goose StatementEnd

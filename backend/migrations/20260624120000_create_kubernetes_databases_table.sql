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

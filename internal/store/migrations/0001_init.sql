-- +goose Up
-- The meta table holds small key/value bookkeeping for the acm database
-- itself (origin marker, format notes). The LCM domain schema (conversations,
-- messages, summary DAG, ...) is introduced in a later migration so this first
-- step stays trivially reversible and the goose version table is established.
CREATE TABLE meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
) STRICT;

INSERT INTO meta (key, value) VALUES ('schema_origin', 'agent-context-manager');

-- +goose Down
DROP TABLE meta;

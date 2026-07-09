-- +goose Up

CREATE TABLE conversation_pins (
    conversation_id TEXT PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
    pinned_at       TEXT NOT NULL
) STRICT;

CREATE TABLE summary_expansions (
    summary_id  TEXT PRIMARY KEY REFERENCES summaries(id) ON DELETE CASCADE,
    expanded_at TEXT NOT NULL
) STRICT;

-- +goose Down
DROP TABLE summary_expansions;
DROP TABLE conversation_pins;

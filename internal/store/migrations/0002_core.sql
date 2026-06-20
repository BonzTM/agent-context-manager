-- +goose Up

-- conversations: one row per agent session. Keyed by (agent, session_id); the
-- id is derived from that pair so ingestion is idempotent.
CREATE TABLE conversations (
    id          TEXT PRIMARY KEY,
    agent       TEXT NOT NULL,
    session_id  TEXT NOT NULL,
    session_key TEXT NOT NULL DEFAULT '',
    title       TEXT NOT NULL DEFAULT '',
    archived    INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    UNIQUE (agent, session_id)
) STRICT;

-- messages: the lossless, append-only verbatim store. content is canonical;
-- raw holds the original event JSON when available. Dedupe is enforced per
-- conversation on identity_hash so re-ingested transcript lines collapse.
CREATE TABLE messages (
    id              TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    seq             INTEGER NOT NULL,
    role            TEXT NOT NULL,
    content         TEXT NOT NULL,
    token_count     INTEGER NOT NULL DEFAULT 0,
    tool_name       TEXT NOT NULL DEFAULT '',
    external_id     TEXT NOT NULL DEFAULT '',
    identity_hash   TEXT NOT NULL,
    raw             TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL,
    UNIQUE (conversation_id, seq),
    UNIQUE (conversation_id, identity_hash)
) STRICT;

CREATE INDEX idx_messages_conv_seq ON messages (conversation_id, seq);

-- Full-text index over message content for lcm_grep-style search. message_id
-- links back to messages; the other columns are stored UNINDEXED so they can be
-- filtered/returned without a join when convenient.
CREATE VIRTUAL TABLE messages_fts USING fts5 (
    content,
    message_id UNINDEXED,
    conversation_id UNINDEXED,
    role UNINDEXED
);

-- +goose Down
DROP TABLE messages_fts;
DROP INDEX idx_messages_conv_seq;
DROP TABLE messages;
DROP TABLE conversations;

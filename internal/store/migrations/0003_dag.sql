-- +goose Up

-- summaries: nodes of the summary DAG. Leaf nodes (depth 0) cover raw messages;
-- condensed nodes (depth >= 1) cover lower summaries. The id is content-derived
-- so re-summarizing the same span is idempotent.
CREATE TABLE summaries (
    id                       TEXT PRIMARY KEY,
    conversation_id          TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    kind                     TEXT NOT NULL,
    depth                    INTEGER NOT NULL,
    content                  TEXT NOT NULL,
    token_count              INTEGER NOT NULL DEFAULT 0,
    source_count             INTEGER NOT NULL DEFAULT 0,
    descendant_message_count INTEGER NOT NULL DEFAULT 0,
    earliest_seq             INTEGER NOT NULL DEFAULT 0,
    latest_seq               INTEGER NOT NULL DEFAULT 0,
    created_at               TEXT NOT NULL
) STRICT;

CREATE INDEX idx_summaries_conv_depth ON summaries (conversation_id, depth);

-- summary_messages: lossless pointers from a leaf summary to its source messages.
CREATE TABLE summary_messages (
    summary_id TEXT NOT NULL REFERENCES summaries(id) ON DELETE CASCADE,
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    PRIMARY KEY (summary_id, message_id)
) STRICT;

-- summary_parents: DAG edges from a condensed parent to its child summaries.
CREATE TABLE summary_parents (
    parent_id TEXT NOT NULL REFERENCES summaries(id) ON DELETE CASCADE,
    child_id  TEXT NOT NULL REFERENCES summaries(id) ON DELETE CASCADE,
    PRIMARY KEY (parent_id, child_id)
) STRICT;

-- context_items: the exact ordered active window the model would see, a mix of
-- raw message pointers and summary pointers.
CREATE TABLE context_items (
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    ordinal         INTEGER NOT NULL,
    item_type       TEXT NOT NULL,
    ref_id          TEXT NOT NULL,
    PRIMARY KEY (conversation_id, ordinal)
) STRICT;

-- large_files: oversized payloads offloaded to disk, with only an exploration
-- summary kept inline.
CREATE TABLE large_files (
    id                  TEXT PRIMARY KEY,
    conversation_id     TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    message_id          TEXT NOT NULL DEFAULT '',
    storage_uri         TEXT NOT NULL,
    byte_size           INTEGER NOT NULL DEFAULT 0,
    token_count         INTEGER NOT NULL DEFAULT 0,
    exploration_summary TEXT NOT NULL DEFAULT '',
    created_at          TEXT NOT NULL
) STRICT;

-- Full-text index over summary content so lcm_grep can search summaries too.
CREATE VIRTUAL TABLE summaries_fts USING fts5 (
    content,
    summary_id UNINDEXED,
    conversation_id UNINDEXED,
    depth UNINDEXED
);

-- +goose Down
DROP TABLE summaries_fts;
DROP TABLE large_files;
DROP TABLE context_items;
DROP TABLE summary_parents;
DROP TABLE summary_messages;
DROP INDEX idx_summaries_conv_depth;
DROP TABLE summaries;

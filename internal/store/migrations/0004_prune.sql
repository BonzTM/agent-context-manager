-- +goose Up

-- Drop the pre-release surface that never gained a consumer: session_key and
-- archived were stored but nothing read or set them, and idx_messages_conv_seq
-- duplicated the UNIQUE (conversation_id, seq) constraint's implicit index.
ALTER TABLE conversations DROP COLUMN session_key;
ALTER TABLE conversations DROP COLUMN archived;
DROP INDEX idx_messages_conv_seq;

-- +goose Down
CREATE INDEX idx_messages_conv_seq ON messages (conversation_id, seq);
ALTER TABLE conversations ADD COLUMN archived INTEGER NOT NULL DEFAULT 0;
ALTER TABLE conversations ADD COLUMN session_key TEXT NOT NULL DEFAULT '';

-- +goose Up

-- Record which exploration-summary extractor produced a large file's summary
-- (json/csv/sql/code for the deterministic type-aware extractors, or the
-- summarizer/truncation fallbacks), so drill-down shows how the description
-- was derived.
ALTER TABLE large_files ADD COLUMN extractor TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE large_files DROP COLUMN extractor;

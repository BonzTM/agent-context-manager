CREATE INDEX IF NOT EXISTS idx_acm_runs_project_receipt_created
	ON acm_runs (project_id, receipt_id, created_at DESC, run_id DESC);

CREATE TABLE IF NOT EXISTS impact_analysis_snapshots (
    event_id TEXT PRIMARY KEY REFERENCES change_events(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    baseline_metrics JSONB NOT NULL,
    impact_metrics JSONB NOT NULL,
    deltas JSONB NOT NULL,
    impact_score DOUBLE PRECISION NOT NULL,
    impact_level TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_snapshots_event_id ON impact_analysis_snapshots(event_id);

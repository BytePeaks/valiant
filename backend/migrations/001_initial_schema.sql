CREATE TABLE IF NOT EXISTS change_events (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    change_type TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    affected_services TEXT[],
    metadata JSONB,
    summary TEXT
);

CREATE INDEX IF NOT EXISTS idx_change_events_timestamp ON change_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_change_events_affected_services ON change_events USING GIN(affected_services);

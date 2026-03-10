-- Migration 007: Add event_links table and blast_radius column
-- Supports persistent linking between intent events (CI builds, config changes)
-- and execution events (deployment/statefulset rollouts)

CREATE TABLE IF NOT EXISTS event_links (
    id TEXT PRIMARY KEY,
    intent_event_id TEXT NOT NULL REFERENCES change_events(id) ON DELETE CASCADE,
    execution_event_id TEXT NOT NULL REFERENCES change_events(id) ON DELETE CASCADE,
    link_type TEXT NOT NULL,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB DEFAULT '{}',
    UNIQUE(intent_event_id, execution_event_id, link_type)
);

CREATE INDEX IF NOT EXISTS idx_event_links_intent ON event_links(intent_event_id);
CREATE INDEX IF NOT EXISTS idx_event_links_execution ON event_links(execution_event_id);

-- Add blast_radius column to change_events for storing affected workload scope
ALTER TABLE change_events ADD COLUMN IF NOT EXISTS blast_radius JSONB;

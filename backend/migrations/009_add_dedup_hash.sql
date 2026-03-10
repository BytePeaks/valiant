ALTER TABLE change_events ADD COLUMN IF NOT EXISTS dedup_hash TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_change_events_dedup_hash ON change_events (dedup_hash) WHERE dedup_hash IS NOT NULL;

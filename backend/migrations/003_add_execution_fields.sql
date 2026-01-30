ALTER TABLE change_events
ADD COLUMN IF NOT EXISTS trigger_type TEXT,
ADD COLUMN IF NOT EXISTS execution_id TEXT,
ADD COLUMN IF NOT EXISTS end_time TIMESTAMPTZ;

-- Backfill existing data to adhere to new model (best effort)
UPDATE change_events SET trigger_type = 'manual', execution_id = id WHERE trigger_type IS NULL;
UPDATE change_events SET execution_id = id WHERE execution_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_change_events_execution_id ON change_events(execution_id);

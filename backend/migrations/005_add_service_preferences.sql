CREATE TABLE IF NOT EXISTS service_preferences (
    service_name VARCHAR(255) PRIMARY KEY,
    visible_metrics TEXT[] DEFAULT '{}'
);

ALTER TABLE impact_analysis_snapshots
DROP COLUMN IF EXISTS visible_metrics;
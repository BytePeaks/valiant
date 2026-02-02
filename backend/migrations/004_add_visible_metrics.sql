ALTER TABLE impact_analysis_snapshots
ADD COLUMN IF NOT EXISTS visible_metrics TEXT[] DEFAULT '{}';
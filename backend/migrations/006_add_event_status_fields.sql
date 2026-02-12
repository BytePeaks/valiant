ALTER TABLE change_events
ADD COLUMN status VARCHAR(20) DEFAULT 'ready',
ADD COLUMN invalid_reason VARCHAR(50),
ADD COLUMN skew_seconds INT;

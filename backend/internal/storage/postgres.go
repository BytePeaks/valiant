package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"valiant/internal/domain"

	"github.com/lib/pq"
)

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(db *sql.DB) *PostgresStorage {
	return &PostgresStorage{db: db}
}

func (s *PostgresStorage) SaveChangeEvent(ctx context.Context, event domain.ChangeEvent) error {
	query := `
		INSERT INTO change_events (id, source, change_type, timestamp, affected_services, metadata, summary)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			source = EXCLUDED.source,
			change_type = EXCLUDED.change_type,
			timestamp = EXCLUDED.timestamp,
			affected_services = EXCLUDED.affected_services,
			metadata = EXCLUDED.metadata,
			summary = EXCLUDED.summary
	`

	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = s.db.ExecContext(ctx, query,
		event.ID,
		event.Source,
		event.ChangeType,
		event.Timestamp,
		pq.Array(event.AffectedServices),
		metadataJSON,
		event.Summary,
	)

	if err != nil {
		return fmt.Errorf("failed to save change event: %w", err)
	}

	return nil
}

func (s *PostgresStorage) GetChangeEvents(ctx context.Context, filters map[string]interface{}) ([]domain.ChangeEvent, error) {
	query := `
		SELECT id, source, change_type, timestamp, affected_services, metadata, summary
		FROM change_events
		ORDER BY timestamp DESC
		LIMIT 100
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query change events: %w", err)
	}
	defer rows.Close()

	var events []domain.ChangeEvent
	for rows.Next() {
		var event domain.ChangeEvent
		var metadataJSON []byte
		var affectedServices pq.StringArray

		err := rows.Scan(
			&event.ID,
			&event.Source,
			&event.ChangeType,
			&event.Timestamp,
			&affectedServices,
			&metadataJSON,
			&event.Summary,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan change event: %w", err)
		}

		event.AffectedServices = []string(affectedServices)
		if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		events = append(events, event)
	}

	return events, nil
}

func (s *PostgresStorage) GetChangeEventByID(ctx context.Context, id string) (domain.ChangeEvent, error) {
	query := `
		SELECT id, source, change_type, timestamp, affected_services, metadata, summary
		FROM change_events
		WHERE id = $1
	`

	var event domain.ChangeEvent
	var metadataJSON []byte
	var affectedServices pq.StringArray

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&event.ID,
		&event.Source,
		&event.ChangeType,
		&event.Timestamp,
		&affectedServices,
		&metadataJSON,
		&event.Summary,
	)

	if err == sql.ErrNoRows {
		return domain.ChangeEvent{}, fmt.Errorf("event not found")
	}
	if err != nil {
		return domain.ChangeEvent{}, fmt.Errorf("failed to get change event: %w", err)
	}

	event.AffectedServices = []string(affectedServices)
	if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
		return domain.ChangeEvent{}, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return event, nil
}

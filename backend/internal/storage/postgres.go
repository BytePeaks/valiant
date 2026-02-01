package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"valiant/internal/domain"

	"github.com/lib/pq"
)

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(db *sql.DB) *PostgresStorage {
	return &PostgresStorage{db: db}
}

func (s *PostgresStorage) RunMigration(schemaPath string) error {
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	if _, err := s.db.Exec(string(content)); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	return nil
}

func (s *PostgresStorage) SaveChangeEvent(ctx context.Context, event domain.ChangeEvent) error {
	query := `
		INSERT INTO change_events (id, source, trigger_type, execution_id, change_type, timestamp, end_time, affected_services, metadata, summary)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			source = EXCLUDED.source,
			trigger_type = EXCLUDED.trigger_type,
			execution_id = EXCLUDED.execution_id,
			change_type = EXCLUDED.change_type,
			timestamp = EXCLUDED.timestamp,
			end_time = EXCLUDED.end_time,
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
		event.TriggerType,
		event.ExecutionID,
		event.ChangeType,
		event.Timestamp,
		event.EndTime,
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
		SELECT id, source, trigger_type, execution_id, change_type, timestamp, end_time, affected_services, metadata, summary
		FROM change_events
	`
	var args []interface{}
	var whereClauses []string
	argCount := 1

	if triggerType, ok := filters["trigger_type"]; ok {
		whereClauses = append(whereClauses, fmt.Sprintf("trigger_type = $%d", argCount))
		args = append(args, triggerType)
		argCount++
	}
	if from, ok := filters["from_timestamp"]; ok {
		whereClauses = append(whereClauses, fmt.Sprintf("timestamp >= $%d", argCount))
		args = append(args, from)
		argCount++
	}
	if to, ok := filters["to_timestamp"]; ok {
		whereClauses = append(whereClauses, fmt.Sprintf("timestamp <= $%d", argCount))
		args = append(args, to)
		argCount++
	}
	if services, ok := filters["services_any_of"].([]string); ok && len(services) > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf("affected_services && $%d", argCount))
		args = append(args, pq.Array(services))
		argCount++
	}

	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	query += `
		ORDER BY timestamp DESC
		LIMIT 100
	`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query change events: %w", err)
	}
	defer rows.Close()

	var events []domain.ChangeEvent
	for rows.Next() {
		var event domain.ChangeEvent
		var metadataJSON []byte
		var affectedServices pq.StringArray
		var triggerType, executionID sql.NullString
		var endTime sql.NullTime

		err := rows.Scan(
			&event.ID,
			&event.Source,
			&triggerType,
			&executionID,
			&event.ChangeType,
			&event.Timestamp,
			&endTime,
			&affectedServices,
			&metadataJSON,
			&event.Summary,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan change event: %w", err)
		}

		event.TriggerType = triggerType.String
		event.ExecutionID = executionID.String
		if endTime.Valid {
			t := endTime.Time
			event.EndTime = &t
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
		SELECT id, source, trigger_type, execution_id, change_type, timestamp, end_time, affected_services, metadata, summary
		FROM change_events
		WHERE id = $1
	`

	var event domain.ChangeEvent
	var metadataJSON []byte
	var affectedServices pq.StringArray
	var triggerType, executionID sql.NullString
	var endTime sql.NullTime

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&event.ID,
		&event.Source,
		&triggerType,
		&executionID,
		&event.ChangeType,
		&event.Timestamp,
		&endTime,
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

	event.TriggerType = triggerType.String
	event.ExecutionID = executionID.String
	if endTime.Valid {
		t := endTime.Time
		event.EndTime = &t
	}

	event.AffectedServices = []string(affectedServices)
	if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
		return domain.ChangeEvent{}, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return event, nil
}

func (s *PostgresStorage) GetServices(ctx context.Context) ([]string, error) {
	query := `
		SELECT DISTINCT unnest(affected_services) as service
		FROM change_events
		WHERE affected_services IS NOT NULL
		ORDER BY service ASC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get services: %w", err)
	}
	defer rows.Close()

	var services []string
	for rows.Next() {
		var service string
		if err := rows.Scan(&service); err != nil {
			return nil, err
		}
		services = append(services, service)
	}

	return services, nil
}

func (s *PostgresStorage) GetEventsPendingAnalysis(ctx context.Context) ([]domain.ChangeEvent, error) {
	query := `
		SELECT e.id, e.source, e.trigger_type, e.execution_id, e.change_type, e.timestamp, e.end_time, e.affected_services, e.metadata, e.summary
		FROM change_events e
		LEFT JOIN impact_analysis_snapshots s ON e.id = s.event_id
		WHERE s.event_id IS NULL
		ORDER BY e.timestamp DESC
		LIMIT 50
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending events: %w", err)
	}
	defer rows.Close()

	var events []domain.ChangeEvent
	for rows.Next() {
		var event domain.ChangeEvent
		var metadataJSON []byte
		var affectedServices pq.StringArray
		var triggerType, executionID sql.NullString
		var endTime sql.NullTime

		err := rows.Scan(
			&event.ID,
			&event.Source,
			&triggerType,
			&executionID,
			&event.ChangeType,
			&event.Timestamp,
			&endTime,
			&affectedServices,
			&metadataJSON,
			&event.Summary,
		)
		if err != nil {
			return nil, err
		}

		event.TriggerType = triggerType.String
		event.ExecutionID = executionID.String
		if endTime.Valid {
			t := endTime.Time
			event.EndTime = &t
		}
		event.AffectedServices = []string(affectedServices)
		json.Unmarshal(metadataJSON, &event.Metadata)

		events = append(events, event)
	}

	return events, nil
}

func (s *PostgresStorage) SaveImpactAnalysis(ctx context.Context, analysis domain.ImpactAnalysis) error {
	query := `
		INSERT INTO impact_analysis_snapshots 
		(event_id, created_at, baseline_metrics, impact_metrics, deltas, impact_score, impact_level)
		VALUES ($1, NOW(), $2, $3, $4, $5, $6)
		ON CONFLICT (event_id) DO UPDATE SET
			created_at = NOW(),
			baseline_metrics = EXCLUDED.baseline_metrics,
			impact_metrics = EXCLUDED.impact_metrics,
			deltas = EXCLUDED.deltas,
			impact_score = EXCLUDED.impact_score,
			impact_level = EXCLUDED.impact_level
	`

	baselineJSON, _ := json.Marshal(analysis.BaselineMetrics)
	impactJSON, _ := json.Marshal(analysis.ImpactMetrics)
	deltasJSON, _ := json.Marshal(analysis.Deltas)

	_, err := s.db.ExecContext(ctx, query,
		analysis.ChangeEvent.ID,
		baselineJSON,
		impactJSON,
		deltasJSON,
		analysis.ImpactScore,
		analysis.ImpactLevel,
	)

	if err != nil {
		return fmt.Errorf("failed to save impact analysis: %w", err)
	}

	return nil
}

func (s *PostgresStorage) GetImpactAnalysisByEventID(ctx context.Context, eventID string) (*domain.ImpactAnalysis, error) {
	query := `
		SELECT baseline_metrics, impact_metrics, deltas, impact_score, impact_level
		FROM impact_analysis_snapshots
		WHERE event_id = $1
	`

	var baselineJSON, impactJSON, deltasJSON []byte
	var analysis domain.ImpactAnalysis
	
	err := s.db.QueryRowContext(ctx, query, eventID).Scan(
		&baselineJSON,
		&impactJSON,
		&deltasJSON,
		&analysis.ImpactScore,
		&analysis.ImpactLevel,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Return nil if not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get impact analysis: %w", err)
	}

	if err := json.Unmarshal(baselineJSON, &analysis.BaselineMetrics); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(impactJSON, &analysis.ImpactMetrics); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(deltasJSON, &analysis.Deltas); err != nil {
		return nil, err
	}

	return &analysis, nil
}
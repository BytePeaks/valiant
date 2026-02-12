package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
	"valiant/internal/config"
	"valiant/internal/domain"

	"github.com/lib/pq"
)

const max_future_skew = 2 * time.Minute

type PostgresStorage struct {
	db     *sql.DB
	config *config.Config // Add config reference
}

func NewPostgresStorage(db *sql.DB, cfg *config.Config) *PostgresStorage {
	return &PostgresStorage{db: db, config: cfg}
}

func (s *PostgresStorage) RunMigration(schemaPath string) error {
	migrationName := filepath.Base(schemaPath)

	// Special handling for the initial schema_migrations table creation
	if migrationName != "000_create_schema_migrations_table.sql" {
		// Check if migration has already been applied
		var count int
		err := s.db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = $1", migrationName).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check migration status for %s: %w", migrationName, err)
		}
		if count > 0 {
			// fmt.Printf("Migration %s already applied, skipping.\n", migrationName) // Only uncomment for debugging
			return nil
		}
	}

	content, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read migration file %s: %w", migrationName, err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction for migration %s: %w", migrationName, err)
	}

	if _, err := tx.Exec(string(content)); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to execute migration %s: %w", migrationName, err)
	}

	// Record migration as applied, unless it's the schema_migrations table itself
	if migrationName != "000_create_schema_migrations_table.sql" {
		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", migrationName); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", migrationName, err)
		}
	} else {
		// If it's the schema_migrations table, we don't record it in itself during its creation.
		// It's considered implicitly applied once successfully executed.
		// Future runs will simply CREATE TABLE IF NOT EXISTS without issues.
	}

	return tx.Commit()
}

func (s *PostgresStorage) SaveChangeEvent(ctx context.Context, event domain.ChangeEvent) error {
	// Validate timestamps
	now := time.Now()
	limit := now.Add(max_future_skew)
	event.Status = "ready" // Default status

	if event.Timestamp.After(limit) {
		skew := event.Timestamp.Sub(now)
		event.Status = "invalid_time"
		event.InvalidReason = "timestamp_in_future"
		event.SkewSeconds = int(skew.Seconds())
		log.Printf(
			"Warning: Invalid ChangeEvent received. event_id=%s source=%s offending_field=timestamp skew_seconds=%d",
			event.ID,
			event.Source,
			event.SkewSeconds,
		)
	} else if event.EndTime != nil && event.EndTime.After(limit) {
		skew := event.EndTime.Sub(now)
		event.Status = "invalid_time"
		event.InvalidReason = "end_time_in_future"
		event.SkewSeconds = int(skew.Seconds())
		log.Printf(
			"Warning: Invalid ChangeEvent received. event_id=%s source=%s offending_field=end_time skew_seconds=%d",
			event.ID,
			event.Source,
			event.SkewSeconds,
		)
	}

	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	var blastRadiusJSON []byte
	if event.BlastRadius != nil {
		blastRadiusJSON, err = json.Marshal(event.BlastRadius)
		if err != nil {
			return fmt.Errorf("failed to marshal blast_radius: %w", err)
		}
	}

	// Compute dedup hash for API events with linking metadata (not K8s collector events which have deterministic IDs).
	// Includes timestamp (truncated to minute) so that distinct events at different times are not falsely deduped,
	// while true retries (same content within the same minute) are still caught.
	var dedupHash sql.NullString
	if event.Source != "kubernetes" {
		imageTag := event.Metadata["image_tag"]
		gitSha := event.Metadata["git_commit_sha"]
		if imageTag != "" || gitSha != "" {
			raw := strings.Join(event.AffectedServices, ",") + "|" + event.ChangeType + "|" + imageTag + "|" + gitSha + "|" + event.Timestamp.UTC().Truncate(time.Minute).Format(time.RFC3339)
			hash := sha256.Sum256([]byte(raw))
			dedupHash = sql.NullString{String: hex.EncodeToString(hash[:]), Valid: true}
		}
	}

	// Stage 1: Try insert with dedup_hash conflict check (silently drops content-duplicates)
	if dedupHash.Valid {
		dedupQuery := `
			INSERT INTO change_events (id, source, trigger_type, execution_id, change_type, timestamp, end_time, affected_services, metadata, summary, status, invalid_reason, skew_seconds, blast_radius, is_intent, is_execution, dedup_hash)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
			ON CONFLICT (dedup_hash) WHERE dedup_hash IS NOT NULL DO NOTHING
		`
		result, err := s.db.ExecContext(ctx, dedupQuery,
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
			event.Status,
			sql.NullString{String: event.InvalidReason, Valid: event.InvalidReason != ""},
			sql.NullInt32{Int32: int32(event.SkewSeconds), Valid: event.SkewSeconds != 0},
			blastRadiusJSON,
			event.IsIntent,
			event.IsExecution,
			dedupHash,
		)
		if err != nil {
			// If this fails due to id conflict, fall through to the upsert below
			if !isUniqueViolation(err) {
				return fmt.Errorf("failed to save change event: %w", err)
			}
		} else {
			rowsAffected, _ := result.RowsAffected()
			if rowsAffected == 0 {
				log.Printf("Duplicate event suppressed by dedup_hash for event_id=%s service=%s", event.ID, strings.Join(event.AffectedServices, ","))
				return nil
			}
			return nil // Successfully inserted
		}
	}

	// Stage 2: Standard upsert on id (handles same-ID re-submissions and K8s events)
	query := `
		INSERT INTO change_events (id, source, trigger_type, execution_id, change_type, timestamp, end_time, affected_services, metadata, summary, status, invalid_reason, skew_seconds, blast_radius, is_intent, is_execution, dedup_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (id) DO UPDATE SET
			source = EXCLUDED.source,
			trigger_type = EXCLUDED.trigger_type,
			execution_id = EXCLUDED.execution_id,
			change_type = EXCLUDED.change_type,
			timestamp = EXCLUDED.timestamp,
			end_time = EXCLUDED.end_time,
			affected_services = EXCLUDED.affected_services,
			metadata = EXCLUDED.metadata,
			summary = EXCLUDED.summary,
			status = EXCLUDED.status,
			invalid_reason = EXCLUDED.invalid_reason,
			skew_seconds = EXCLUDED.skew_seconds,
			blast_radius = EXCLUDED.blast_radius,
			is_intent = EXCLUDED.is_intent,
			is_execution = EXCLUDED.is_execution,
			dedup_hash = EXCLUDED.dedup_hash
	`

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
		event.Status,
		sql.NullString{String: event.InvalidReason, Valid: event.InvalidReason != ""},
		sql.NullInt32{Int32: int32(event.SkewSeconds), Valid: event.SkewSeconds != 0},
		blastRadiusJSON,
		event.IsIntent,
		event.IsExecution,
		dedupHash,
	)

	if err != nil {
		return fmt.Errorf("failed to save change event: %w", err)
	}

	return nil
}

// isUniqueViolation checks if a PostgreSQL error is a unique constraint violation.
func isUniqueViolation(err error) bool {
	if pqErr, ok := err.(*pq.Error); ok {
		return pqErr.Code == "23505"
	}
	return false
}

// generateContextualLinks creates deep links for a change event based on configured templates.
func (s *PostgresStorage) generateContextualLinks(event domain.ChangeEvent) []domain.ContextualLink {
	var links []domain.ContextualLink

	for _, tpl := range s.config.Linking {
		// Check if all required metadata keys are present
		hasAllKeys := true
		for _, key := range tpl.MetadataHas {
			if _, ok := event.Metadata[key]; !ok {
				hasAllKeys = false
				break
			}
			// Additionally, check if the value for the key is not an empty string
			if event.Metadata[key] == "" {
				hasAllKeys = false
				break
			}
		}

		if !hasAllKeys {
			continue
		}

		// Execute the template
		t, err := template.New(tpl.Name).Parse(tpl.URLTemplate)
		if err != nil {
			fmt.Printf("Error parsing link template '%s': %v\n", tpl.Name, err)
			continue
		}

		var buf bytes.Buffer
		if err := t.Execute(&buf, event.Metadata); err != nil {
			fmt.Printf("Error executing link template '%s': %v\n", tpl.Name, err)
			continue
		}

		generatedURL := buf.String()
		// If the template rendered "<no value>", it means a key was missing or empty, so skip this link.
		if strings.Contains(generatedURL, "<no value>") || generatedURL == "" {
			fmt.Printf("Skipping link '%s' due to missing metadata key or empty value in template: %s\n", tpl.Name, generatedURL)
			continue
		}

		links = append(links, domain.ContextualLink{
			Name: tpl.Name,
			URL:  generatedURL,
		})
	}

	return links
}

func (s *PostgresStorage) GetChangeEvents(ctx context.Context, filters map[string]interface{}) ([]domain.ChangeEvent, int, error) {
	var args []interface{}
	var whereClauses []string
	argCount := 1

	if triggerType, ok := filters["trigger_type"]; ok {
		whereClauses = append(whereClauses, fmt.Sprintf("trigger_type = $%d", argCount))
		args = append(args, triggerType)
		argCount++
	}
	if changeType, ok := filters["change_type"]; ok {
		whereClauses = append(whereClauses, fmt.Sprintf("change_type = $%d", argCount))
		args = append(args, changeType)
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
	if ns, ok := filters["namespace"]; ok {
		whereClauses = append(whereClauses, fmt.Sprintf("metadata ->> 'env' = $%d", argCount))
		args = append(args, ns)
		argCount++
	}
	if search, ok := filters["search"].(string); ok && search != "" {
		pattern := "%" + search + "%"
		whereClauses = append(whereClauses, fmt.Sprintf("(summary ILIKE $%d OR metadata::text ILIKE $%d)", argCount, argCount+1))
		args = append(args, pattern, pattern)
		argCount += 2
	}
	if metadata, ok := filters["metadata_has_any"].(map[string]string); ok && len(metadata) > 0 {
		var metadataClauses []string
		for key, value := range metadata {
			metadataClauses = append(metadataClauses, fmt.Sprintf("metadata @> $%d", argCount))
			jsonFilter := fmt.Sprintf(`{"%s": "%s"}`, key, value)
			args = append(args, jsonFilter)
			argCount++
		}
		whereClauses = append(whereClauses, "("+strings.Join(metadataClauses, " OR ")+")")
	}
	if services, ok := filters["services_any_of"].([]string); ok && len(services) > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf("affected_services && $%d", argCount))
		args = append(args, pq.Array(services))
		argCount++
	}
	if isExecutionOnly, ok := filters["is_execution_only"].(bool); ok && isExecutionOnly {
		whereClauses = append(whereClauses, "is_execution = TRUE")
	}
	if isIntentOnly, ok := filters["is_intent_only"].(bool); ok && isIntentOnly {
		whereClauses = append(whereClauses, "is_intent = TRUE")
	}
	if linkedOnly, ok := filters["linked_only"].(bool); ok && linkedOnly {
		whereClauses = append(whereClauses, `is_execution = TRUE AND EXISTS (
			SELECT 1 FROM event_links el
			WHERE el.execution_event_id = change_events.id
			AND el.link_type IN ('sha_match', 'image_tag_match', 'image_sha_inferred')
		)`)
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Count query
	countQuery := "SELECT COUNT(*) FROM change_events" + whereSQL
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count change events: %w", err)
	}

	// Pagination defaults
	limit := 50
	offset := 0
	if l, ok := filters["limit"].(int); ok && l > 0 {
		limit = l
		if limit > 200 {
			limit = 200
		}
	}
	if o, ok := filters["offset"].(int); ok && o >= 0 {
		offset = o
	}

	selectQuery := `
		SELECT id, source, trigger_type, execution_id, change_type, timestamp, end_time, affected_services, metadata, summary, status, invalid_reason, skew_seconds, blast_radius, is_intent, is_execution
		FROM change_events` + whereSQL + fmt.Sprintf(`
		ORDER BY timestamp DESC
		LIMIT %d OFFSET %d`, limit, offset)

	rows, err := s.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query change events: %w", err)
	}
	defer rows.Close()

	var events []domain.ChangeEvent
	for rows.Next() {
		var event domain.ChangeEvent
		var metadataJSON []byte
		var affectedServices pq.StringArray
		var triggerType, executionID, status, invalidReason sql.NullString
		var endTime sql.NullTime
		var skewSeconds sql.NullInt32
		var blastRadiusJSON sql.NullString
		var isIntent, isExecution sql.NullBool

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
			&status,
			&invalidReason,
			&skewSeconds,
			&blastRadiusJSON,
			&isIntent,
			&isExecution,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan change event: %w", err)
		}

		event.TriggerType = triggerType.String
		event.ExecutionID = executionID.String
		if endTime.Valid {
			t := endTime.Time
			event.EndTime = &t
		}

		event.Status = status.String
		event.InvalidReason = invalidReason.String
		event.SkewSeconds = int(skewSeconds.Int32)
		event.IsIntent = isIntent.Bool
		event.IsExecution = isExecution.Bool

		event.AffectedServices = []string(affectedServices)
		if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		if blastRadiusJSON.Valid && blastRadiusJSON.String != "" {
			var br domain.BlastRadius
			if err := json.Unmarshal([]byte(blastRadiusJSON.String), &br); err == nil {
				event.BlastRadius = &br
			}
		}

		events = append(events, event)
	}

	// Populate ContextualLinks for all retrieved events
	for i := range events {
		events[i].ContextualLinks = s.generateContextualLinks(events[i])
	}

	return events, total, nil
}

func (s *PostgresStorage) DeleteChangeEventsOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM change_events WHERE timestamp < $1", cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old change events: %w", err)
	}
	return result.RowsAffected()
}

func (s *PostgresStorage) GetChangeEventByID(ctx context.Context, id string) (domain.ChangeEvent, error) {
	query := `
		SELECT id, source, trigger_type, execution_id, change_type, timestamp, end_time, affected_services, metadata, summary, status, invalid_reason, skew_seconds, blast_radius, is_intent, is_execution
		FROM change_events
		WHERE id = $1
	`

	var event domain.ChangeEvent
	var metadataJSON []byte
	var affectedServices pq.StringArray
	var triggerType, executionID, status, invalidReason sql.NullString
	var endTime sql.NullTime
	var skewSeconds sql.NullInt32
	var blastRadiusJSON sql.NullString
	var isIntent, isExecution sql.NullBool

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
		&status,
		&invalidReason,
		&skewSeconds,
		&blastRadiusJSON,
		&isIntent,
		&isExecution,
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

	event.Status = status.String
	event.InvalidReason = invalidReason.String
	event.SkewSeconds = int(skewSeconds.Int32)

	event.AffectedServices = []string(affectedServices)
	if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
		return domain.ChangeEvent{}, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	if blastRadiusJSON.Valid && blastRadiusJSON.String != "" {
		var br domain.BlastRadius
		if err := json.Unmarshal([]byte(blastRadiusJSON.String), &br); err == nil {
			event.BlastRadius = &br
		}
	}

	event.ContextualLinks = s.generateContextualLinks(event)

	return event, nil
}
func (s *PostgresStorage) GetServices(ctx context.Context, namespace string, linkedOnly bool) ([]string, error) {
	var whereClauses []string
	var args []interface{}
	argCount := 1

	whereClauses = append(whereClauses, "affected_services IS NOT NULL")

	if namespace != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("metadata ->> 'env' = $%d", argCount))
		args = append(args, namespace)
		argCount++
	}

	if linkedOnly {
		whereClauses = append(whereClauses, `is_execution = TRUE AND EXISTS (
			SELECT 1 FROM event_links el
			WHERE el.execution_event_id = change_events.id
			AND el.link_type IN ('sha_match', 'image_tag_match', 'image_sha_inferred')
		)`)
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT unnest(affected_services) as service
		FROM change_events
		WHERE %s
		ORDER BY service ASC
	`, strings.Join(whereClauses, " AND "))

	rows, err := s.db.QueryContext(ctx, query, args...)
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
		SELECT e.id, e.source, e.trigger_type, e.execution_id, e.change_type, e.timestamp, e.end_time, e.affected_services, e.metadata, e.summary, e.blast_radius
		FROM change_events e
		LEFT JOIN impact_analysis_snapshots s ON e.id = s.event_id
		WHERE s.event_id IS NULL AND e.status = 'ready'
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
		var blastRadiusJSON sql.NullString

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
			&blastRadiusJSON,
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

		if blastRadiusJSON.Valid && blastRadiusJSON.String != "" {
			var br domain.BlastRadius
			if err := json.Unmarshal([]byte(blastRadiusJSON.String), &br); err == nil {
				event.BlastRadius = &br
			}
		}

		event.ContextualLinks = s.generateContextualLinks(event)
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

func (s *PostgresStorage) GetAnalyzedEventIDs(ctx context.Context, eventIDs []string) (map[string]bool, error) {
	if len(eventIDs) == 0 {
		return map[string]bool{}, nil
	}

	query := `SELECT event_id FROM impact_analysis_snapshots WHERE event_id = ANY($1)`
	rows, err := s.db.QueryContext(ctx, query, pq.Array(eventIDs))
	if err != nil {
		return nil, fmt.Errorf("failed to check analyzed event IDs: %w", err)
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result[id] = true
	}
	return result, nil
}

func (s *PostgresStorage) GetServicePreferences(ctx context.Context, serviceName string) ([]string, error) {
	query := `
		SELECT visible_metrics FROM service_preferences WHERE service_name = $1
	`
	var visibleMetrics pq.StringArray
	err := s.db.QueryRowContext(ctx, query, serviceName).Scan(&visibleMetrics)

	if err == sql.ErrNoRows {
		return []string{}, nil // Return empty slice if no preferences found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get service preferences for %s: %w", serviceName, err)
	}

	return []string(visibleMetrics), nil
}

func (s *PostgresStorage) SaveServicePreferences(ctx context.Context, serviceName string, visibleMetrics []string) error {
	query := `
		INSERT INTO service_preferences (service_name, visible_metrics)
		VALUES ($1, $2)
		ON CONFLICT (service_name) DO UPDATE SET
			visible_metrics = EXCLUDED.visible_metrics
	`
	_, err := s.db.ExecContext(ctx, query, serviceName, pq.Array(visibleMetrics))
	if err != nil {
		return fmt.Errorf("failed to save service preferences for %s: %w", serviceName, err)
	}
	return nil
}

func (s *PostgresStorage) GetNamespaces(ctx context.Context) ([]string, error) {
	query := `
		SELECT DISTINCT metadata ->> 'env' as namespace
		FROM change_events
		WHERE metadata ? 'env'
		ORDER BY namespace ASC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespaces: %w", err)
	}
	defer rows.Close()

	var namespaces []string
	for rows.Next() {
		var namespace string
		if err := rows.Scan(&namespace); err != nil {
			return nil, err
		}
		namespaces = append(namespaces, namespace)
	}

	return namespaces, nil
}

// SaveEventLink persists an event link with upsert semantics.
func (s *PostgresStorage) SaveEventLink(ctx context.Context, link domain.EventLink) error {
	query := `
		INSERT INTO event_links (id, intent_event_id, execution_event_id, link_type, confidence, created_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (intent_event_id, execution_event_id, link_type) DO UPDATE SET
			confidence = EXCLUDED.confidence,
			metadata = EXCLUDED.metadata
	`

	metadataJSON, err := json.Marshal(link.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal link metadata: %w", err)
	}

	_, err = s.db.ExecContext(ctx, query,
		link.ID,
		link.IntentEventID,
		link.ExecutionEventID,
		link.LinkType,
		link.Confidence,
		link.CreatedAt,
		metadataJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to save event link: %w", err)
	}

	return nil
}

// GetEventLinksByEventID returns all links where the given event is either intent or execution.
func (s *PostgresStorage) GetEventLinksByEventID(ctx context.Context, eventID string) ([]domain.EventLink, error) {
	query := `
		SELECT id, intent_event_id, execution_event_id, link_type, confidence, created_at, metadata
		FROM event_links
		WHERE intent_event_id = $1 OR execution_event_id = $1
		ORDER BY confidence DESC, created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to query event links: %w", err)
	}
	defer rows.Close()

	var links []domain.EventLink
	for rows.Next() {
		var link domain.EventLink
		var metadataJSON []byte

		err := rows.Scan(
			&link.ID,
			&link.IntentEventID,
			&link.ExecutionEventID,
			&link.LinkType,
			&link.Confidence,
			&link.CreatedAt,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event link: %w", err)
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &link.Metadata)
		}
		if link.Metadata == nil {
			link.Metadata = make(map[string]string)
		}

		links = append(links, link)
	}

	return links, nil
}

// GetEventLinksByExecutionID returns all links for a specific execution event.
func (s *PostgresStorage) GetEventLinksByExecutionID(ctx context.Context, executionEventID string) ([]domain.EventLink, error) {
	query := `
		SELECT id, intent_event_id, execution_event_id, link_type, confidence, created_at, metadata
		FROM event_links
		WHERE execution_event_id = $1
		ORDER BY confidence DESC, created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, executionEventID)
	if err != nil {
		return nil, fmt.Errorf("failed to query event links by execution: %w", err)
	}
	defer rows.Close()

	var links []domain.EventLink
	for rows.Next() {
		var link domain.EventLink
		var metadataJSON []byte

		err := rows.Scan(
			&link.ID,
			&link.IntentEventID,
			&link.ExecutionEventID,
			&link.LinkType,
			&link.Confidence,
			&link.CreatedAt,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event link: %w", err)
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &link.Metadata)
		}
		if link.Metadata == nil {
			link.Metadata = make(map[string]string)
		}

		links = append(links, link)
	}

	return links, nil
}

// GetRecentConfigChangeEvents returns config change events affecting a service within a time window.
func (s *PostgresStorage) GetRecentConfigChangeEvents(ctx context.Context, service string, before time.Time, within time.Duration) ([]domain.ChangeEvent, error) {
	windowStart := before.Add(-within)

	query := `
		SELECT id, source, trigger_type, execution_id, change_type, timestamp, end_time, affected_services, metadata, summary, status, invalid_reason, skew_seconds, blast_radius
		FROM change_events
		WHERE change_type IN ('configmap_update', 'secret_update')
		  AND affected_services && $1
		  AND timestamp >= $2
		  AND timestamp <= $3
		ORDER BY timestamp DESC
	`

	rows, err := s.db.QueryContext(ctx, query, pq.Array([]string{service}), windowStart, before)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent config changes: %w", err)
	}
	defer rows.Close()

	var events []domain.ChangeEvent
	for rows.Next() {
		var event domain.ChangeEvent
		var metadataJSON []byte
		var affectedServices pq.StringArray
		var triggerType, executionID, status, invalidReason sql.NullString
		var endTime sql.NullTime
		var skewSeconds sql.NullInt32
		var blastRadiusJSON sql.NullString

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
			&status,
			&invalidReason,
			&skewSeconds,
			&blastRadiusJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan config change event: %w", err)
		}

		event.TriggerType = triggerType.String
		event.ExecutionID = executionID.String
		if endTime.Valid {
			t := endTime.Time
			event.EndTime = &t
		}
		event.Status = status.String
		event.InvalidReason = invalidReason.String
		event.SkewSeconds = int(skewSeconds.Int32)
		event.AffectedServices = []string(affectedServices)

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &event.Metadata)
		}
		if event.Metadata == nil {
			event.Metadata = make(map[string]string)
		}

		if blastRadiusJSON.Valid && blastRadiusJSON.String != "" {
			var br domain.BlastRadius
			if err := json.Unmarshal([]byte(blastRadiusJSON.String), &br); err == nil {
				event.BlastRadius = &br
			}
		}

		events = append(events, event)
	}

	return events, nil
}

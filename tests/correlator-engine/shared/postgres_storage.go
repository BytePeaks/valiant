package shared

import (
	"context"
	"database/sql"
	"valiant/internal/config"
	"valiant/internal/domain"
	"valiant/internal/storage"
	"time"
)

// TestPostgresStorage embeds the production PostgresStorage and implements the storage.Storage interface for testing.
type TestPostgresStorage struct {
	*storage.PostgresStorage
}

// NewTestPostgresStorage creates a new TestPostgresStorage instance.
func NewTestPostgresStorage(db *sql.DB, cfg *config.Config) *TestPostgresStorage {
	return &TestPostgresStorage{
		PostgresStorage: storage.NewPostgresStorage(db, cfg),
	}
}

// RunMigration is a no-op for the test storage as migrations are handled by SetupTestDB.
func (s *TestPostgresStorage) RunMigration(schemaPath string) error {
	return nil
}

// Below methods simply delegate to the embedded PostgresStorage, satisfying the storage.Storage interface.

func (s *TestPostgresStorage) SaveChangeEvent(ctx context.Context, event domain.ChangeEvent) error {
	return s.PostgresStorage.SaveChangeEvent(ctx, event)
}

func (s *TestPostgresStorage) GetChangeEvents(ctx context.Context, filters map[string]interface{}) ([]domain.ChangeEvent, int, error) {
	return s.PostgresStorage.GetChangeEvents(ctx, filters)
}

func (s *TestPostgresStorage) GetChangeEventByID(ctx context.Context, id string) (domain.ChangeEvent, error) {
	return s.PostgresStorage.GetChangeEventByID(ctx, id)
}

func (s *TestPostgresStorage) GetServices(ctx context.Context) ([]string, error) {
	return s.PostgresStorage.GetServices(ctx)
}

func (s *TestPostgresStorage) GetNamespaces(ctx context.Context) ([]string, error) {
	return s.PostgresStorage.GetNamespaces(ctx)
}

func (s *TestPostgresStorage) GetEventsPendingAnalysis(ctx context.Context) ([]domain.ChangeEvent, error) {
	return s.PostgresStorage.GetEventsPendingAnalysis(ctx)
}

func (s *TestPostgresStorage) SaveImpactAnalysis(ctx context.Context, analysis domain.ImpactAnalysis) error {
	return s.PostgresStorage.SaveImpactAnalysis(ctx, analysis)
}

func (s *TestPostgresStorage) GetImpactAnalysisByEventID(ctx context.Context, eventID string) (*domain.ImpactAnalysis, error) {
	return s.PostgresStorage.GetImpactAnalysisByEventID(ctx, eventID)
}

func (s *TestPostgresStorage) GetServicePreferences(ctx context.Context, serviceName string) ([]string, error) {
	return s.PostgresStorage.GetServicePreferences(ctx, serviceName)
}

func (s *TestPostgresStorage) SaveServicePreferences(ctx context.Context, serviceName string, visibleMetrics []string) error {
	return s.PostgresStorage.SaveServicePreferences(ctx, serviceName, visibleMetrics)
}

func (s *TestPostgresStorage) GetAnalyzedEventIDs(ctx context.Context, eventIDs []string) (map[string]bool, error) {
	return s.PostgresStorage.GetAnalyzedEventIDs(ctx, eventIDs)
}

func (s *TestPostgresStorage) DeleteChangeEventsOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	return s.PostgresStorage.DeleteChangeEventsOlderThan(ctx, cutoff)
}

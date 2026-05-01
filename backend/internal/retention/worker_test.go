package retention

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
	"valiant/internal/domain"
)

// mockStorage is a mock implementation of the storage.Storage interface.
type mockStorage struct {
	deleteCalled  bool
	deleteCutoff  time.Time
	deleteCounter int64
	deleteError   error
	mu            sync.Mutex
}

func (m *mockStorage) SaveChangeEvent(ctx context.Context, event domain.ChangeEvent) error {
	return nil
}
func (m *mockStorage) GetChangeEvents(ctx context.Context, filters map[string]interface{}) ([]domain.ChangeEvent, int, error) {
	return nil, 0, nil
}
func (m *mockStorage) GetChangeEventByID(ctx context.Context, id string) (domain.ChangeEvent, error) {
	return domain.ChangeEvent{}, nil
}
func (m *mockStorage) GetServices(ctx context.Context, namespace string, linkedOnly bool) ([]string, error) {
	return nil, nil
}
func (m *mockStorage) GetNamespaces(ctx context.Context) ([]string, error) {
	return nil, nil
}
func (m *mockStorage) GetEventsPendingAnalysis(ctx context.Context) ([]domain.ChangeEvent, error) {
	return nil, nil
}
func (m *mockStorage) SaveImpactAnalysis(ctx context.Context, analysis domain.ImpactAnalysis) error {
	return nil
}
func (m *mockStorage) GetImpactAnalysisByEventID(ctx context.Context, eventID string) (*domain.ImpactAnalysis, error) {
	return nil, nil
}
func (m *mockStorage) GetServicePreferences(ctx context.Context, serviceName string) ([]string, error) {
	return nil, nil
}
func (m *mockStorage) SaveServicePreferences(ctx context.Context, serviceName string, visibleMetrics []string) error {
	return nil
}

func (m *mockStorage) GetAnalyzedEventIDs(ctx context.Context, eventIDs []string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (m *mockStorage) SaveEventLink(ctx context.Context, link domain.EventLink) error { return nil }
func (m *mockStorage) GetEventLinksByEventID(ctx context.Context, eventID string) ([]domain.EventLink, error) {
	return nil, nil
}
func (m *mockStorage) GetEventLinksByExecutionID(ctx context.Context, executionEventID string) ([]domain.EventLink, error) {
	return nil, nil
}
func (m *mockStorage) GetRecentConfigChangeEvents(ctx context.Context, service string, before time.Time, within time.Duration) ([]domain.ChangeEvent, error) {
	return nil, nil
}

func (m *mockStorage) DeleteChangeEventsOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteCalled = true
	m.deleteCutoff = cutoff
	return m.deleteCounter, m.deleteError
}

func (m *mockStorage) GetServicesHealth(ctx context.Context) ([]domain.ServiceHealth, error) {
	return nil, nil
}

func (m *mockStorage) GetSetting(ctx context.Context, key string) (string, error) { return "", nil }

func (m *mockStorage) SaveSetting(ctx context.Context, key, value string) error { return nil }

func TestWorker_DeletesOldEvents(t *testing.T) {
	ttl := 24 * time.Hour
	interval := 10 * time.Millisecond

	mockStore := &mockStorage{
		deleteCounter: 5, // Simulate 5 events deleted
	}

	worker := NewWorker(mockStore, func() time.Duration { return ttl })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the worker in a goroutine
	go worker.Start(ctx, interval)

	// Allow the worker to run at least once
	time.Sleep(interval * 2)

	// Stop the worker
	cancel()

	// Check the results
	mockStore.mu.Lock()
	defer mockStore.mu.Unlock()

	if !mockStore.deleteCalled {
		t.Errorf("expected DeleteChangeEventsOlderThan to be called, but it wasn't")
	}

	// Check if the cutoff time is approximately correct
	expectedCutoff := time.Now().Add(-ttl)
	if mockStore.deleteCutoff.IsZero() || mockStore.deleteCutoff.After(expectedCutoff.Add(time.Second)) || mockStore.deleteCutoff.Before(expectedCutoff.Add(-time.Second)) {
		t.Errorf("expected cutoff time to be around %v, but got %v", expectedCutoff, mockStore.deleteCutoff)
	}
}

func TestWorker_StorageError(t *testing.T) {
	ttl := 24 * time.Hour
	interval := 10 * time.Millisecond

	mockStore := &mockStorage{
		deleteError: errors.New("db error"),
	}

	worker := NewWorker(mockStore, func() time.Duration { return ttl })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the worker in a goroutine
	go worker.Start(ctx, interval)

	// Allow the worker to run at least once
	time.Sleep(interval * 2)

	// Stop the worker
	cancel()

	// Check the results
	mockStore.mu.Lock()
	defer mockStore.mu.Unlock()

	if !mockStore.deleteCalled {
		t.Errorf("expected DeleteChangeEventsOlderThan to be called, but it wasn't")
	}
}

func TestWorker_ZeroEventsDeleted(t *testing.T) {
	ttl := 24 * time.Hour
	interval := 10 * time.Millisecond

	mockStore := &mockStorage{
		deleteCounter: 0, // Simulate 0 events deleted
	}

	worker := NewWorker(mockStore, func() time.Duration { return ttl })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the worker in a goroutine
	go worker.Start(ctx, interval)

	// Allow the worker to run at least once
	time.Sleep(interval * 2)

	// Stop the worker
	cancel()

	// Check the results
	mockStore.mu.Lock()
	defer mockStore.mu.Unlock()

	if !mockStore.deleteCalled {
		t.Errorf("expected DeleteChangeEventsOlderThan to be called, but it wasn't")
	}
}

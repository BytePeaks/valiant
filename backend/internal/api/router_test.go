package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"valiant/internal/api"
	"valiant/internal/domain"
	"valiant/internal/config"
)

// MockStorage for API tests
type MockStorage struct {
	events []domain.ChangeEvent
}

func (m *MockStorage) SaveChangeEvent(ctx context.Context, event domain.ChangeEvent) error {
	m.events = append(m.events, event)
	return nil
}
func (m *MockStorage) GetChangeEvents(ctx context.Context, f map[string]interface{}) ([]domain.ChangeEvent, error) {
	return m.events, nil
}
func (m *MockStorage) GetChangeEventByID(ctx context.Context, id string) (domain.ChangeEvent, error) {
	for _, e := range m.events {
		if e.ID == id {
			return e, nil
		}
	}
	return domain.ChangeEvent{}, nil
}
func (m *MockStorage) GetServices(ctx context.Context) ([]string, error) { return nil, nil }
func (m *MockStorage) GetEventsPendingAnalysis(ctx context.Context) ([]domain.ChangeEvent, error) {
	return nil, nil
}
func (m *MockStorage) SaveImpactAnalysis(ctx context.Context, a domain.ImpactAnalysis) error { return nil }
func (m *MockStorage) GetImpactAnalysisByEventID(ctx context.Context, id string) (*domain.ImpactAnalysis, error) {
	return nil, nil
}
func (m *MockStorage) GetServicePreferences(ctx context.Context, serviceName string) ([]string, error) {
	return []string{}, nil
}
func (m *MockStorage) SaveServicePreferences(ctx context.Context, serviceName string, visibleMetrics []string) error {
	return nil
}

func (m *MockStorage) GetNamespaces(ctx context.Context) ([]string, error) {
	return []string{"production", "staging"}, nil
}

// MockMetrics for API tests
type MockMetrics struct{}

func (m *MockMetrics) GetAverageMetrics(ctx context.Context, services []string, start, end time.Time) (domain.MetricValues, error) {
	return domain.MetricValues{}, nil
}

func (m *MockMetrics) GetAvailableMetrics() []domain.MetricInfo {
	return []domain.MetricInfo{
		{Name: "error_rate", Icon: "AlertCircle"},
		{Name: "latency_p95_ms", Icon: "Clock"},
	}
}

func TestHealthCheck(t *testing.T) {
	cfg := &config.Config{}
	router := api.NewRouter(&MockStorage{}, nil, &MockMetrics{}, cfg)
	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", res.StatusCode)
	}
}

func TestPostEvent(t *testing.T) {
	store := &MockStorage{}
	cfg := &config.Config{}
	router := api.NewRouter(store, nil, &MockMetrics{}, cfg)
	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	event := domain.ChangeEvent{ID: "test-1", Summary: "Test Event"}
	body, _ := json.Marshal(event)
	
	res, err := http.Post(ts.URL+"/api/v1/events", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", res.StatusCode)
	}

	if len(store.events) != 1 {
		t.Errorf("expected 1 event in store, got %d", len(store.events))
	}
}

func TestGetMetrics(t *testing.T) {
	cfg := &config.Config{}
	router := api.NewRouter(&MockStorage{}, nil, &MockMetrics{}, cfg)
	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/v1/metrics")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", res.StatusCode)
	}

	var metrics []domain.MetricInfo
	if err := json.NewDecoder(res.Body).Decode(&metrics); err != nil {
		t.Fatal(err)
	}

	if len(metrics) != 2 {
		t.Errorf("expected 2 metrics, got %d", len(metrics))
	}

	if metrics[0].Name != "error_rate" || metrics[1].Name != "latency_p95_ms" {
		t.Errorf("unexpected metrics: %v", metrics)
	}
}

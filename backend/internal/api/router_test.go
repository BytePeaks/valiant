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
	"valiant/internal/config"
	"valiant/internal/domain"
)

// MockStorage for API tests
type MockStorage struct {
	events           []domain.ChangeEvent
	lastFilters      map[string]interface{}
	analyzedEventIDs map[string]bool
}

func (m *MockStorage) SaveChangeEvent(ctx context.Context, event domain.ChangeEvent) error {
	m.events = append(m.events, event)
	return nil
}
func (m *MockStorage) GetChangeEvents(ctx context.Context, f map[string]interface{}) ([]domain.ChangeEvent, int, error) {
	m.lastFilters = f
	return m.events, len(m.events), nil
}
func (m *MockStorage) DeleteChangeEventsOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}
func (m *MockStorage) GetChangeEventByID(ctx context.Context, id string) (domain.ChangeEvent, error) {
	for _, e := range m.events {
		if e.ID == id {
			return e, nil
		}
	}
	return domain.ChangeEvent{}, nil
}
func (m *MockStorage) GetServices(ctx context.Context, namespace string, linkedOnly bool) ([]string, error) {
	return nil, nil
}
func (m *MockStorage) GetEventsPendingAnalysis(ctx context.Context) ([]domain.ChangeEvent, error) {
	return nil, nil
}
func (m *MockStorage) SaveImpactAnalysis(ctx context.Context, a domain.ImpactAnalysis) error {
	return nil
}
func (m *MockStorage) GetImpactAnalysisByEventID(ctx context.Context, id string) (*domain.ImpactAnalysis, error) {
	return nil, nil
}
func (m *MockStorage) GetServicePreferences(ctx context.Context, serviceName string) ([]string, error) {
	return []string{}, nil
}
func (m *MockStorage) SaveServicePreferences(ctx context.Context, serviceName string, visibleMetrics []string) error {
	return nil
}

func (m *MockStorage) GetAnalyzedEventIDs(ctx context.Context, eventIDs []string) (map[string]bool, error) {
	if m.analyzedEventIDs != nil {
		result := make(map[string]bool)
		for _, id := range eventIDs {
			if m.analyzedEventIDs[id] {
				result[id] = true
			}
		}
		return result, nil
	}
	return map[string]bool{}, nil
}

func (m *MockStorage) GetNamespaces(ctx context.Context) ([]string, error) {
	return []string{"production", "staging"}, nil
}

func (m *MockStorage) SaveEventLink(ctx context.Context, link domain.EventLink) error { return nil }
func (m *MockStorage) GetEventLinksByEventID(ctx context.Context, eventID string) ([]domain.EventLink, error) {
	return nil, nil
}
func (m *MockStorage) GetEventLinksByExecutionID(ctx context.Context, executionEventID string) ([]domain.EventLink, error) {
	return nil, nil
}
func (m *MockStorage) GetRecentConfigChangeEvents(ctx context.Context, service string, before time.Time, within time.Duration) ([]domain.ChangeEvent, error) {
	return nil, nil
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

func TestGetEventsPaginatedResponse(t *testing.T) {
	store := &MockStorage{
		events: []domain.ChangeEvent{
			{ID: "e1", Summary: "Event 1"},
			{ID: "e2", Summary: "Event 2"},
		},
	}
	cfg := &config.Config{}
	router := api.NewRouter(store, nil, &MockMetrics{}, cfg)
	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/v1/events")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", res.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	if _, ok := result["events"]; !ok {
		t.Error("response missing 'events' key")
	}
	if _, ok := result["total"]; !ok {
		t.Error("response missing 'total' key")
	}
	if _, ok := result["limit"]; !ok {
		t.Error("response missing 'limit' key")
	}
	if _, ok := result["offset"]; !ok {
		t.Error("response missing 'offset' key")
	}

	total := int(result["total"].(float64))
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	limit := int(result["limit"].(float64))
	if limit != 50 {
		t.Errorf("expected default limit 50, got %d", limit)
	}
	offset := int(result["offset"].(float64))
	if offset != 0 {
		t.Errorf("expected default offset 0, got %d", offset)
	}
}

func TestGetEventsPaginationParams(t *testing.T) {
	store := &MockStorage{}
	cfg := &config.Config{}
	router := api.NewRouter(store, nil, &MockMetrics{}, cfg)
	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/v1/events?limit=10&offset=20")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", res.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	limit := int(result["limit"].(float64))
	if limit != 10 {
		t.Errorf("expected limit 10, got %d", limit)
	}
	offset := int(result["offset"].(float64))
	if offset != 20 {
		t.Errorf("expected offset 20, got %d", offset)
	}

	// Verify filters were passed to storage
	if store.lastFilters["limit"] != 10 {
		t.Errorf("expected limit filter 10, got %v", store.lastFilters["limit"])
	}
	if store.lastFilters["offset"] != 20 {
		t.Errorf("expected offset filter 20, got %v", store.lastFilters["offset"])
	}
}

func TestGetEventsFilterParams(t *testing.T) {
	store := &MockStorage{}
	cfg := &config.Config{}
	router := api.NewRouter(store, nil, &MockMetrics{}, cfg)
	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/v1/events?service=api,web&change_type=deployment_rollout&namespace=production&search=rollout&from=2025-01-01T00:00:00Z&to=2025-12-31T23:59:59Z")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", res.StatusCode)
	}

	f := store.lastFilters
	services, ok := f["services_any_of"].([]string)
	if !ok || len(services) != 2 || services[0] != "api" || services[1] != "web" {
		t.Errorf("expected services [api web], got %v", f["services_any_of"])
	}
	if f["change_type"] != "deployment_rollout" {
		t.Errorf("expected change_type deployment_rollout, got %v", f["change_type"])
	}
	if f["namespace"] != "production" {
		t.Errorf("expected namespace production, got %v", f["namespace"])
	}
	if f["search"] != "rollout" {
		t.Errorf("expected search rollout, got %v", f["search"])
	}

	fromTime, ok := f["from_timestamp"].(time.Time)
	if !ok {
		t.Error("expected from_timestamp to be time.Time")
	} else if fromTime.Year() != 2025 || fromTime.Month() != 1 {
		t.Errorf("unexpected from_timestamp: %v", fromTime)
	}

	toTime, ok := f["to_timestamp"].(time.Time)
	if !ok {
		t.Error("expected to_timestamp to be time.Time")
	} else if toTime.Year() != 2025 || toTime.Month() != 12 {
		t.Errorf("unexpected to_timestamp: %v", toTime)
	}
}

func TestGetEventsEmptyReturnsArray(t *testing.T) {
	store := &MockStorage{}
	cfg := &config.Config{}
	router := api.NewRouter(store, nil, &MockMetrics{}, cfg)
	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/v1/events")
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	events, ok := result["events"].([]interface{})
	if !ok {
		t.Fatal("expected events to be an array")
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestGetEventsContainsAnalysisStatus(t *testing.T) {
	cfg := &config.Config{}
	cfg.Analysis.ImpactDur = 30 * time.Minute

	oldEvent := domain.ChangeEvent{
		ID:        "old-evt",
		Summary:   "Old event",
		Timestamp: time.Now().Add(-2 * time.Hour),
	}
	recentEvent := domain.ChangeEvent{
		ID:        "recent-evt",
		Summary:   "Recent event",
		Timestamp: time.Now().Add(-1 * time.Minute),
	}
	analyzedEvent := domain.ChangeEvent{
		ID:        "analyzed-evt",
		Summary:   "Analyzed event",
		Timestamp: time.Now().Add(-3 * time.Hour),
	}

	store := &MockStorage{
		events:           []domain.ChangeEvent{oldEvent, recentEvent, analyzedEvent},
		analyzedEventIDs: map[string]bool{"analyzed-evt": true},
	}

	router := api.NewRouter(store, nil, &MockMetrics{}, cfg)
	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/v1/events")
	if err != nil {
		t.Fatal(err)
	}

	var result struct {
		Events []struct {
			ID             string `json:"id"`
			AnalysisStatus string `json:"analysis_status"`
		} `json:"events"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	if len(result.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(result.Events))
	}

	statusByID := make(map[string]string)
	for _, e := range result.Events {
		statusByID[e.ID] = e.AnalysisStatus
	}

	// Old event with no analysis and closed window should be "ready"
	if statusByID["old-evt"] != "ready" {
		t.Errorf("expected old-evt status 'ready', got '%s'", statusByID["old-evt"])
	}

	// Recent event with open window should be "pending"
	if statusByID["recent-evt"] != "pending" {
		t.Errorf("expected recent-evt status 'pending', got '%s'", statusByID["recent-evt"])
	}

	// Analyzed event should be "completed"
	if statusByID["analyzed-evt"] != "completed" {
		t.Errorf("expected analyzed-evt status 'completed', got '%s'", statusByID["analyzed-evt"])
	}
}

func TestRankingsEndpoint_MissingParams(t *testing.T) {
	cfg := &config.Config{}
	router := api.NewRouter(&MockStorage{}, nil, &MockMetrics{}, cfg)
	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	// Missing service
	res, err := http.Get(ts.URL + "/api/v1/rankings?from=2025-01-01T00:00:00Z&to=2025-01-02T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing service, got %d", res.StatusCode)
	}

	// Missing from/to
	res, err = http.Get(ts.URL + "/api/v1/rankings?service=api")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing from/to, got %d", res.StatusCode)
	}

	// Invalid timestamp format
	res, err = http.Get(ts.URL + "/api/v1/rankings?service=api&from=not-a-date&to=2025-01-02T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid from, got %d", res.StatusCode)
	}
}

func TestRankingsEndpoint_MethodNotAllowed(t *testing.T) {
	cfg := &config.Config{}
	router := api.NewRouter(&MockStorage{}, nil, &MockMetrics{}, cfg)
	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	res, err := http.Post(ts.URL+"/api/v1/rankings", "application/json", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST, got %d", res.StatusCode)
	}
}

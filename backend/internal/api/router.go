package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"valiant/internal/config"
	"valiant/internal/correlator"
	"valiant/internal/domain"
	"valiant/internal/metrics"
	"valiant/internal/storage"
)

type Router struct {
	storage    storage.Storage
	correlator *correlator.Engine
	metrics    metrics.MetricsProvider
	config     *config.Config
}

func NewRouter(s storage.Storage, c *correlator.Engine, m metrics.MetricsProvider, cfg *config.Config) *Router {
	return &Router{
		storage:    s,
		correlator: c,
		metrics:    m,
		config:     cfg,
	}
}

func (router *Router) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("/api/v1/events", router.handleEvents)
	mux.HandleFunc("/api/v1/services", router.handleServices)
	mux.HandleFunc("/api/v1/services/", router.handleServicePreferences)
	mux.HandleFunc("/api/v1/analyze", router.handleAnalysis)
	mux.HandleFunc("/api/v1/metrics", router.handleMetrics)
	mux.HandleFunc("/api/v1/namespaces", router.handleNamespaces)

	return corsMiddleware(mux)
}

func (router *Router) handleServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	services, err := router.storage.GetServices(r.Context())
	if err != nil {
		http.Error(w, "Failed to fetch services", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(services)
}

func (router *Router) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	metricInfo := router.metrics.GetAvailableMetrics()
	json.NewEncoder(w).Encode(metricInfo)
}

func (router *Router) handleServicePreferences(w http.ResponseWriter, r *http.Request) {
	serviceName := strings.TrimPrefix(r.URL.Path, "/api/v1/services/")
	serviceName = strings.TrimSuffix(serviceName, "/preferences")

	if r.Method == http.MethodGet {
		preferences, err := router.storage.GetServicePreferences(r.Context(), serviceName)
		if err != nil {
			http.Error(w, "Failed to get service preferences", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(preferences)
		return
	}

	if r.Method == http.MethodPost {
		var preferences []string
		if err := json.NewDecoder(r.Body).Decode(&preferences); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := router.storage.SaveServicePreferences(r.Context(), serviceName, preferences); err != nil {
			http.Error(w, "Failed to save service preferences", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// eventWithStatus wraps a ChangeEvent with its analysis status for the API response.
type eventWithStatus struct {
	domain.ChangeEvent
	AnalysisStatus string `json:"analysis_status"` // "pending", "ready", "completed"
}

// computeAnalysisStatus determines the analysis state for an event.
// "completed" = snapshot exists, "pending" = impact window still open, "ready" = window closed, no analysis yet.
func (router *Router) computeAnalysisStatus(event domain.ChangeEvent, analyzed bool) string {
	if analyzed {
		return "completed"
	}

	impactDur := router.config.Analysis.ImpactDur
	impactPivot := event.Timestamp
	if event.EndTime != nil {
		impactPivot = *event.EndTime
	}
	impactEnd := impactPivot.Add(5 * time.Minute).Add(impactDur)

	if time.Now().UTC().Before(impactEnd) {
		return "pending"
	}
	return "ready"
}

func (router *Router) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var event domain.ChangeEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := router.storage.SaveChangeEvent(r.Context(), event); err != nil {
			http.Error(w, "Failed to save event", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		return
	}

	if r.Method == http.MethodGet {
		q := r.URL.Query()
		filters := make(map[string]interface{})

		if v := q.Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				filters["limit"] = n
			}
		}
		if v := q.Get("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				filters["offset"] = n
			}
		}
		if v := q.Get("service"); v != "" {
			filters["services_any_of"] = strings.Split(v, ",")
		}
		if v := q.Get("namespace"); v != "" {
			filters["namespace"] = v
		}
		if v := q.Get("change_type"); v != "" {
			filters["change_type"] = v
		}
		if v := q.Get("from"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				filters["from_timestamp"] = t
			}
		}
		if v := q.Get("to"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				filters["to_timestamp"] = t
			}
		}
		if v := q.Get("search"); v != "" {
			filters["search"] = v
		}

		events, total, err := router.storage.GetChangeEvents(r.Context(), filters)
		if err != nil {
			http.Error(w, "Failed to fetch events", http.StatusInternalServerError)
			return
		}

		if events == nil {
			events = []domain.ChangeEvent{}
		}

		// Batch-check which events have analysis snapshots
		eventIDs := make([]string, len(events))
		for i, e := range events {
			eventIDs[i] = e.ID
		}
		analyzedMap, err := router.storage.GetAnalyzedEventIDs(r.Context(), eventIDs)
		if err != nil {
			http.Error(w, "Failed to check analysis status", http.StatusInternalServerError)
			return
		}

		// Enrich events with analysis status
		enriched := make([]eventWithStatus, len(events))
		for i, e := range events {
			enriched[i] = eventWithStatus{
				ChangeEvent:    e,
				AnalysisStatus: router.computeAnalysisStatus(e, analyzedMap[e.ID]),
			}
		}

		limit := 50
		if l, ok := filters["limit"].(int); ok && l > 0 {
			limit = l
			if limit > 200 {
				limit = 200
			}
		}
		offset := 0
		if o, ok := filters["offset"].(int); ok && o >= 0 {
			offset = o
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"events": enriched,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (router *Router) handleAnalysis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		EventID string `json:"event_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	event, err := router.storage.GetChangeEventByID(r.Context(), req.EventID)
	if err != nil {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	analysis, err := router.correlator.AnalyzeImpact(r.Context(), event)
	if err == correlator.ErrImpactWindowNotClosed {
		w.WriteHeader(http.StatusUnprocessableEntity) // 422 indicates semantic issue (too early)
		json.NewEncoder(w).Encode(analysis)
		return
	}
	if err != nil {
		http.Error(w, "Analysis failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(analysis)
}

func (router *Router) handleNamespaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	dbNamespaces, err := router.storage.GetNamespaces(r.Context())
	if err != nil {
		http.Error(w, "Failed to fetch namespaces from database", http.StatusInternalServerError)
		return
	}

	// Merge with namespaces from config
	configNamespaces := router.config.Kubernetes.Namespaces
	
	// Use a map to store unique namespaces
	allNamespaces := make(map[string]bool)
	for _, ns := range dbNamespaces {
		allNamespaces[ns] = true
	}
	for _, ns := range configNamespaces {
		allNamespaces[ns] = true
	}

	// Convert map keys to a slice
	uniqueNamespaces := make([]string, 0, len(allNamespaces))
	for ns := range allNamespaces {
		uniqueNamespaces = append(uniqueNamespaces, ns)
	}
	
	json.NewEncoder(w).Encode(uniqueNamespaces)
}

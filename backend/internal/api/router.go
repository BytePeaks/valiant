package api

import (
	"encoding/json"
	"net/http"
	"valiant/internal/correlator"
	"valiant/internal/domain"
	"valiant/internal/metrics" // Import metrics package
	"valiant/internal/storage"
	"valiant/internal/config"
	"strings"
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
		events, err := router.storage.GetChangeEvents(r.Context(), nil)
		if err != nil {
			http.Error(w, "Failed to fetch events", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(events)
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

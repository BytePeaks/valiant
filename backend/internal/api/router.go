package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"valiant/internal/config"
	"valiant/internal/correlator"
	"valiant/internal/domain"
	"valiant/internal/metrics"
	"valiant/internal/storage"
)

type Router struct {
	storage      storage.Storage
	correlator   *correlator.Engine
	metrics      metrics.MetricsProvider
	config       *config.Config
	retentionTTL *atomic.Int64
}

func NewRouter(s storage.Storage, c *correlator.Engine, m metrics.MetricsProvider, cfg *config.Config, retentionTTL *atomic.Int64) *Router {
	return &Router{
		storage:      s,
		correlator:   c,
		metrics:      m,
		config:       cfg,
		retentionTTL: retentionTTL,
	}
}

func (router *Router) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("/api/v1/events", router.handleEvents)
	mux.HandleFunc("/api/v1/events/", router.handleEventLinks)
	mux.HandleFunc("/api/v1/services", router.handleServices)
	mux.HandleFunc("/api/v1/services/health", router.handleServicesHealth)
	mux.HandleFunc("/api/v1/services/", router.handleServicePreferences)
	mux.HandleFunc("/api/v1/analyze", router.handleAnalysis)
	mux.HandleFunc("/api/v1/metrics", router.handleMetrics)
	mux.HandleFunc("/api/v1/metrics/custom", router.handleCustomMetrics)
	mux.HandleFunc("/api/v1/namespaces", router.handleNamespaces)
	mux.HandleFunc("/api/v1/rankings", router.handleRankings)
	mux.HandleFunc("/api/v1/settings/retention", router.handleRetentionSettings)

	return corsMiddleware(mux)
}

func (router *Router) handleServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	namespace := r.URL.Query().Get("namespace")
	linkedOnly := r.URL.Query().Get("linked_only") == "true"

	// Validate namespace against config if namespaces are configured
	if namespace != "" && len(router.config.Kubernetes.Namespaces) > 0 {
		allowed := false
		for _, ns := range router.config.Kubernetes.Namespaces {
			if ns == namespace {
				allowed = true
				break
			}
		}
		if !allowed {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]string{})
			return
		}
	}

	services, err := router.storage.GetServices(r.Context(), namespace, linkedOnly)
	if err != nil {
		http.Error(w, "Failed to fetch services", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(services)
}

func (router *Router) handleServicesHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	health, err := router.storage.GetServicesHealth(r.Context())
	if err != nil {
		http.Error(w, "Failed to fetch services health", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

type retentionSettingsRequest struct {
	EventTTL string `json:"event_ttl"`
}

type retentionSettingsResponse struct {
	EventTTL    string `json:"event_ttl"`
	EventTTLDur int64  `json:"event_ttl_seconds"`
}

func (router *Router) handleRetentionSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		ttl := time.Duration(router.retentionTTL.Load())
		json.NewEncoder(w).Encode(retentionSettingsResponse{
			EventTTL:    router.config.Retention.EventTTL,
			EventTTLDur: int64(ttl.Seconds()),
		})
		return
	}

	if r.Method == http.MethodPost {
		var req retentionSettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.EventTTL == "" {
			http.Error(w, "event_ttl is required", http.StatusBadRequest)
			return
		}
		d, err := config.ParseDuration(req.EventTTL)
		if err != nil || d <= 0 {
			http.Error(w, "invalid event_ttl: use formats like 90d, 30d, 168h", http.StatusBadRequest)
			return
		}
		if err := router.storage.SaveSetting(r.Context(), "retention_event_ttl", req.EventTTL); err != nil {
			http.Error(w, "failed to save setting", http.StatusInternalServerError)
			return
		}
		router.retentionTTL.Store(int64(d))
		router.config.Retention.EventTTL = req.EventTTL
		router.config.Retention.EventTTLDur = d
		json.NewEncoder(w).Encode(retentionSettingsResponse{
			EventTTL:    req.EventTTL,
			EventTTLDur: int64(d.Seconds()),
		})
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (router *Router) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	metricInfo := router.metrics.GetAvailableMetrics()
	if metricInfo == nil {
		http.Error(w, `{"error":"Prometheus not configured"}`, http.StatusServiceUnavailable)
		return
	}
	json.NewEncoder(w).Encode(metricInfo)
}

func (router *Router) handleCustomMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	json.NewEncoder(w).Encode(router.config.Prometheus.AdditionalMetrics)
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
	AnalysisStatus string              `json:"analysis_status"`           // "pending", "ready", "completed"
	LinkedIntent   *domain.ChangeEvent `json:"linked_intent,omitempty"`   // The CI event that triggered this execution
	LinkType       string              `json:"link_type,omitempty"`       // "sha_match" or "image_tag_match"
	LinkConfidence float64             `json:"link_confidence,omitempty"` // 0.0 to 1.0
	LinkReason     string              `json:"link_reason,omitempty"`     // Human-readable reason from link metadata
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

		// Set IsIntent and IsExecution based on TriggerType for API-submitted events
		if isExecutionTriggerType(event.TriggerType) {
			event.IsExecution = true
			event.IsIntent = false
		} else { // Default or explicit intent type
			event.IsIntent = true
			event.IsExecution = false
		}

		if err := router.storage.SaveChangeEvent(r.Context(), event); err != nil {
			http.Error(w, "Failed to save event", http.StatusInternalServerError)
			return
		}

		// Eager linking: attempt to create intent-execution links immediately
		if event.IsExecution {
			links, err := router.correlator.CreateIntentExecutionLinks(r.Context(), event)
			if err != nil {
				log.Printf("Warning: eager linking failed for execution event %s: %v", event.ID, err)
			} else if len(links) == 0 {
				log.Printf("Orphaned execution event %s: %s (no matching CI event found)", event.ID, event.Summary)
			}
		} else if event.IsIntent {
			links, err := router.correlator.LinkIntentToExecutions(r.Context(), event)
			if err != nil {
				log.Printf("Warning: eager linking failed for intent event %s: %v", event.ID, err)
			} else if len(links) > 0 {
				log.Printf("Intent event %s linked to %d execution(s)", event.ID, len(links))
			}
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

		if v := q.Get("is_execution_only"); v != "" {
			if b, err := strconv.ParseBool(v); err == nil && b {
				filters["is_execution_only"] = true
			}
		}

		if v := q.Get("linked_only"); v != "" {
			if b, err := strconv.ParseBool(v); err == nil && b {
				filters["linked_only"] = true
			}
		}
		if v := q.Get("git_sha"); v != "" {
			filters["git_sha"] = v
		}
		if v := q.Get("metadata_key"); v != "" {
			filters["metadata_key"] = v
		}
		if v := q.Get("metadata_value"); v != "" {
			filters["metadata_value"] = v
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

		// Enrich events with analysis status and linked intent info
		enriched := make([]eventWithStatus, len(events))
		for i, e := range events {
			ews := eventWithStatus{
				ChangeEvent:    e,
				AnalysisStatus: router.computeAnalysisStatus(e, analyzedMap[e.ID]),
			}

			// Look up linked intent for execution events
			if e.IsExecution {
				links, err := router.storage.GetEventLinksByExecutionID(r.Context(), e.ID)
				if err == nil {
					for _, link := range links {
						if link.LinkType == "sha_match" || link.LinkType == "image_tag_match" || link.LinkType == "image_sha_inferred" {
							intentEvent, err := router.storage.GetChangeEventByID(r.Context(), link.IntentEventID)
							if err == nil {
								ews.LinkedIntent = &intentEvent
								ews.LinkType = link.LinkType
								ews.LinkConfidence = link.Confidence
								ews.LinkReason = link.Metadata["reason"]
							}
							break // Use the highest-confidence link (already sorted by confidence DESC)
						}
					}
				}
			}

			enriched[i] = ews
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

func (router *Router) handleEventLinks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Parse event ID from path: /api/v1/events/{id}/links
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/events/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 || parts[1] != "links" || parts[0] == "" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	eventID := parts[0]

	links, err := router.storage.GetEventLinksByEventID(r.Context(), eventID)
	if err != nil {
		http.Error(w, "Failed to fetch event links", http.StatusInternalServerError)
		return
	}

	if links == nil {
		links = []domain.EventLink{}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"event_id": eventID,
		"links":    links,
		"total":    len(links),
	})
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
	if errors.Is(err, metrics.ErrPrometheusUnavailable) {
		http.Error(w, `{"error":"Prometheus not configured. Set prometheus.url in config or ensure Prometheus is discoverable in-cluster."}`, http.StatusServiceUnavailable)
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

	// If namespaces are explicitly configured, return only those
	if len(router.config.Kubernetes.Namespaces) > 0 {
		ns := make([]string, len(router.config.Kubernetes.Namespaces))
		copy(ns, router.config.Kubernetes.Namespaces)
		sort.Strings(ns)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ns)
		return
	}

	// Fallback: discover from DB when no namespaces configured
	dbNamespaces, err := router.storage.GetNamespaces(r.Context())
	if err != nil {
		http.Error(w, "Failed to fetch namespaces from database", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dbNamespaces)
}

func (router *Router) handleRankings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	service := q.Get("service")
	if service == "" {
		http.Error(w, "service parameter is required", http.StatusBadRequest)
		return
	}

	fromStr := q.Get("from")
	toStr := q.Get("to")
	if fromStr == "" || toStr == "" {
		http.Error(w, "from and to parameters are required (RFC3339 format)", http.StatusBadRequest)
		return
	}

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		http.Error(w, "invalid from parameter: expected RFC3339 format", http.StatusBadRequest)
		return
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		http.Error(w, "invalid to parameter: expected RFC3339 format", http.StatusBadRequest)
		return
	}

	ranked, err := router.correlator.RankChanges(r.Context(), service, from, to)
	if err != nil {
		http.Error(w, "Ranking failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"service": service,
		"from":    from,
		"to":      to,
		"ranked":  ranked,
		"total":   len(ranked),
	})
}

// isIntentTriggerType determines if a trigger type generally represents an 'intent' event.
func isIntentTriggerType(triggerType string) bool {
	t := strings.ToLower(triggerType)
	return t == "ci" || t == "build_success" || t == "cicd_deploy_intent" ||
		t == "github-actions" || t == "gitlab-ci" || t == "jenkins" || t == "codebuild"
}

// isExecutionTriggerType determines if a trigger type generally represents an 'execution' event (non-k8s).
func isExecutionTriggerType(triggerType string) bool {
	t := strings.ToLower(triggerType)
	return t == "manual" || t == "gitops" || t == "kubernetes-api" || t == "argocd" || t == "gitops-config"
}

package api

import (
	"encoding/json"
	"net/http"
	"valiant/internal/correlator"
	"valiant/internal/domain"
	"valiant/internal/storage"
)

type Router struct {
	storage    storage.Storage
	correlator *correlator.Engine
}

func NewRouter(s storage.Storage, c *correlator.Engine) *Router {
	return &Router{
		storage:    s,
		correlator: c,
	}
}

func (router *Router) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	mux.HandleFunc("/api/v1/events", router.handleEvents)

	return mux
}

func (router *Router) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// Example: Receive a manual change event
		var event domain.ChangeEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// In a real scenario, we'd save it and maybe trigger analysis
		w.WriteHeader(http.StatusCreated)
		return
	}
	// TODO: GET for listing events
	w.WriteHeader(http.StatusMethodNotAllowed)
}

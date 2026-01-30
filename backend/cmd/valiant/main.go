package main

import (
	"fmt"
	"log"
	"net/http"
	"valiant/internal/api"
	"valiant/internal/correlator"
	"valiant/internal/metrics"
	"valiant/internal/storage"
)

func main() {
	fmt.Println("Starting Valiant Backend...")

	// Initialize dependencies (using placeholders/defaults for now)
	// In a real app, we would load config and connect to DB here.
	store := storage.NewPostgresStorage(nil)
	metricClient := metrics.NewPrometheusClient("http://localhost:9090")
	engine := correlator.NewEngine(store, metricClient)
	router := api.NewRouter(store, engine)

	server := &http.Server{
		Addr:    ":8080",
		Handler: router.Handler(),
	}

	fmt.Printf("Server listening on %s\n", server.Addr)
	log.Fatal(server.ListenAndServe())
}

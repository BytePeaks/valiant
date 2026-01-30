package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"valiant/internal/api"
	"valiant/internal/config"
	"valiant/internal/correlator"
	"valiant/internal/metrics"
	"valiant/internal/storage"

	_ "github.com/lib/pq"
)

func main() {
	fmt.Println("Starting Valiant Backend...")

	// Load configuration
	cfg := config.Load()

	// Connect to database
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("Connected to PostgreSQL database")

	// Initialize dependencies
	store := storage.NewPostgresStorage(db)
	metricClient := metrics.NewPrometheusClient("http://localhost:9090")
	engine := correlator.NewEngine(store, metricClient)
	router := api.NewRouter(store, engine)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router.Handler(),
	}

	fmt.Printf("Server listening on %s\n", server.Addr)
	log.Fatal(server.ListenAndServe())
}

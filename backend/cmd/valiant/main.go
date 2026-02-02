package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"valiant/internal/api"
	"valiant/internal/collector"
	"valiant/internal/config"
	"valiant/internal/correlator"
	"valiant/internal/domain"
	"valiant/internal/metrics"
	"valiant/internal/storage"

	_ "github.com/lib/pq"
)

func main() {
	fmt.Println("Starting Valiant Backend...")

	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Printf("Warning: Failed to load config file: %v. Using defaults/env.", err)
	}

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

	// Run migrations
	if err := store.RunMigration("migrations/001_initial_schema.sql"); err != nil {
		log.Fatalf("Failed to run migrations (001): %v", err)
	}
	if err := store.RunMigration("migrations/002_add_impact_snapshots.sql"); err != nil {
		log.Fatalf("Failed to run migrations (002): %v", err)
	}
	if err := store.RunMigration("migrations/003_add_execution_fields.sql"); err != nil {
		log.Fatalf("Failed to run migrations (003): %v", err)
	}
	if err := store.RunMigration("migrations/005_add_service_preferences.sql"); err != nil {
		log.Fatalf("Failed to run migrations (005): %v", err)
	}
	fmt.Println("Database migrations applied")

	metricClient, err := metrics.NewPrometheusClient(cfg.Prometheus.URL, cfg.Prometheus.Queries, cfg.Prometheus.AdditionalMetrics)
	if err != nil {
		log.Fatalf("Failed to initialize prometheus client: %v", err)
	}

	engine := correlator.NewEngine(store, metricClient, cfg)
	router := api.NewRouter(store, engine, metricClient, cfg)

	// Setup application context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Background Worker (Automatic Analysis)
	worker := correlator.NewWorker(engine)
	go worker.Start(ctx)

	// Start Collectors
	eventChan := make(chan domain.ChangeEvent, 100)

	// Processor loop: Save events from channel
	go func() {
		for event := range eventChan {
			log.Printf("Received event: %s (%s)", event.Summary, event.ID)
			if err := store.SaveChangeEvent(ctx, event); err != nil {
				log.Printf("Failed to save event: %v", err)
			}
		}
	}()

	if cfg.Kubernetes.Enabled {
		k8sCollector, err := collector.NewKubernetesCollector(*cfg, nil)
		if err != nil {
			log.Printf("Failed to initialize Kubernetes collector: %v", err)
		} else {
			go func() {
				log.Println("Starting Kubernetes collector...")
				if err := k8sCollector.Start(ctx, eventChan); err != nil {
					log.Printf("Kubernetes collector stopped with error: %v", err)
				}
			}()
		}
	}

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router.Handler(),
	}

	fmt.Printf("Server listening on %s\n", server.Addr)
	log.Fatal(server.ListenAndServe())
}

package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"valiant/internal/api"
	"valiant/internal/collector"
	"valiant/internal/config"
	"valiant/internal/correlator"
	"valiant/internal/domain"
	"valiant/internal/metrics"
	"valiant/internal/retention"
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
	store := storage.NewPostgresStorage(db, cfg)

	// Run migrations
	migrationsPath := "migrations"

	// First, ensure the schema_migrations table exists
	if err := store.RunMigration(filepath.Join(migrationsPath, "000_create_schema_migrations_table.sql")); err != nil {
		log.Fatalf("Failed to run initial migration to create schema_migrations table: %v", err)
	}
	fmt.Println("Initial migration 000_create_schema_migrations_table.sql applied (if not already present)")

	files, err := os.ReadDir(migrationsPath)
	if err != nil {
		log.Fatalf("Failed to read migrations directory: %v", err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	for _, file := range files {
		if file.Name() == "000_create_schema_migrations_table.sql" {
			continue // Skip the initial migration, it's already handled
		}
		if !file.IsDir() {
			migrationFileName := file.Name()
			if strings.HasSuffix(migrationFileName, ".sql") {
				migrationPath := filepath.Join(migrationsPath, migrationFileName)
				if err := store.RunMigration(migrationPath); err != nil {
					log.Fatalf("Failed to run migration %s: %v", migrationFileName, err)
				}
				fmt.Printf("Migration %s applied\n", migrationFileName)
			}
		}
	}
	fmt.Println("Database migrations applied")

	metricClient, err := metrics.NewPrometheusClient(cfg.Prometheus.URL, cfg.Prometheus.Queries, cfg.Prometheus.AdditionalMetrics, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize prometheus client: %v", err)
	}

	engine := correlator.NewEngine(store, metricClient, cfg)
	router := api.NewRouter(store, engine, metricClient, cfg)

	// Setup application context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Background Worker (Automatic Analysis)
	worker := correlator.NewWorker(engine, cfg.Worker.PollingIntervalDur)
	go worker.Start(ctx)

	// Start Retention Worker
	if cfg.Retention.EventTTLDur > 0 {
		retentionWorker := retention.NewWorker(store, cfg.Retention.EventTTLDur)
		go retentionWorker.Start(ctx, cfg.Retention.CleanupIntervalDur)
	}

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

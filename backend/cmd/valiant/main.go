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
	"sync/atomic"
	"time"
	"valiant/internal/api"
	"valiant/internal/collector"
	"valiant/internal/config"
	"valiant/internal/correlator"
	"valiant/internal/discovery"
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

	var metricClient metrics.MetricsProvider
	prometheusURL := cfg.Prometheus.URL
	if prometheusURL == "" {
		discoverer, err := discovery.NewPrometheusDiscoverer(*cfg)
		if err != nil {
			log.Printf("WARN: Prometheus auto-discovery unavailable (no K8s client): %v", err)
		} else {
			discoverCtx, discoverCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer discoverCancel()
			if url, err := discoverer.Discover(discoverCtx); err != nil {
				log.Printf("WARN: Prometheus auto-discovery found no endpoint: %v — metrics disabled", err)
			} else {
				prometheusURL = url
			}
		}
	}
	if prometheusURL != "" {
		pc, err := metrics.NewPrometheusClient(prometheusURL, cfg.Prometheus.Queries, cfg.Prometheus.AdditionalMetrics, cfg)
		if err != nil {
			log.Printf("WARN: Failed to initialize Prometheus client: %v — metrics disabled", err)
			metricClient = &metrics.UnavailableMetricsProvider{}
		} else {
			log.Printf("INFO: Prometheus client initialized at %s", prometheusURL)
			metricClient = pc
		}
	} else {
		log.Printf("WARN: No Prometheus URL configured or discovered — metrics disabled")
		metricClient = &metrics.UnavailableMetricsProvider{}
	}

	engine := correlator.NewEngine(store, metricClient, cfg)

	// Setup application context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load persisted retention TTL from DB (overrides config.yaml if present)
	var retentionTTLNanos atomic.Int64
	retentionTTLNanos.Store(int64(cfg.Retention.EventTTLDur))
	if ttlStr, err := store.GetSetting(ctx, "retention_event_ttl"); err == nil && ttlStr != "" {
		if d, err := config.ParseDuration(ttlStr); err == nil {
			retentionTTLNanos.Store(int64(d))
			cfg.Retention.EventTTLDur = d
			cfg.Retention.EventTTL = ttlStr
		}
	}

	getTTL := func() time.Duration { return time.Duration(retentionTTLNanos.Load()) }
	router := api.NewRouter(store, engine, metricClient, cfg, &retentionTTLNanos)

	// Start Background Worker (Automatic Analysis)
	worker := correlator.NewWorker(engine, cfg.Worker.PollingIntervalDur)
	go worker.Start(ctx)

	// Start Retention Worker
	if cfg.Retention.EventTTLDur > 0 {
		retentionWorker := retention.NewWorker(store, getTTL)
		go retentionWorker.Start(ctx, cfg.Retention.CleanupIntervalDur)
	}

	// Start Collectors
	eventChan := make(chan domain.ChangeEvent, 100)

	// Processor loop: Save events from channel and eagerly link
	go func() {
		for event := range eventChan {
			log.Printf("Received event: %s (%s)", event.Summary, event.ID)
			if err := store.SaveChangeEvent(ctx, event); err != nil {
				log.Printf("Failed to save event: %v", err)
				continue
			}
			// Eager linking for K8s-collected execution events
			if event.IsExecution {
				links, err := engine.CreateIntentExecutionLinks(ctx, event)
				if err != nil {
					log.Printf("Warning: eager linking failed for event %s: %v", event.ID, err)
				} else if len(links) == 0 {
					log.Printf("Orphaned execution event %s: %s (no matching CI event found)", event.ID, event.Summary)
				}
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

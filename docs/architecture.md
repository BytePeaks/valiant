# Architecture

Valiant is a client-server application with a **Go backend**, **Next.js frontend**, **PostgreSQL** database, and **Prometheus** integration.

---

## System Overview

```
┌─────────────────┐     ┌──────────────────┐
│   Next.js UI    │────>│   Go Backend     │
│   (port 3000)   │<────│   (port 8080)    │
└─────────────────┘     └──────┬───┬───────┘
                               │   │
                    ┌──────────┘   └──────────┐
                    ▼                          ▼
             ┌─────────────┐          ┌──────────────┐
             │ PostgreSQL  │          │  Prometheus   │
             │   (5432)    │          │   (9090)      │
             └─────────────┘          └──────────────┘
                    ▲
                    │
         ┌─────────┴─────────┐
         │  K8s API Server   │
         │  (collector)      │
         └───────────────────┘
```

---

## Component Responsibilities

### Backend (Go)

The backend is a single Go binary with no external framework (stdlib `net/http` only). Entry point: `cmd/valiant/main.go`.

Responsibilities:
- Serves the REST API (CORS enabled)
- Runs the Kubernetes collector (watches for deployment rollouts)
- Runs the correlator engine (impact scoring)
- Runs background workers (automatic analysis, event retention cleanup)
- Manages PostgreSQL schema migrations at startup

### Frontend (Next.js)

Next.js 16 app directory structure with React 18, TypeScript (strict mode), and Tailwind CSS.

Responsibilities:
- Dashboard with timeline visualization of change events
- Service analytics with impact scores and metric breakdowns
- Filter/search events by service, namespace, date range
- Trigger manual analysis and view results
- Toggle metric display preferences per service

Communicates with the backend via `NEXT_PUBLIC_API_URL` (default `http://localhost:8080/api/v1`).

### PostgreSQL

PostgreSQL 16 serves as the single datastore. Migrations (`backend/migrations/`, files 000-008) are applied automatically at startup.

Stores:
- **Change events** - Normalized `ChangeEvent` records from all collectors (includes blast radius as JSONB)
- **Impact analysis snapshots** - Immutable analysis results
- **Event links** - Relationships between events (intent-execution links, config trigger links)
- **Service preferences** - Per-service metric display settings

### Prometheus

An external Prometheus instance that Valiant queries via the HTTP API (`/api/v1/query_range`). Valiant never writes to Prometheus - it only reads metric data during analysis.

When running in Kubernetes without an explicit `prometheus.url`, the backend runs a one-shot auto-discovery at startup: it lists Services matching `app.kubernetes.io/name=prometheus` or `app=prometheus`, scores candidates by port, name, namespace, and component labels, then validates the top candidates via `GET /api/v1/status/buildinfo`. If no endpoint is found, the backend starts in degraded mode with metrics disabled (HTTP 503 on metric endpoints).

---

## Backend Package Structure

```
backend/
├── cmd/valiant/main.go          # Entry point, wiring, startup
└── internal/
    ├── api/router.go            # HTTP routing, CORS, request handlers
    ├── collector/
    │   ├── kubernetes.go        # K8s deployment/configmap/secret watcher
    │   └── cicd.go              # CI/CD event handling
    ├── correlator/
    │   ├── engine.go            # Impact scoring, delta calculation, ranking
    │   └── worker.go            # Background auto-analysis worker
    ├── config/config.go         # YAML + env var configuration loading
    ├── domain/models.go         # Core types: ChangeEvent, ImpactAnalysis, RankedChange
    ├── discovery/prometheus.go  # Kubernetes-based Prometheus endpoint auto-discovery
    ├── metrics/
    │   ├── metrics.go           # MetricsProvider interface, UnavailableMetricsProvider sentinel
    │   └── prometheus.go        # Prometheus HTTP client, PromQL template execution
    └── storage/postgres.go      # PostgreSQL implementation of the Storage interface
```

---

## Data Flow

1. **Ingestion** - Collectors observe external systems and translate events into the `ChangeEvent` model:
   - **Kubernetes Collector**: Watches the K8s API for annotated Deployment rollouts, ConfigMap/Secret changes. Captures `rollout_start` and `rollout_end` timestamps.
   - **REST API**: Receives events via `POST /api/v1/events` from CI/CD pipelines or external tools.

2. **Storage** - Events are persisted in PostgreSQL with full metadata.

3. **Analysis Trigger** - Either:
   - A user clicks "Analyze" in the UI (or calls `POST /api/v1/analyze`)
   - The background worker (runs every 5 minutes) auto-triggers for events with closed impact windows

4. **Correlation** - The correlator engine:
   - Checks for an existing snapshot (returns cached if found)
   - Checks for intent-execution linking via 3-tier confidence ladder: `sha_match` (1.0), `image_tag_match` (0.9), `image_sha_inferred` (0.85). Unmatched executions are flagged as orphaned.
   - For rollout events: searches for recent ConfigMap/Secret changes that affected the same service (config trigger linking)
   - Fetches baseline and impact metrics from Prometheus
   - Calculates deltas, impact score, confidence score
   - Classifies impact level (NONE/LOW/MEDIUM/HIGH)

5. **Snapshotting** - Results are stored as an immutable `ImpactAnalysisSnapshot` in PostgreSQL.

6. **Visualization** - The frontend fetches events and snapshots via the REST API and renders:
   - Timeline view with event cards and analysis status badges
   - Service analytics with metric breakdowns and delta visualizations
   - Change rankings for incident investigation

---

## Design Trade-offs

### Prometheus HTTP API (not Remote Read)

Uses the standard Prometheus query API (`/api/v1/query_range`). Simpler to implement and requires no special Prometheus configuration. Trade-off: can put query load on Prometheus during analysis.

### No Kafka / Event Bus

Collectors write directly to the API, which stores data in PostgreSQL. Greatly simplifies the architecture. Trade-off: no buffering - if the backend is unavailable, events may be lost.

### Deterministic Heuristics (not ML)

All scoring logic is rule-based and fully explainable. Trade-off: may miss complex patterns that ML could detect, but avoids black-box scoring and training pipeline complexity.

### On-Demand Correlation (not Real-time)

Analysis is triggered by user request or background worker, not run continuously. The system consumes resources only when in use. Trade-off: no proactive incident detection in the OSS version.

### PostgreSQL as Single Datastore

One robust relational database for all data. Well-suited for discrete events (not time-series). Simple deployment and operations. Sufficient for single-cluster scale.

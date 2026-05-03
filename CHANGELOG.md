# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased] - v1.0.0

### Added
- **Markdown/URL rendering in event metadata fields** — event cards now display all metadata key-value pairs (excluding fields already rendered inline such as `git_sha` and `image_tag`). Values that are plain URLs are rendered as clickable links with an external-link icon. Values containing inline markdown (`**bold**`, `*italic*`, `` `code` ``, `[text](url)`) are rendered with basic formatting. The display appears on both standard `TimelineEvent` cards and `DeploymentStoryCard` cards (which merge intent + execution metadata, with execution values taking precedence on key conflicts).
- **Concurrent Group Visualization** — the main dashboard timeline now automatically detects and visually groups events that deployed within the same 30-minute window. When two or more events fall within this window, they are wrapped in a labeled bracket with an amber timeline-line highlight, a "N concurrent deployments" badge, the affected service names, and a `HH:MM–HH:MM` time span. Single-event slots render unchanged.
- **Event search by Git SHA and metadata** — the main dashboard filter panel now includes two new targeted search controls: a dedicated **Git SHA** field (prefix/partial match against `git_commit_sha` and `git_sha` metadata keys, case-insensitive) and a **Metadata Filter** key=value pair that performs an exact JSONB containment match on any metadata field. Both are wired to `GET /api/v1/events` via new `git_sha`, `metadata_key`, and `metadata_value` query parameters with corresponding storage-layer filter clauses.
- **Service Health Pulse** — traffic-light indicator per service derived from the latest completed impact analysis. A `GET /api/v1/services/health` endpoint returns an array of `ServiceHealth` objects (service name, status, impact level, score, last analyzed timestamp). Status maps as: `NONE`/`LOW` → `healthy` (green), `MEDIUM` → `warning` (amber, pulsing), `HIGH` → `degraded` (red, pulsing), no data → `unknown` (gray). The indicator appears as a small dot next to each service chip in the main dashboard filter panel and as a labeled badge in the service analytics page header.
- **Retention settings in UI** — event TTL is now configurable from the dashboard without editing `config.yaml`. A gear icon in the main header opens a modal with quick-select presets (7 d → 1 year) and a free-text field accepting the same `Nd`/`Nh` duration format as the config file. Settings are persisted to a new `system_settings` Postgres table (migration `010`) and take effect immediately — the retention worker reads the live TTL on every cleanup cycle via an `atomic.Int64`, so no restart is required. `GET /api/v1/settings/retention` returns the current TTL; `POST /api/v1/settings/retention` validates, persists, and hot-updates the running worker.

## [1.0.0-beta] - 2026-04-29

### Added
- **Prometheus auto-discovery** — Valiant now automatically locates a Prometheus endpoint when running in Kubernetes, so `prometheus.url` no longer needs to be set manually in-cluster. On startup the backend queries the Kubernetes Services API (both `app.kubernetes.io/name=prometheus` and `app=prometheus` label selectors across all reachable namespaces), scores each candidate by port number, port name, service name, namespace, and component labels, validates the top candidates against `/api/v1/status/buildinfo`, and promotes the highest-ranked responding endpoint. Explicit `prometheus.url` config (or `PROMETHEUS_URL` env var) still takes priority and bypasses discovery entirely. If no endpoint is found, the backend starts normally with metrics disabled (HTTP 503 on metric endpoints) rather than refusing to start. Scheme fallback (http↔https) is tried automatically on redirect or TLS responses. RBAC: the ClusterRole now includes `get`/`list` on `services` and `namespaces`.
- **Helm chart** (`deploy/helm/valiant/`) — install Valiant with `helm install valiant valiant/valiant`. The chart renders the full `config.yaml` from `values.yaml`, with every backend configuration field exposed as a Helm value. Supports optional bundled PostgreSQL (`postgresql.enabled`, default on), optional frontend Ingress (`ingress.enabled`), and cluster-scoped vs namespace-scoped RBAC (`rbac.clusterScoped`). A fail guard prevents misconfigured namespace-scoped RBAC. Post-install NOTES guide users to the UI and warn about default passwords.
- **Multi-metric UI toggle in service analytics** — The per-service analytics page (`/services/<name>`) now includes an inline **Metric Filters** panel above the change timeline. Each available metric (core and custom) is rendered as a toggleable chip; active metrics are highlighted in blue, inactive in gray. Clicking a chip toggles visibility immediately across all analysis cards on the page. A **Save Preferences** button persists the selection via the existing service-preferences API, with a brief "Saved!" confirmation. Page-level metric state is threaded as props through `Timeline` into `TimelineEvent` and `DeploymentStoryCard`, eliminating per-event preference fetches and keeping all cards in sync. The per-card "Customize" button is hidden on the service page since the panel serves that role; it remains available on the main dashboard timeline.
- **Deployment Story timeline** — Correlated events (intent + execution) are now rendered as a single unified `DeploymentStoryCard` on the dashboard timeline. The card presents a horizontal pipeline — `CI Build → Image Push → K8s Rollout → Impact Score` — with each stage showing its own icon, summary, and metadata (git SHA, image tag, timestamp). The Impact Score stage hosts the Analyze button inline; once triggered, the full metric-delta grid expands below. A chevron toggle reveals extended detail (timestamps, blast radius, contextual links). Events without a linked intent continue to render as standard `TimelineEvent` cards.
- **Analysis status badges in event timeline** — PENDING, READY, and ANALYZED status indicators now appear as header badges on each event card in the main timeline, making analysis state scannable at a glance without expanding cards or visiting detail pages.
- **Live image diffing on deployment events** — Deployment and StatefulSet rollout cards now display an `old → new` image tag diff inline, showing exactly what changed without needing to cross-reference CI events. The Kubernetes collector tracks the previous image tag per workload and stores it as `previous_image_tag` in event metadata; the frontend renders the diff in a compact monospace row on each affected card.
- **GitHub Actions example configs** — Three drop-in workflow snippets added under `example/github-actions/`: `ci-build-notify.yml` sends a CI intent event (`build_success`) after a successful build including `git_sha` and `image_tag` for linking; `deployment-notify.yml` sends a deployment execution event (`deployment_rollout`) with a precise `end_time` to anchor the impact window; `reusable-notify.yml` is a `workflow_call` reusable workflow that any pipeline can invoke with a single `uses:` reference for both event types.

## [1.0.0-alpha] - 2026-02-12

### Added
- Initialized Valiant project structure.
- **Backend (Go):**
  - Implemented **Orphan Event Detection**: Execution events (e.g., GitOps, manual) without a corresponding intent event (e.g., CI) within a configurable correlation window are now marked as `IsOrphaned`.
  - **Intent-Execution Linking**: Implemented backend logic to link CI (Intent) and GitOps/manual (Execution) events using `git_sha` or `image_tag` within a configurable `intent_execution_correlation_window`. This replaces the previous `IsOrphaned` check based solely on services.
  - Extended `PostgresStorage.GetChangeEvents` to support filtering by metadata content (`git_sha` or `image_tag`).
  - Renamed `OrphanCorrelationWindow` to `IntentExecutionCorrelationWindow` in configuration for clarity.
  - Extended `PostgresStorage.GetChangeEvents` to support filtering by trigger type, time range, and affected services.
  - Added `IsOrphaned` field to `domain.ImpactAnalysis` for API response.
  - Added `OrphanCorrelationWindow` configuration to `config.yaml`.
  - Project initialization with Go modules.
  - Core domain models for `ChangeEvent`, `MetricValues`, and `ImpactAnalysis`.
  - Defined interfaces for `Storage`, `MetricsProvider`, and `Collector`.
  - Skeleton implementations for PostgreSQL storage, Prometheus metrics, and Kubernetes, Git, and CI/CD collectors.
  - Core impact correlation engine logic with configurable weights and thresholds.
  - Custom weights of metrics (built-in and custom) applicable in config.yaml
  - **Timestamp Guardrail for ChangeEvent Ingestion**: Implemented a defensive mechanism to prevent `ChangeEvent`s with future timestamps from entering the analysis workflow.
    - Defined a `max_future_skew` of 2 minutes (hardcoded). Events with `Timestamp` or `EndTime` beyond this future limit are marked as invalid.
    - Invalid events are persisted with `status='invalid_time'`, `invalid_reason` (e.g., `timestamp_in_future`), and `skew_seconds` for auditability.
    - Structured warning logs are emitted for invalid events.
    - Analysis workers (`GetEventsPendingAnalysis`) now explicitly filter and skip events with `status != 'ready'`, preventing them from entering the analysis queue.
  - **Configurable Worker Polling Interval**: Made the analysis worker's polling frequency configurable via `config.yaml`.
    - Added a `worker.polling_interval` field to `config.yaml` (default "5m"), allowing users to adjust how often the worker checks for new events to process.
    - The backend worker now uses this configurable interval.
  - HTTP API server with health check, event submission, event listing, and impact analysis endpoints.
  - Initial database migration for the `change_events` table.
  - Environment-based configuration management.
  - Implemented `PostgresStorage` with actual SQL queries for saving and retrieving change events.
  - Added automatic execution of `001_initial_schema.sql` on backend startup.
  - Implemented CORS middleware to allow cross-origin requests from the frontend.
  - Updated `main.go` to connect to PostgreSQL using `lib/pq` and configuration.
  - Implemented `PrometheusClient` with actual PromQL queries for error rate, latency, RPS, and saturation metrics.
  - Basic homepage component.
  - API client layer for interacting with the backend.
- **Deep Linking Utility**:
  - Implemented dynamic generation of contextual links within the backend's event retrieval (specifically `PostgresStorage.GetEventsPendingAnalysis`) based on configurable `LinkTemplate`s.
  - Enhanced `PostgresStorage` to gracefully handle invalid URL templates and missing metadata keys, skipping link generation when `<no value>` is produced.
  - Expanded deep linking test suite (`tests/deep-linking`) with comprehensive edge cases, including empty `MetadataHas`, static URLs, special characters in metadata, and cases with partial or missing metadata.
  - Updated example seed scripts (`example/scripts/seed_data.ps1`, `example/scripts/seed_data.sh`) to generate events with varied metadata for deep linking, including valid, partially valid, and invalid scenarios, enabling thorough testing and demonstration.
- **Documentation:**
  - Added `HOW_TO_USE.md` with "Quick Start", "Core Concepts", and "For Dummies" guide on connecting apps.
  - Fixed missing Tailwind CSS configuration (`tailwind.config.ts`, `postcss.config.js`) and dependencies.
  - Moved styling dependencies to `dependencies` and updated Dockerfile to ensure `npm install` runs on startup, fixing volume sync issues.
  - Added `--legacy-peer-deps` to npm install commands to resolve React 19 peer dependency conflicts.
  - Implemented `GitCollector` to parse git tags and commits from a local repository.
  - Implemented immutable `ImpactAnalysis` snapshots. Analysis results are now stored in the database and reused to prevent historical drift.
  - Added support for `config.yaml` to configure analysis windows (baseline/impact durations), and now **customizable Prometheus query templates** with variable substitution (`{{ .Services }}`, `{{ .Duration }}`).
  - Implemented **Automatic Background Analysis**: A worker now periodically checks for events with expired impact windows and calculates/snapshots their impact without user intervention.
  - Fixed `fetchEvents` reference error in frontend by renaming handler to `fetchData`.
  - Improved data models in the frontend API layer.
  - **BREAKING:** Changed monitoring model to focus on execution boundaries (Trigger Type: CI, GitOps). Manual events are now deprecated.
  - Added `trigger_type`, `execution_id`, and `end_time` to ChangeEvent model and database schema.
  - Polished UI with new icons for triggers: `GitBranch` for GitOps, `Bot` for CI.
  - Updated seed scripts to exclude manual actions, reflecting the execution-only model.
  - Disabled `GitCollector` default behavior to align with the new model.
  - Updated seed scripts (`.sh` and `.ps1`) to generate 12 diverse events with varying timestamps and distinct affected services (fixing filtering issues).
  - Fully implemented `KubernetesCollector` using `client-go` with strict auditor logic. It now only emits events for completed rollouts (`Available=True`) that prove intent via `valiant.io/source` annotation.
  - Added support for `allowed_sources` configuration to trust only specific deployment systems (ArgoCD, Helm, CI/CD).
  - Refined K8s detection to capture precise `rollout_start` and `rollout_end` timestamps for impact anchoring.
  - Refactored `Collector` interface to be streaming/push-based (`Start(ctx, chan)`).
- **Frontend (Next.js):**
  - Next.js 16 project skeleton with TypeScript and React 19.
  - Root layout with Inter font and global CSS.
  - Implemented `Timeline` and `TimelineEvent` components.
  - Added on-demand impact analysis triggering from the UI.
  - Added detailed metric deltas breakdown in the analysis view with improved visual design (cards, colors).
  - Added "Show More" pagination logic to the timeline view.
  - Polished UI with `lucide-react` icons and improved layout for events and metrics.
  - Implemented dedicated Service Dashboard pages (`/services/[name]`) for deep-linking and focused analysis.
  - Enhanced Service Filter UI with focus links.
  - Added "Pending Analysis" badge for events where the impact window (35m) has not yet closed.
  - Fixed filtering logic in Service Pages to handle URL encoding correctly.
  - Refined Service Filter pills on homepage to embed the focus link inside the pill.
  - Enforced impact window validation in the backend: `AnalyzeImpact` now returns a specific error and "PENDING" status if called too early, preventing premature analysis.
  - Added tooltips to metric cards to explain what each metric represents.
  - Added "time ago" badge (e.g., "5m ago") next to event timestamps.
  - Added safe handling of null API responses in `fetchChangeEvents`.
  - Modal with PromQL queries and their weights for overall score.
- **Infrastructure:**
  - Dockerfiles for Backend and Frontend.
  - docker-compose.yml for full stack local development (Postgres, Backend, Frontend).
  - Added bash and PowerShell scripts for seeding mock data.


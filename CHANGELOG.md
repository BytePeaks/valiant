# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initialized Valiant project structure.
- **Backend (Go):**
  - Project initialization with Go modules.
  - Core domain models for `ChangeEvent`, `MetricValues`, and `ImpactAnalysis`.
  - Defined interfaces for `Storage`, `MetricsProvider`, and `Collector`.
  - Skeleton implementations for PostgreSQL storage, Prometheus metrics, and Kubernetes, Git, and CI/CD collectors.
  - Core impact correlation engine logic with configurable weights and thresholds.
  - HTTP API server with health check, event submission, event listing, and impact analysis endpoints.
  - Initial database migration for the `change_events` table.
  - Environment-based configuration management.
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
- **Infrastructure:**
  - Dockerfiles for Backend and Frontend.
  - docker-compose.yml for full stack local development (Postgres, Backend, Frontend).
  - Added bash and PowerShell scripts for seeding mock data.
- **Backend (Go):**
  - Implemented `PostgresStorage` with actual SQL queries for saving and retrieving change events.
  - Added automatic execution of `001_initial_schema.sql` on backend startup.
  - Implemented CORS middleware to allow cross-origin requests from the frontend.
  - Updated `main.go` to connect to PostgreSQL using `lib/pq` and configuration.
  - Implemented `PrometheusClient` with actual PromQL queries for error rate, latency, RPS, and saturation metrics.
  - Basic homepage component.
  - API client layer for interacting with the backend.

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
  - Added safe handling of null API responses in `fetchChangeEvents`.
  - Improved data models in the frontend API layer.
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

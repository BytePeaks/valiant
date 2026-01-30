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
  - Skeleton implementations for PostgreSQL storage, Prometheus metrics, and Kubernetes collection.
  - Core impact correlation engine logic skeleton.
  - HTTP API server with health check and basic event submission endpoint.
  - Initial database migration for the `change_events` table.
  - Environment-based configuration management.
- **Frontend (Next.js):**
  - Next.js 14 project skeleton with TypeScript.
  - Root layout with Inter font and global CSS.
  - Basic homepage component.
  - API client layer for interacting with the backend.

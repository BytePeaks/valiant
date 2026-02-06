# Kubernetes Collector Test Suite

Comprehensive test suite for the Valiant Kubernetes collector logic.

## Overview

Tests the logic for collecting change events from Kubernetes. This includes:
- Deployment rollout detection.
- ConfigMap and Secret change tracking.
- Annotation-based filtering.
- Metadata extraction.

## Test Structure

- `deployments/` - Tests for deployment-related events.
- `configmaps/` - Tests for ConfigMap-related events.
- `secrets/` - Tests for Secret-related events.
- `metadata/` - Tests for metadata extraction from K8s objects.
- `shared/` - Test fixtures and helpers.

## Running Tests

### Docker (Recommended)
```bash
./docker-test.sh
```
### Windows
```powershell
./docker-test.ps1
```

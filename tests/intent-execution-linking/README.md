# Intent-Execution Linking Test Suite

Comprehensive test suite for the Valiant event correlation logic.

## Overview

Tests the core logic for linking CI (Intent) events with GitOps/manual (Execution) events. This includes:
- Metadata matching (git commit SHA, image tag)
- Time window correlation
- Orphan detection

## Test Structure

- `matching/` - Tests for metadata matching logic.
- `time-windows/` - Tests for temporal correlation logic.
- `orphan-detection/` - Tests for orphan detection logic.
- `shared/` - Test fixtures, mocks, and assertions.
- `simulator/` - Event generation for complex scenarios.

## Running Tests

### Docker (Recommended)
```bash
./docker-test.sh
```
### Windows
```powershell
./docker-test.ps1
```

### Manual
```bash
export DB_HOST=localhost DB_PORT=5432 DB_USER=testuser DB_PASSWORD=testpass
go test ./... -v
```

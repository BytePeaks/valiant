# Correlator Engine Test Suite

Comprehensive test suite for the Valiant impact scoring and correlation engine.

## Overview

Tests the core correlation logic including:
- Impact score calculation (weighted algorithm)
- Impact level classification (NONE/LOW/MEDIUM/HIGH)
- Metric delta calculations
- Deterministic reproducibility

## Test Structure

- `scoring/` - Impact scoring algorithm tests
- `determinism/` - Reproducibility verification tests
- `metrics/` - Individual and combined metric tests
- `shared/` - Test fixtures, mocks, and assertions

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

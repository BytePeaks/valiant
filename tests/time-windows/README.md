# Time Window Calculation Test Suite

Comprehensive test suite for the Valiant time window calculation logic.

## Overview

Tests the logic for calculating baseline and impact analysis windows. This includes:
- Standard baseline window calculation.
- Impact window calculation with and without `end_time`.
- Window closure validation logic.

## Test Structure

- `baseline/` - Tests for baseline window calculation.
- `impact/` - Tests for impact window calculation.
- `window-closure/` - Tests for the impact window closure check.
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

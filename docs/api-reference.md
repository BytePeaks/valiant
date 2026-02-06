# API Reference

All endpoints are served by the Go backend on the configured port (default `8080`). CORS is enabled for all origins.

---

## Health Check

### `GET /health`

Returns `OK` with status `200` if the backend is running.

```bash
curl http://localhost:8080/health
# OK
```

---

## Change Events

### `POST /api/v1/events`

Submit a new change event (e.g., from a CI/CD pipeline).

**Request Body:**

```json
{
  "trigger_type": "CI",
  "change_type": "build_success",
  "affected_services": ["payment-service"],
  "summary": "Building payment-service v2.4.0",
  "timestamp": "2026-01-31T15:30:00Z",
  "end_time": "2026-01-31T15:32:00Z",
  "metadata": {
    "git_commit_sha": "a1b2c3d4e5f6",
    "image_tag": "v2.4.0"
  }
}
```

| Field | Type | Required | Description |
|:------|:-----|:---------|:------------|
| `trigger_type` | string | yes | `"CI"`, `"GitOps"`, or `"manual"` |
| `change_type` | string | yes | e.g., `"build_success"`, `"deployment_rollout"`, `"configmap_update"` |
| `affected_services` | string[] | yes | Services affected by this change |
| `summary` | string | no | Human-readable description |
| `timestamp` | string (RFC3339) | yes | When the change started |
| `end_time` | string (RFC3339) | no | When the change completed |
| `metadata` | object | no | Key-value pairs (e.g., `git_commit_sha`, `image_tag`) |

**Response:** `201 Created` (empty body)

**Errors:**
- `400 Bad Request` - Invalid JSON payload

---

### `GET /api/v1/events`

List change events with optional filtering and pagination.

**Query Parameters:**

| Parameter | Type | Description |
|:----------|:-----|:------------|
| `limit` | int | Max results to return (default 50, max 200) |
| `offset` | int | Number of results to skip |
| `service` | string | Filter by service name(s), comma-separated |
| `namespace` | string | Filter by Kubernetes namespace |
| `change_type` | string | Filter by change type |
| `from` | string (RFC3339) | Start of time range |
| `to` | string (RFC3339) | End of time range |
| `search` | string | Search in summary and metadata |

**Response:**

```json
{
  "events": [
    {
      "id": "evt-abc123",
      "trigger_type": "GitOps",
      "change_type": "deployment_rollout",
      "affected_services": ["payment-service"],
      "timestamp": "2026-01-31T15:30:00Z",
      "end_time": "2026-01-31T15:32:00Z",
      "metadata": { "git_commit_sha": "a1b2c3d" },
      "summary": "Rolled out payment-service v2.4.0",
      "analysis_status": "ready"
    }
  ],
  "total": 42,
  "limit": 50,
  "offset": 0
}
```

The `analysis_status` field indicates the event's analysis state:
- `"pending"` - Impact window hasn't closed yet
- `"ready"` - Window closed, no analysis performed yet
- `"completed"` - Analysis snapshot exists

---

## Impact Analysis

### `POST /api/v1/analyze`

Trigger impact analysis for a specific change event.

**Request Body:**

```json
{
  "event_id": "evt-abc123"
}
```

**Response (success):** `200 OK`

```json
{
  "change_event": { "..." },
  "is_orphaned": false,
  "baseline_metrics": {
    "error_rate": 0.02,
    "latency_p95_ms": 150.5,
    "rps": 1200.0,
    "cpu": 0.45,
    "memory": 512000000,
    "additional_metrics": { "orders_per_minute": 85.3 }
  },
  "impact_metrics": { "..." },
  "deltas": { "..." },
  "impact_score": 0.65,
  "impact_level": "MEDIUM",
  "confidence_score": 1.0
}
```

**Response (window not closed):** `422 Unprocessable Entity`

```json
{
  "change_event": { "..." },
  "impact_level": "PENDING"
}
```

This is returned when analysis is requested before the post-execution impact window has elapsed. Retry after the window closes (default 30 minutes after `rollout_end` + 5-minute buffer).

**Errors:**
- `400 Bad Request` - Invalid JSON payload
- `404 Not Found` - Event ID does not exist
- `500 Internal Server Error` - Analysis failed (Prometheus unreachable, database error, etc.)

---

## Services

### `GET /api/v1/services`

List all services that have at least one change event.

**Response:** `200 OK`

```json
["payment-service", "order-service", "user-service"]
```

---

## Namespaces

### `GET /api/v1/namespaces`

List all known namespaces (merged from database events and config).

**Response:** `200 OK`

```json
["default", "payment-app", "production"]
```

---

## Metrics

### `GET /api/v1/metrics`

List all available metrics (core + additional from config).

**Response:** `200 OK`

```json
[
  { "name": "error_rate" },
  { "name": "latency_p95_ms" },
  { "name": "rps" },
  { "name": "cpu" },
  { "name": "memory" },
  { "name": "orders_per_minute", "icon": "ShoppingCart" },
  { "name": "payment_failure_rate", "icon": "CreditCard" }
]
```

---

## Change Rankings

### `GET /api/v1/rankings`

Rank concurrent changes for a service by likelihood of causing degradation.

**Query Parameters:**

| Parameter | Type | Required | Description |
|:----------|:-----|:---------|:------------|
| `service` | string | yes | Service name to rank changes for |
| `from` | string (RFC3339) | yes | Start of time range |
| `to` | string (RFC3339) | yes | End of time range |

**Example:**

```bash
curl "http://localhost:8080/api/v1/rankings?service=payment-service&from=2026-01-31T14:00:00Z&to=2026-01-31T16:00:00Z"
```

**Response:** `200 OK`

```json
{
  "service": "payment-service",
  "from": "2026-01-31T14:00:00Z",
  "to": "2026-01-31T16:00:00Z",
  "ranked": [
    {
      "analysis": { "..." },
      "rank": 1,
      "likelihood_score": 0.78,
      "temporal_proximity": 0.9,
      "change_type_weight": 1.0,
      "service_scope": 1.0
    }
  ],
  "total": 3
}
```

**Errors:**
- `400 Bad Request` - Missing `service`, `from`, or `to` parameters, or invalid date format

---

## Service Metric Preferences

### `GET /api/v1/services/{name}/preferences`

Get the list of metric names enabled for display in the UI for a specific service.

**Response:** `200 OK`

```json
["error_rate", "latency_p95_ms", "orders_per_minute"]
```

### `POST /api/v1/services/{name}/preferences`

Set which metrics to display in the UI for a specific service.

**Request Body:**

```json
["error_rate", "latency_p95_ms", "rps", "orders_per_minute"]
```

**Response:** `200 OK` (empty body)

**Errors:**
- `400 Bad Request` - Invalid JSON payload

---

## Error Response Format

All error responses return a plain text error message in the response body with an appropriate HTTP status code.

Common status codes:
- `400` - Bad request (invalid input)
- `404` - Resource not found
- `405` - Method not allowed
- `422` - Unprocessable entity (semantic error, e.g., impact window not closed)
- `500` - Internal server error

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
| `metadata` | object | no | Arbitrary string key-value pairs. Well-known keys (`git_commit_sha`, `image_tag`) drive linking and diffing. All other keys are displayed in the UI — values that are plain `https://` URLs render as clickable links; values containing `**bold**`, `*italic*`, `` `code` ``, or `[text](url)` markdown syntax are rendered inline. |

**Response:** `201 Created` (empty body)

**Errors:**
- `400 Bad Request` - Invalid JSON payload, or `timestamp`/`end_time` are too far in the future.
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
      "metadata": {
        "git_commit_sha": "a1b2c3d",
        "repository_url": "https://github.com/my-org/my-repo"
      },
      "summary": "Rolled out payment-service v2.4.0",
      "status": "ready",
      "invalid_reason": "",
      "skew_seconds": 0,
      "contextual_links": [
        {
          "name": "View Commit on GitHub",
          "url": "https://github.com/my-org/my-repo/commit/a1b2c3d"
        }
      ]
    }
  ],
  "total": 42,
  "limit": 50,
  "offset": 0
}
```

The `status` field indicates the event's validity state and its eligibility for analysis:
- `"ready"` - Event is valid and eligible for analysis (or has already been analyzed).
- `"invalid_time"` - Event's `timestamp` or `end_time` was too far in the future at ingestion (more than 2 minutes from the backend's clock), preventing it from being analyzed. This event will be ignored by the analysis worker.

| Field | Type | Description |
|:------|:-----|:------------|
| `id` | string | Unique identifier for the event |
| `trigger_type` | string | `"CI"`, `"GitOps"`, or `"manual"` |
| `change_type` | string | e.g., `"deployment_rollout"`, `"build_success"` |
| `affected_services` | string[] | Services affected by this change |
| `summary` | string | Human-readable description |
| `timestamp` | string (RFC3339) | When the change started |
| `end_time` | string (RFC3339) | When the change completed (optional) |
| `metadata` | object | Key-value pairs |
| `status` | string | Indicates the event's validity state (`"ready"` or `"invalid_time"`) |
| `invalid_reason` | string | If `status` is `"invalid_time"`, provides the reason (e.g., `"timestamp_in_future"`) |
| `skew_seconds` | int | If `status` is `"invalid_time"`, indicates how many seconds into the future the timestamp was |
| `contextual_links` | object[] | Generated deep links to external systems |

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

**Query Parameters:**

| Parameter | Type | Description |
|:----------|:-----|:------------|
| `namespace` | string | Filter by Kubernetes namespace. When `kubernetes.namespaces` is configured, the namespace is validated against the config - unconfigured namespaces return an empty list. |
| `linked_only` | string | Set to `"true"` to return only services that have intent-execution links (`sha_match`, `image_tag_match`, or `image_sha_inferred`). |

**Response:** `200 OK`

```json
["payment-service", "order-service", "user-service"]
```

---

## Namespaces

### `GET /api/v1/namespaces`

List monitored namespaces. When `kubernetes.namespaces` is configured in `config.yaml`, returns exactly those namespaces (sorted). When unconfigured, falls back to discovering namespaces from stored events in the database.

**Response:** `200 OK`

```json
["default", "payment-app"]
```

---

## Metrics

### `GET /api/v1/metrics`

List all available metrics (core + additional from config).

**Response:** `200 OK`

```json
[
  {
    "name": "error_rate",
    "icon": "AlertCircle",
    "weight": 0.4,
    "type": "builtin",
    "description": "Rate of HTTP errors (status 5xx) for a service.",
    "promql": "avg_over_time(sum(rate(http_requests_total{service=~\"{{ .Services }}\",status=~\"5..\"}[1m]))[{{ .Duration }}])"
  },
  {
    "name": "orders_per_minute",
    "icon": "ShoppingCart",
    "weight": 0.1,
    "type": "custom",
    "description": "Custom metric: orders_per_minute",
    "query": "sum(rate(app_orders_total{service=~\"{{ .Services }}\"}[1m])) * 60"
  }
]

| Field | Type | Description |
|:------|:-----|:------------|
| `name` | string | Unique identifier for the metric |
| `icon` | string | Optional. Lucide icon name for UI display |
| `weight` | float | Normalized weight (0.0 - 1.0) contributing to the overall impact score |
| `type` | string | Metric type: `"builtin"` or `"custom"` |
| `description` | string | Human-readable description of the metric |
| `promql` | string | PromQL query template for built-in metrics |
| `query` | string | Actual PromQL query string for custom metrics |
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

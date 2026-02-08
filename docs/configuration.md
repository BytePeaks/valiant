# Configuration

Valiant is configured through a combination of **environment variables** and a **YAML config file**. Environment variables take precedence for the settings they cover.

---

## Environment Variables

| Variable | Description | Default |
|:---------|:------------|:--------|
| `DATABASE_URL` | PostgreSQL connection string (URI format) | `postgres://user:password@localhost:5432/valiant?sslmode=disable` |
| `PORT` | Backend HTTP server port | `8080` |
| `PROMETHEUS_URL` | Prometheus base URL | `http://localhost:9090` |

These can also be set in `config.yaml` - but the env var wins if both are present.

---

## Config File

The backend looks for `config.yaml` in the working directory. For Docker Compose, the example config is mounted automatically from `example/config.yaml`.

Below is the complete reference with all available settings:

### Top-Level Settings

```yaml
# PostgreSQL connection string (can be overridden by DATABASE_URL env var)
database_url: "postgres://valiant:valiant_password@postgres:5432/valiant?sslmode=disable"

# Backend HTTP server port (can be overridden by PORT env var)
port: "8080"
```

### Prometheus

```yaml
prometheus:
  # Prometheus server URL (can be overridden by PROMETHEUS_URL env var)
  url: "http://prometheus:9090"

  # Override default PromQL queries for core metrics.
  # Available template variables:
  #   {{ .Services }} - Regex-friendly service list (e.g., "svc-a|svc-b")
  #   {{ .Duration }} - Analysis window duration (e.g., "30m")
  queries:
    error_rate: 'avg_over_time(sum(rate(http_requests_total{service=~"{{ .Services }}",status=~"5.."}[1m]))[{{ .Duration }}])'
    latency_p95_ms: 'avg_over_time(histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket{service=~"{{ .Services }}"}[1m])))[{{ .Duration }}])'
    rps: 'avg_over_time(sum(rate(http_requests_total{service=~"{{ .Services }}"}[1m]))[{{ .Duration }}])'
    cpu: 'avg_over_time(sum(rate(container_cpu_usage_seconds_total{container=~"{{ .Services }}"}[1m]))[{{ .Duration }}])'
    memory: 'avg_over_time(sum(container_memory_usage_bytes{container=~"{{ .Services }}"})[{{ .Duration }}])'

  # Additional custom metrics collected alongside the 5 core metrics.
  # Each metric needs a name, a PromQL query, and an optional Lucide icon name.
  additional_metrics:
    - name: "orders_per_minute"
      query: |
        sum(rate(app_orders_total{service=~"{{ .Services }}"}[1m])) * 60
      icon: "ShoppingCart"
    - name: "payment_failure_rate"
      query: |
        sum(rate(app_payment_failures_total{service=~"{{ .Services }}"}[1m])) / sum(rate(app_payments_total{service=~"{{ .Services }}"}[1m]))
      icon: "CreditCard"
```

> **Important**: Use multiline YAML (`|`) for query values to avoid breaking YAML parsing with special characters.

### Kubernetes Collector

```yaml
kubernetes:
  # Enable/disable the automatic K8s event collector
  enabled: true

  # Path to kubeconfig for local development. Leave empty for in-cluster config.
  kube_config_path: ""

  # Namespaces to watch. Empty = watch ALL namespaces.
  # Cross-namespace visibility requires a ClusterRole.
  namespaces: ["default", "payment-app"]

  # Only record rollouts with the 'valiant.io/source' annotation
  require_annotation: true

  # Trusted values for the 'valiant.io/source' annotation
  allowed_sources: ["argocd", "helm", "cicd"]

  # Watch ConfigMaps for data changes.
  # Affected services are resolved by scanning Deployments that reference the ConfigMap.
  watch_configmaps: true

  # Watch Secrets for data changes.
  # ServiceAccountToken-type secrets are automatically skipped.
  # Secret data values and hashes are never exposed in event metadata.
  watch_secrets: true
```

### Retention

```yaml
retention:
  # How long to keep change events before automatic cleanup.
  # Supports "d" suffix for days (e.g., "90d") and standard Go durations (e.g., "2160h").
  event_ttl: "90d"

  # How often the retention worker runs to delete expired events.
  cleanup_interval: "1h"
```

### Analysis Windows

```yaml
analysis:
  # Duration to look back BEFORE rollout_start to establish a health baseline.
  baseline_window: "30m"

  # Duration to look forward AFTER rollout_end to measure stability impact.
  # This captures post-deploy issues like memory leaks, cache warmup, or slow crashes.
  post_execution_impact_window: "30m"

  # Duration to look back from an execution event to find a corresponding intent (CI) event.
  # If no match is found within this window, the execution event is marked as orphaned.
  intent_execution_correlation_window: "1h"
```

### Analysis Metrics Weights

```yaml
analysis:
  # Metric Weights
  # --------------
  # Define the relative importance of each metric in the impact score calculation.
  # The weights are normalized, so you can use any numbers you want (e.g., 1-100).
  # If a metric is not listed here, it will not be included in the score.
  #
  # The default built-in weights are applied automatically. Uncomment and modify
  # 'weights_built_in' if you need to override these defaults.
  weights_built_in:
    error_rate: 0.4
    latency_p95_ms: 0.3
    cpu: 0.1
    memory: 0.1
    rps: 0.1

  # Define weights for any custom metrics you have configured under `prometheus.additional_metrics`.
  # The name here must exactly match the `name` in the `additional_metrics` list.
  weights_custom:
    orders_per_minute: 0.1
    payment_failure_rate: 0.2
```

---

## Complete Example

See [`example/config.yaml`](../example/config.yaml) for a fully annotated configuration file ready for local development.

---

## Prometheus Query Templates

Valiant uses Go template syntax in PromQL queries. Two variables are available:

| Variable | Type | Description | Example Value |
|:---------|:-----|:------------|:--------------|
| `{{ .Services }}` | string | Pipe-separated regex of service names | `"payment-svc\|order-svc"` |
| `{{ .Duration }}` | string | Analysis window duration | `"30m"` |

### Tips for Custom Queries

- Ensure your Prometheus metrics have a label that matches your Kubernetes Deployment name (e.g., `service="payment-service"`)
- Use `rate()` with a `[1m]` range for counters to get per-second rates
- Wrap with `avg_over_time(...)[{{ .Duration }}])` to get the average over the analysis window
- Test queries in the Prometheus UI before adding them to config
- Inefficient PromQL can severely impact Prometheus performance - test with realistic cardinality

### Custom Metrics in the UI

Additional metrics defined in config are:
1. Collected during every analysis alongside the 5 core metrics
2. Stored in the immutable analysis snapshot
3. Shown in the UI on the service analytics page
4. Toggleable per-service via **metric display preferences** (`GET/POST /api/v1/services/{name}/preferences`)

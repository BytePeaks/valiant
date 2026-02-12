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
  # This window also provides implicit clock skew tolerance: if CI and cluster clocks
  # differ by less than this duration, linking still works. Increase if you observe
  # orphaned events caused by clock drift rather than missing CI signals.
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

### Linking Configuration

Valiant can automatically generate clickable deep links to external systems (like Git repositories, CI/CD pipelines, or deployment dashboards) based on metadata provided in change events. This enhances traceability by allowing users to quickly navigate to the source of a change.

The `linking` section in your `config.yaml` allows you to define templates for these links. Each template specifies the conditions under which a link should be generated and how its URL should be constructed.

```yaml
# Linking Configuration
# ---------------------
# Define templates to generate clickable deep links from event metadata.
# These links will appear in the UI for relevant ChangeEvents.
#
# Each template specifies:
# - name: Display name for the link in the UI.
# - metadata_has: A list of metadata keys that MUST be present in a ChangeEvent's
#                 metadata for this link template to be considered. This prevents
#                 broken or irrelevant links.
# - url_template: A Go template string to construct the URL. Variables correspond
#                 to keys in the ChangeEvent's metadata (e.g., {{ .git_commit_sha }}).
#                 If a required metadata key is missing, the link will not be generated.
linking:
  - name: "View Commit on GitHub"
    metadata_has: ["repository_url", "git_commit_sha"]
    url_template: "{{ .repository_url }}/commit/{{ .git_commit_sha }}"

  - name: "View Build on Jenkins"
    metadata_has: ["jenkins_url", "jenkins_job_name", "jenkins_build_id"]
    url_template: "{{ .jenkins_url }}/job/{{ .jenkins_job_name }}/{{ .jenkins_build_id }}"

  - name: "Open ArgoCD Application"
    metadata_has: ["argocd_url", "argocd_app_name"]
    url_template: "{{ .argocd_url }}/applications/{{ .argocd_app_name }}"
```

### Worker

```yaml
worker:
  # How often the analysis worker checks for new events to process.
  # Supports standard Go durations (e.g., "1m", "30s", "1h").
  polling_interval: "5m"
```

**Template Variables:**

The `url_template` uses Go's `text/template` syntax. You can reference any key from the `ChangeEvent`'s `metadata` map using the `{{ .keyName }}` notation. For example, if your event metadata contains `{"git_commit_sha": "abcdef", "repository_url": "https://github.com/my-org/my-repo"}`, the template `{{ .repository_url }}/commit/{{ .git_commit_sha }}` would resolve to `https://github.com/my-org/my-repo/commit/abcdef`.

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

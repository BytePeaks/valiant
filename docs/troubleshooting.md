# Troubleshooting

Common issues, error explanations, performance characteristics, and security considerations.

---

## Common Errors

### `ErrImpactWindowNotClosed` (HTTP 422)

**What it means**: You requested analysis for an event whose post-execution impact window hasn't elapsed yet.

**Why it happens**: Valiant needs to wait for the full impact window (default 30 minutes after `rollout_end` + 5-minute buffer) to collect enough metric data for a meaningful comparison.

**What to do**: Wait for the window to close. The UI shows a "Pending" badge on events that aren't ready yet. Alternatively, the background worker will automatically analyze it once the window closes.

### Database Connection Errors

```
database connection failed: dial tcp 127.0.0.1:5432: connect: connection refused
```

**Check**:
- Is PostgreSQL running? (`docker-compose up -d postgres`)
- Is the `DATABASE_URL` environment variable correct?
- Is the database `valiant` created?
- Can you connect manually? `psql "postgres://valiant:valiant_password@localhost:5432/valiant?sslmode=disable"`

### Prometheus Query Failures

```
Analysis failed: prometheus query error
```

**Check**:
- Is Prometheus reachable at the configured URL?
- Do your PromQL queries reference labels that exist in your Prometheus data?
- Does the `service` label in Prometheus match your Kubernetes Deployment name?
- Test queries directly in the Prometheus UI (`/graph`) before using them in config

### Kubernetes Collector Not Recording Events

**Check**:
- Is `kubernetes.enabled: true` in config?
- Does the Deployment have the `valiant.io/source` annotation?
- Is the annotation value in the `allowed_sources` list?
- Does the backend's ServiceAccount have permission to watch Deployments, ReplicaSets, ConfigMaps, and Secrets in the target namespaces?
- For cross-namespace visibility, a `ClusterRole` and `ClusterRoleBinding` are required

### Events Show as "Orphaned"

An event is marked orphaned when Valiant can't find a matching CI (intent) event within the correlation window across all three linking tiers (`sha_match`, `image_tag_match`, `image_sha_inferred`).

**This happens when**:
- No CI event was sent with a matching `git_commit_sha` or `image_tag`
- The CI event was sent more than `intent_execution_correlation_window` (default 1h) before the deployment
- The execution event has no `git_commit_sha` or `image_tag` metadata at all
- The image tag doesn't contain the commit SHA (for `image_sha_inferred` fallback)

**To fix**: Ensure your CI pipeline sends an event to `POST /api/v1/events` with at least one of:
- `metadata.image_tag` matching the container image (zero-config, 0.9 confidence)
- `metadata.git_commit_sha` matching a `valiant.io/git-sha` annotation (1.0 confidence)
- `metadata.git_commit_sha` where the image tag contains the SHA prefix (0.85 confidence)

### Events Linked to the Wrong CI Build

**What it means**: An execution event is correlated with a CI build that didn't actually produce the deployed artifact.

**Why it happens**: This typically occurs when using **mutable image tags** (`:latest`, `:staging`, `:dev`). Multiple CI builds produce different images under the same tag, so Valiant's `image_tag_match` (0.9 confidence) links to whichever CI event has the matching tag - which may be the wrong build.

**To fix**:
- Switch to **immutable tags** (semver, commit SHA, build number) for production deployments
- Set the `valiant.io/git-sha` annotation on Deployments so Valiant can use `sha_match` (1.0 confidence) instead of relying on tag matching
- Check the `link_reason` field in the API response to verify which metadata was used for linking

### Clock Skew Between CI and Cluster

**What it means**: CI events and K8s execution events have mismatched timestamps, causing linking failures or `invalid_time` rejections.

**How Valiant handles clock skew**:
- Timestamps more than **2 minutes in the future** are flagged as `invalid_time` at ingestion (the event is stored but excluded from analysis)
- The default **1-hour correlation window** (`intent_execution_correlation_window`) provides implicit tolerance for clock drift between CI systems and the Kubernetes cluster
- If CI and cluster clocks differ by less than 1 hour, linking still works correctly

**To fix**:
- Ensure NTP is configured on both CI runners and cluster nodes
- If drift exceeds 1 hour, increase `intent_execution_correlation_window` in `config.yaml`
- Check `skew_seconds` in the API response for events marked `invalid_time`

---

## Performance

### Correlator Engine Complexity

The impact scoring logic has complexity of **O(N * M)**, where:
- **N** = number of change events in the analysis timeframe
- **M** = number of Prometheus metric queries per event (5 core + additional metrics)

For typical usage (single service, handful of events), this completes in seconds.

### Network-Bound Operations

The primary bottleneck is **Prometheus query latency**. Each analysis requires multiple Prometheus HTTP API calls (one per metric per window). Performance depends on:
- Prometheus server load and query speed
- Network latency between Valiant and Prometheus
- Cardinality of the queried metrics
- Time range of the analysis windows

### Database Performance

PostgreSQL handles the event storage workload well for single-cluster deployments. As event volume grows:
- The retention policy (default `90d`) keeps the database lean
- Pagination on `GET /api/v1/events` limits query size (max 200 per page)
- Analysis snapshots are immutable and cached - repeat lookups don't recompute

---

## Known Limitations

- **Single-cluster focus** - The OSS version monitors one Kubernetes cluster and one Prometheus instance
- **On-demand analysis** - No continuous real-time monitoring or proactive alerting (Pro feature)
- **Deterministic only** - Rule-based scoring cannot detect previously unknown patterns
- **No event buffering** - If the backend is down, events from collectors may be lost
- **No native Git collector** - Git metadata is captured via CI/CD webhook events, not by watching Git repositories directly

---

## Security Considerations

### No Built-in Authentication

The OSS version has **no authentication or authorization**. Deploy Valiant within a trusted network boundary. For production, place it behind an API gateway or ingress controller with authentication.

### Prometheus Access

Valiant queries Prometheus directly. Ensure:
- Valiant's access to Prometheus is limited to query permissions only
- No sensitive metric data is inadvertently exposed through the Valiant API
- Prometheus is not publicly accessible through Valiant

### API Exposure

If exposing Valiant externally:
- Add an authentication proxy (e.g., OAuth2 Proxy)
- Enable rate limiting at the ingress/gateway level
- Restrict `POST` endpoints to authorized sources only

### Metadata Sensitivity

Change event metadata (`metadata` field) is stored as-is and returned in API responses. Avoid sending sensitive values (credentials, tokens, secrets) in event metadata. Secret data values and hashes from ConfigMap/Secret watchers are never exposed in event metadata.

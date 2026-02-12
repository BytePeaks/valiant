# How Valiant Works

This document explains Valiant's core concepts, analysis model, and scoring engine.

---

## Intent vs Execution

Valiant strictly separates **Intent** from **Execution**:

- **Intent (CI/Git)** - A code commit, merge, or CI build. These signals record that *someone intends to change something*, but the running system hasn't changed yet. Intent events carry metadata (Git SHA, image tag) used for linking.
- **Execution (Boundary)** - When a system (ArgoCD, Helm, CI/CD pipeline, or manual action) actually applies a change to the cluster. **Stability monitoring only starts here.**

This separation matters because a merged PR doesn't affect production until it's deployed. Valiant watches the execution boundary - the moment the change becomes live.

```
Intent (CI Build) ──────────────> Execution (K8s Rollout) ──────> Impact Analysis
       metadata: git_commit_sha=a1b2c3d ──links to──> metadata: git_commit_sha=a1b2c3d
```

---

## The Auditor Model

Valiant acts as a strict auditor. It only records a change if two conditions are met:

1. **Intentional** - The Kubernetes Deployment must have the `valiant.io/source` annotation (e.g., `argocd`, `helm`, `cicd`). This filters out accidental `kubectl edit` changes.
2. **Completed** - The rollout must be 100% finished and the Deployment must be `Available`.

### Execution Fingerprinting

Static annotations alone don't prove a change was automated. To distinguish automated deploys from manual edits, your CI/CD tool should set a **unique fingerprint** on each change:

- **Automated sync**: CI updates `valiant.io/git-sha`. Valiant sees a new SHA and records the event.
- **Manual edit**: A human edits a CPU limit. The SHA remains the same. Valiant sees no fingerprint change and **ignores** the rollout.

---

## Impact Analysis Windows

When analysis is triggered (manually via UI/API, or automatically by the background worker), Valiant uses two time windows anchored to the change event:

```
 ◄──── Baseline Window ────►         ◄──── Impact Window ────►
       (default 30m)                       (default 30m)
 ┌────────────────────────────┐      ┌─────────────────────────┐
 │   Health BEFORE change     │      │  Health AFTER change    │
 └────────────────────────────┘      └─────────────────────────┘
                              ▲      ▲
                      rollout_start  rollout_end + 5m buffer
```

- **Baseline Window**: Measures service health *before* the rollout started. Duration is configurable (default `30m`), ending 5 minutes before `rollout_start`.
- **Impact Window**: Measures service health *after* the rollout finished. Starts 5 minutes after `rollout_end` (buffer for stabilization) and runs for the configured duration (default `30m`).

### Window Enforcement

Analysis is **rejected** with `ErrImpactWindowNotClosed` until the impact window has fully elapsed. This prevents incomplete data from producing misleading scores. The UI shows a "Pending" badge for events whose window hasn't closed yet.

---

## Scoring Engine

The correlator engine compares baseline metrics against impact metrics to produce a deterministic impact score.

### Metrics Collected

| Metric | What It Measures | Why It Matters |
|:-------|:-----------------|:---------------|
| **Error Rate** | 5xx response rate | Direct signal of crashes or broken logic |
| **Latency P95** | 95th percentile response time | User experience degradation |
| **RPS** | Requests per second | Traffic flow - drops may mean users can't reach the service |
| **CPU** | CPU utilization | Resource efficiency - spikes indicate regressions |
| **Memory** | Memory utilization | Resource efficiency - leaks or bloat |

### Scoring Process

1. **Calculate deltas** - Percentage change from baseline to impact for each metric: `(impact - baseline) / baseline`
2. **Normalize** - Each delta is normalized to a 0-1 range (capped). For error rate, latency, CPU, and memory, increases are bad. For RPS, decreases are bad.
3. **Weighted sum** - Valiant applies configurable weights to the normalized deltas to produce a composite impact score. These weights are defined in `config.yaml` under the `analysis` section, allowing users to customize the relative importance of each built-in and custom metric. The system automatically normalizes these weights so that their sum is 1.0. Setting a metric's weight to `0` effectively removes its contribution to the overall impact score.

#### Built-in metrics weight (default)
| Metric | Weight |
|:-------|:-------|
| Error Rate | 0.4 |
| Latency P95 | 0.3 |
| CPU | 0.1 |
| Memory | 0.1 |
| RPS | 0.1 |

4. **Classify impact level**:

| Score Range | Level |
|:------------|:------|
| >= 0.7 | **HIGH** |
| >= 0.4 | **MEDIUM** |
| >= 0.1 | **LOW** |
| < 0.1 | **NONE** |

### Confidence Score

A secondary score (0.0-1.0) indicates how statistically reliable the analysis is:

- **1.0** - Sufficient traffic volume for reliable percentage deltas
- **0.5** - Low traffic (both baseline and impact RPS < 1.0), deltas may not be meaningful
- **0.1** - No traffic at all across both windows

---

## Intent-Execution Linking

When Valiant encounters an **Execution** event (e.g., a K8s deployment), it looks backward within a configurable correlation window (default `1h`) for a matching **Intent** event (e.g., a CI build) using shared metadata:

- `git_commit_sha` - The same Git SHA appears in both the CI build and the K8s rollout
- `image_tag` - The same container image tag links the build to the deployment

If no matching intent event is found, the execution is marked as **orphaned** - meaning it happened without a corresponding CI signal. This helps identify unexpected or undocumented changes.

---

## Config Trigger Linking

Beyond Intent-Execution linking (CI build → K8s rollout), Valiant can detect when a **ConfigMap or Secret change preceded a deployment rollout** and link them as a potential trigger.

### How It Works

The linking direction is **reverse**: when a `deployment_rollout` or `statefulset_rollout` event occurs, the correlator looks **backwards in time** for recent config changes that affected the same service.

```
        ◄── Config Trigger Window (default 15m) ──►
        ┌──────────────────────────────────────────┐
        │  ConfigMap update (affected: api-gateway) │
        └──────────────────────┬───────────────────┘
                               │ within window?
                               ▼
                     Deployment rollout (api-gateway)
                               │
                               ▼
                     EventLink { type: "config_trigger", confidence: 0.83 }
```

1. **Detection**: The K8s collector detects ConfigMap/Secret data changes via SHA256 hash comparison on each poll cycle.
2. **Affected services discovery**: The collector scans all Deployments and StatefulSets to find workloads that reference the changed resource via:
   - `envFrom.configMapRef` / `envFrom.secretRef`
   - `env[].valueFrom.configMapKeyRef` / `secretKeyRef`
   - `volumes[].configMap` / `volumes[].secret`
3. **Linking on rollout**: When a rollout completes, `CreateConfigTriggerLinks()` queries for config changes that:
   - Have the rollout's service in their `affected_services` array
   - Occurred within the `config_trigger_dur` window (default 15m) before the rollout timestamp
4. **Confidence**: Ranges from **0.7** (at the edge of the window) to **0.9** (immediately before the rollout), based on temporal proximity.

### Known Limitation: Hot-Reload Blind Spot

Config trigger linking only fires when a rollout follows a config change. If your application **hot-reloads** a mounted ConfigMap without triggering a pod restart (e.g., Spring Cloud Config, Envoy xDS, file-watcher patterns), there is no rollout event to anchor the search. The config change event is stored with `affected_services` populated, but the correlator has no execution event to link it to.

**Impact**: A ConfigMap change that degrades a hot-reloading service will appear in the timeline but won't automatically get an impact analysis unless manually triggered via `POST /api/v1/analyze`.

---

## Blast Radius

For ConfigMap and Secret changes, Valiant computes a **blast radius** - the set of workloads that consume the changed resource.

```json
{
  "total_workloads": 3,
  "affected_deployments": ["api-gateway", "auth-service"],
  "affected_statefulsets": ["redis-cluster"]
}
```

### Computation

At collection time, the K8s collector discovers referencing workloads (same pod spec scan used for config trigger linking) and stores the blast radius on the event. This is:
- **Nil** when no workloads reference the config resource (orphaned config)
- **Populated** with the count and names of all Deployments + StatefulSets that mount or env-ref the resource

### Usage

Blast radius is stored as JSONB in the `change_events` table and copied to the `ImpactAnalysis` snapshot. It is **informational metadata** for user investigation - it does not directly influence the impact score or ranking algorithm. The value is in answering: *"How many services could this config change have broken?"*

---

## Change Ranking

When multiple changes occur close together (common during incidents), Valiant ranks them by **likelihood of causing degradation**. The ranking uses four factors:

| Factor | Weight | Description |
|:-------|:-------|:------------|
| Impact Score | 0.5 | Higher impact = more likely cause |
| Temporal Proximity | 0.2 | Changes closer to the incident midpoint rank higher |
| Change Type | 0.2 | Deployment rollouts (1.0) > builds (0.8) > config changes (0.5) |
| Service Scope | 0.1 | Direct service matches (1.0) rank above indirect (0.5) |

The composite likelihood score is calculated as:

```
likelihood = (impact_score * 0.5) + (temporal * 0.2) + (change_type * 0.2) + (scope * 0.1)
```

Results are sorted by likelihood descending and assigned a rank (1 = most likely cause).

---

## Immutable Snapshots

Once an analysis is performed, the results are stored as an **immutable snapshot** in PostgreSQL. This means:

- An analysis performed today will yield the **exact same result** if queried a year from now
- Future deployments or metric changes do not retroactively alter past analyses
- Each snapshot includes: baseline metrics, impact metrics, deltas, impact score, impact level, and confidence score

If an analysis already exists for an event, the engine returns the cached snapshot rather than recomputing.

---

## Contextual Deep Linking

Valiant's contextual deep linking feature enhances traceability by allowing you to navigate directly from a change event in the UI to its original source in external systems, such as Git repositories, CI/CD pipelines, or deployment dashboards. This is achieved by generating clickable URLs based on metadata you provide in change events and templates defined in your `config.yaml`.

### How it Works

1.  **Metadata from Event Payload:** When you send a `ChangeEvent` to Valiant's API (e.g., via `curl`), you include relevant key-value pairs in the `metadata` field (e.g., `git_commit_sha`, `repository_url`, `jenkins_build_id`, `argocd_app_name`).
2.  **Configured Link Templates:** In your `config.yaml`, you define `linking` templates. Each template specifies:
    *   **`name`**: The text displayed for the link in the UI.
    *   **`metadata_has`**: A list of metadata keys that *must* be present in the event's metadata for this specific link to be generated. This ensures only valid, actionable links are created.
    *   **`url_template`**: A Go template string that uses the event's metadata to construct the full URL (e.g., `{{ .repository_url }}/commit/{{ .git_commit_sha }}`).
3.  **Backend Generation:** The Valiant backend processes incoming events. For each event, it attempts to match its metadata against your configured link templates. If a template's `metadata_has` conditions are met, the `url_template` is executed, and a clickable link is generated.
4.  **Frontend Display:** The generated links are included in the API response and displayed in the Valiant UI as prominent, pill-style buttons within the event details.

### Enabling Deep Links for Your System

To utilize this feature, you need to:

1.  **Configure Link Templates:** Add a `linking` section to your `config.yaml` with templates appropriate for your external systems (e.g., GitHub, GitLab, Jenkins, ArgoCD). See the [Configuration](configuration.md) documentation for detailed examples.
2.  **Send Rich Metadata:** Ensure your CI/CD pipelines or scripts include the necessary metadata keys (as required by your templates) in the `metadata` field of the `ChangeEvent` payload when calling Valiant's `POST /api/v1/events` endpoint.

This flexible approach allows you to integrate Valiant with any system where you can extract relevant URL components into event metadata, without requiring Valiant to directly integrate with or monitor those systems.

---

## Custom Metrics

Beyond the five core metrics, you can define **additional metrics** in `config.yaml` using custom PromQL queries:

```yaml
prometheus:
  additional_metrics:
    - name: "orders_per_minute"
      query: |
        sum(rate(app_orders_total{service=~"{{ .Services }}"}[1m])) * 60
      icon: "ShoppingCart"
```

Custom metrics are:
- Collected during every analysis alongside core metrics
- Stored in the immutable snapshot
- Toggleable per-service in the UI via metric display preferences

See [Configuration](configuration.md) for the full reference.

---

## Automatic Analysis

A background worker runs every 5 minutes and automatically triggers analysis for events whose impact window has closed but haven't been analyzed yet. This means you don't have to manually trigger analysis for every event - the worker handles it.

Manual analysis via the UI or API is still available for on-demand investigation.

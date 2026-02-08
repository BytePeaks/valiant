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
       metadata: git_sha=a1b2c3d ──links to──> metadata: git_sha=a1b2c3d
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

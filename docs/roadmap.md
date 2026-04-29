# Roadmap

> **Current release:** `v1.0.0-alpha`

---

## Shipped - v1.0.0-alpha

All features available in the current release.

- **Kubernetes Collector** - Watches Deployments, ReplicaSets, ConfigMaps, and Secrets with annotation-based filtering
- **Execution Fingerprinting** - Ignores rollouts where the generation changed but the fingerprint (git SHA) remained identical, filtering out manual `kubectl edit` noise
- **Orphan Detection** - Highlights execution events with no matching intent (CI) event
- **Deterministic Impact Scoring** - Baseline vs impact window comparison with configurable weighted scoring for built-in and custom metrics
- **Immutable Analysis Snapshots** - Results stored permanently, never retroactively altered
- **Confidence Scoring** - Secondary score based on traffic volume significance
- **Change Ranking** - Ranks concurrent changes by likelihood of causing degradation
- **Customizable PromQL Templates** - Core queries configurable via `config.yaml` with `{{ .Services }}` and `{{ .Duration }}` variables
- **Additional Custom Metrics** - User-defined PromQL queries in config, collected alongside core metrics
- **Automatic Background Analysis** - Worker triggers analysis when impact windows close; polling interval configurable
- **Event Retention Policy** - Configurable TTL with automatic cleanup
- **Intent-Execution Linking** - Links CI events to K8s rollouts via a 3-tier confidence ladder: `sha_match` (1.0), `image_tag_match` (0.9, zero-config), `image_sha_inferred` (0.85). All links include explainable `reason` metadata.
- **Timestamp Guardrail** - Prevents `ChangeEvent`s with future timestamps from entering the analysis workflow; marks them invalid for auditability
- **Contextual Deep Linking** - Dynamic clickable links to external systems (Git, CI/CD) generated from event metadata via configurable `LinkTemplate`s
- **Full UI** - Dashboard timeline, service analytics, metric preferences, search, namespace/type filters

---

## v1.0.0-beta

**Theme: Complete the Foundation**
**Status: In progress**

| # | Item | Status |
|---|------|--------|
| 1 | Analysis status badges in event timeline | ✅ Done |
| 2 | Live image diffing on deployment events | ✅ Done |
| 3 | Deployment Story timeline - intent + execution as a single unified card (`CI Build → Image Push → K8s Rollout → Impact Score`) | ✅ Done |
| 4 | Multi-metric UI toggle in service analytics | ✅ Done |
| 5 | **Helm chart** - `helm install valiant valiant/valiant`, listed on Artifact Hub on release | ✅ Done |
| 6 | GitHub Actions example configs - drop-in workflow snippets for CI/CD event injection | ✅ Done |

---

## v1.0.0

**Theme: General Availability**
**Status: Planned**

Production-ready. OpenShift-capable. The version teams can confidently run in a real cluster.

**Cluster compatibility**
- OpenShift SecurityContextConstraints - full support for restricted SCC environments
- OpenShift RBAC manifests - pre-built Role and RoleBinding manifests alongside standard Kubernetes ones

**UI & operations**
- Service Health Pulse - traffic-light indicator per service derived from last known impact score
- Concurrent Group Visualization - highlight services that deployed within the same time window
- Event search by Git SHA and arbitrary metadata values
- Markdown/URL rendering in event metadata fields
- Retention settings exposed in the UI (no more editing `config.yaml` for TTL)

**Onboarding**
- Demo seed environment - realistic multi-service deployment story out of the box; populated dashboard in under five minutes

---

## v1.1.0

**Theme: Deeper Insights**
**Status: Planned**

Make Valiant stickier for teams using it daily.

- Manual verification tags on snapshots - annotate results with `confirmed root cause`, `false positive`, or `rolled back`
- Blast radius visualization - UI surface for ConfigMap/Secret → affected workloads (data already exists in the API)
- Per-service configurable impact thresholds - LOW/MEDIUM/HIGH tunable per service
- Refined likelihood scoring for ConfigMap and Secret changes vs. image rollouts
- Filtered event export as JSON - for incident reports and external tooling

---

## v1.2.0

**Theme: Ecosystem Integrations**
**Status: Planned**

Connect Valiant to tools teams already use. This is the discovery milestone - Grafana plugin registry, Slack, ArgoCD, and GitHub put Valiant in front of new users without any marketing effort.

**Observability**
- Grafana annotation injection - automatic deploy markers on all existing dashboards
- Grafana panel plugin - listed in the official plugin registry

**Notifications**
- Slack and generic webhook notifications on MEDIUM/HIGH impact results

**Event sources**
- Git Collector - direct collection of tags, releases, and merges from Git repositories
- GitHub webhook receiver - push, tag, and release events with structured metadata
- GitLab webhook receiver
- ArgoCD notifications integration - covers GitOps clusters with no CI pipeline

---

## v2.0.0

**Theme: Proactive Awareness**
**Status: Vision**

Shift from "what happened after my deploy" to "something looks wrong right now."

- Live rollout health scoring - real-time metric trend updated as the impact window fills, not only at the end
- Rollback recommendation - when HIGH impact is detected, surface affected metrics and a direct link to the rollback action in ArgoCD (human decision only, full context surfaced immediately)
- Alertmanager enrichment - Valiant enriches Alertmanager alerts with recent changes to the affected service
- Pre-deployment risk estimation - based on historical snapshots per service, estimate risk profile before a new version ships

---

## Out of Scope

Keeping this honest. Valiant is a focused, single-cluster change impact tool.

- Multi-cluster and multi-environment management
- Authentication / SSO for the Valiant UI itself
- Distributed event bus (Kafka, NATS)
- Service mesh dependency and proximity scoring
- Full postmortem workflow and incident management

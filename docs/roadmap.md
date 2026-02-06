# Roadmap

Current status of features and planned work for Valiant.

---

## Completed

- **Kubernetes Collector** - Watches Deployments, ReplicaSets, ConfigMaps, and Secrets with annotation-based filtering
- **Execution Fingerprinting** - Ignores rollouts where the generation changed but the fingerprint (git-sha) remained identical, filtering out manual `kubectl edit` actions
- **Orphan Detection** - Highlights execution events with no matching intent (CI) event
- **Deterministic Impact Scoring** - Baseline vs impact window comparison with weighted scoring (error_rate=0.4, latency=0.3, cpu=0.1, memory=0.1, rps=0.1)
- **Immutable Analysis Snapshots** - Results stored permanently, never retroactively altered
- **Confidence Scoring** - Secondary score based on traffic volume significance
- **Change Ranking** - Ranks concurrent changes by likelihood of causing degradation
- **Customizable PromQL Templates** - Core queries configurable via `config.yaml` with `{{ .Services }}` and `{{ .Duration }}` variables
- **Additional Custom Metrics** - User-defined PromQL queries in config, collected alongside core metrics
- **Automatic Background Analysis** - Worker triggers analysis when impact windows close
- **Event Retention Policy** - Configurable TTL with automatic cleanup
- **Intent-Execution Linking** - Links CI events to K8s rollouts via shared `git_commit_sha` or `image_tag`
- **Full UI** - Dashboard timeline, service analytics, metric preferences, search, namespace/type filters

---

## In Progress / Short Term

- **Heuristic Linking UI** - Display linked intent+execution events as a single "Deployment Unit" in the timeline ("PR Merged -> Image Built -> K8s Rollout -> Analysis")
- **Multi-Metric UI Support** - Allow users to define arbitrary additional metrics that appear in the UI alongside the core five (backend support exists, UI toggle in progress)
- **State Persistence** - Reflect "PENDING" vs "READY" analysis status in the main events list
- **Live Diffing** - Show actual image tag changes (old -> new) when expanding an event

---

## Planned

- **Git Collector** - Direct collection of tags, releases, and merges from Git repositories
- **Markdown Metadata** - Render metadata URLs as clickable links (GitHub commits, ArgoCD UI)
- **Service Health Summary** - "Pulse" indicator on the homepage based on last known analysis score
- **Event Search Improvements** - Search by Git SHA and metadata values
- **Database Pruning UI** - Expose retention settings in the admin interface
- **RBAC Manifest Generation** - Pre-built YAML templates for ClusterRole/ClusterRoleBinding
- **OpenShift SecurityContextConstraints** - Support for restricted SCC environments
- **Concurrent Group Visualization** - Highlight services deploying at the same time
- **Manual Tags on Snapshots** - Allow users to add verification notes to analysis results

---

## Explicitly Not Planned

These features are considered as not in roadmap.

- Multi-cluster / multi-environment management
- Saved incidents and postmortem export
- RBAC / SSO integration
- Dependency proximity scoring (service mesh)
- Distributed event bus (Kafka/NATS) for buffering

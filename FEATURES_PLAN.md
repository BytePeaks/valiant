# Valiant: Features Roadmap

This document outlines the planned technical evolutions for Valiant, focusing on moving from a stable core to a high-quality investigation GUI for engineering teams.

---

## I. Intent & Execution Linking (The "Story" View)
**Goal:** Group fragmented events into a single logical narrative for the user.

- [ ] **Heuristic Linking:** Implement backend logic to link a `trigger_type: CI` event (Intent) with a `trigger_type: GitOps` event (Execution) if they share the same `git_sha` or `image_tag` within a specific time window.
- [ ] **The Story Timeline:** Update the Frontend to display these linked events as a single "Deployment Unit." 
    - *Visual:* "PR Merged (Intent) -> Image Built -> K8s Rollout (Execution) -> Analysis."
- [x] **Orphan Detection:** Highlight executions that have no matching intent (e.g., a deployment that happened without a corresponding CI build signal).
- [x] **Execution Fingerprinting:** Refine the Kubernetes Collector to ignore rollouts where the `generation` increased but the `valiant.io/git-sha` (or similar fingerprint) remained identical, effectively filtering out manual `kubectl edit` actions.

## II. Customizable Prometheus Queries (YAML Templates)
**Goal:** Allow Valiant to adapt to any environment (Istio, Nginx, Custom Exporters) without code changes.

- [x] **Query Templating:** Move hardcoded PromQL into `config.yaml`.
- [x] **Variable Substitution:** Use placeholders like `{{ .Services }}` and `{{ .Duration }}` in YAML strings.
- [ ] **Multi-Metric Support:** Allow users to define *arbitrary* additional metrics in the YAML config that should appear in the UI alongside the core five.

## III. Dashboard UX & Visuals
**Goal:** Make the investigation experience fast and information-dense.

- [ ] **Live Diffing:** When expanding an event, show the actual "Image Tag" change (Old Image -> New Image) captured during the rollout.
- [ ] **Markdown Metadata:** Render metadata values as clickable links if they are URLs (e.g., links to GitHub commits or ArgoCD UI).
- [ ] **Service Health Summary:** On the homepage, show a small "Pulse" indicator next to each service pill based on its *last* known analysis score.
- [ ] **Event Search:** Add a search bar to filter the timeline by Summary, Git SHA, or Metadata values.

## IV. Backend Robustness & Background Tasks
**Goal:** Automate the "boring" parts of analysis.

- [x] **Automatic Analysis:** Add a background worker that automatically triggers `AnalyzeImpact` once the `post_execution_impact_window` for an event has elapsed.
- [ ] **State Persistence:** Ensure the "PENDING" vs "READY" state is reflected in the main events list so users know which ones are ready to view.
- [ ] **Database Pruning:** Add a configurable retention policy (e.g., "Delete events older than 90 days") to keep the PostgreSQL instance lean.

## V. Contextual Awareness
**Goal:** Help users understand *why* a change might be related to another service.

- [ ] **Implicit Dependencies:** If Service A and Service B deploy at the same time, visually highlight them as a "Concurrent Group."
- [ ] **Manual Tags:** Allow users to add a small "verified" note to an analysis snapshot (e.g., "Confirmed: This latency was expected due to cache clear").

## VI. RBAC & Cross-Namespace Permissions
**Goal:** Enable Valiant to see deployments outside its own namespace in secure environments (OpenShift/K8s).

- [ ] **RBAC Manifest Generation:** Create a set of YAML templates in `/deploy` to grant the Valiant `ServiceAccount` the "Master Key" permissions.
    - **ClusterRole:** Define permissions to `get`, `list`, and `watch` resources in the `apps` group (Deployments, ReplicaSets).
    - **ClusterRoleBinding:** Connect the backend's ServiceAccount to the ClusterRole globally.
- [ ] **Namespace Scoping:** Refine the backend to gracefully handle "Permission Denied" errors. If it can't see Namespace X, it should log a clear instruction for the user: *"Run 'oc policy...' or apply 'cluster-role-binding.yaml' to enable visibility for this namespace."*
- [ ] **OpenShift Security Context:** Add support for OpenShift-specific `SecurityContextConstraints` (SCC) if needed for restricted environments.



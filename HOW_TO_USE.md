# How to Use Valiant

Valiant is a change impact radar designed to help engineering teams identify which **execution events** (successful deployments or CI runs) are correlated with service stability shifts.

---

## 🚀 Quick Start

The fastest way to get Valiant running is using Docker Compose:

```bash
docker-compose up -d --build
```

- **Dashboard:** [http://localhost:3000](http://localhost:3000)
- **Backend API:** [http://localhost:8080](http://localhost:8080)

---

## 🧩 Core Concepts

### 1. Intent vs. Execution
Valiant strictly separates **Intent** from **Execution**:
-   **Intent (Git):** Code commits or merges. These do NOT trigger monitoring because they haven't changed the running system yet. Git info is treated as metadata (e.g., a "Git SHA" attached to a build).
-   **Execution (Boundary):** When a system (like ArgoCD, Helm, or CI) actually applies a change to the cluster. **Stability monitoring only starts here.**

### 2. The "Auditor" Model
Valiant acts as a strict auditor. It only records a change if:
1.  It is **Intentional:** The Kubernetes object must have the `valiant.io/source` annotation.
2.  It is **Completed:** The rollout must be 100% finished and `Available`.

### 3. Impact Analysis (The Pivot)
When you click **Analyze**, Valiant uses two distinct windows:
1.  **Baseline Window:** Measures health *before* the rollout started (lookback from `rollout_start`).
2.  **Impact Window:** Measures health *after* the rollout finished (lookforward from `rollout_end`).

---

## 🔌 How to Connect Your Apps

Valiant connects the dots between your code and your cluster using **Shared Names**.

### 1. The Service Match
Valiant looks at the name of your Kubernetes Deployment (e.g., `payment-service`) and queries Prometheus for metrics with that same label (e.g., `service="payment-service"`). 

**Rule:** Ensure your Prometheus metrics have a label that matches your Deployment name.

### 2. The "Intent" Annotation (Required)
To prevent Valiant from recording accidental or manual changes (like a human typing `kubectl edit`), your Deployment **must** include this annotation:

```yaml
metadata:
  annotations:
    valiant.io/source: "argocd" # or "helm", "cicd"
```

If this is missing, Valiant will ignore the rollout.

### 2. The "Unique Fingerprint" (Requirement)
Static annotations like `valiant.io/source: "argocd"` only prove who *owns* the app. To prevent Valiant from recording a manual `kubectl edit`, your automation **must** provide a unique fingerprint for every change (like a Git SHA or Build ID).

**How it works:**
- **Automated Sync:** Your CI/CD updates the `valiant.io/git-sha`. Valiant sees a new SHA and records the event. ✅
- **Manual Edit:** A human edits the CPU limit. The SHA remains the same. Valiant sees the mismatch and **ignores** the change as "Unintentional." 🛑

---

## 📥 Ingesting Data

### Via Kubernetes (Automated)
If Valiant is in your cluster, it watches the Kubernetes API. It automatically detects when a Deployment generation changes and a new ReplicaSet appears. It waits for the "Available" signal before creating the event.

### Via REST API (CI Signals)
You can signal the start of a pipeline to record the **Intent** part of the story:

```bash
curl -X POST http://localhost:8080/api/v1/events \
  -H "Content-Type: application/json" \
  -d '{
    "trigger_type": "CI",
    "summary": "Building payment-service v2.4.0",
    "metadata": { "git_sha": "a1b2c3d" }
  }'
```

---

## 📊 Using the Dashboard

1.  **Timeline:** Chronological list of execution boundaries.
2.  **Pending State:** If a deployment just finished, you'll see a **"Pending"** badge. You must wait for the observation window (default 30m) to close before you can see the results.
3.  **Analyze:** Trigger the calculation. Results are **Snapshotted**—meaning they are frozen in time and won't change even if you deploy something else later.
4.  **Confidence Score:** A high score (Shield icon) means there was enough traffic (RPS) to make the percentage deltas statistically reliable.

---

## 📈 Understanding the Metrics

| Metric | Measures | Why it matters |
| :--- | :--- | :--- |
| **Errors** | 5xx Rate | **Stability:** Direct signal of crashes or broken logic. |
| **Latency** | P95 Speed | **Experience:** High values mean your app is lagging. |
| **RPS** | Traffic | **Flow:** A sudden drop might mean users can't reach you. |
| **CPU/Mem** | Resources | **Efficiency:** Spikes here indicate potential leaks or regressions. |
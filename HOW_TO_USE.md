# How to Use Valiant

Valiant is a change impact radar designed to help engineering teams quickly identify which system changes (deployments, configuration updates, git tags) are correlated with service degradations.

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

### 1. Change Events
A **ChangeEvent** is a normalized representation of any change in your system. This can be:
- A Kubernetes Deployment rollout.
- A Git Tag/Release.
- A manual configuration change.
- A CI/CD build success.

### 2. Impact Analysis
When you "Analyze" an event, Valiant:
1.  **Lookback:** Defines a **Baseline** window (30m to 5m before the change).
2.  **Lookforward:** Defines an **Impact** window (5m to 30m after the change).
3.  **Metrics Query:** Fetches average metrics (Error Rate, Latency, etc.) from Prometheus for both windows.
4.  **Scoring:** Calculates the percentage delta and assigns an **Impact Score (0-100%)** based on weighted heuristics.

---

## 🔌 How to Connect Your Apps (The "Magic")

You might be wondering: *"How does Valiant know that my deployment caused that error rate spike?"*

It relies on **Shared Names**.

### 1. The Name Match
When a deployment happens (e.g., via the Kubernetes Collector), Valiant creates an event with two key pieces of info:
1.  **Timestamp:** When it happened.
2.  **Affected Service:** e.g., `payment-service` (This typically comes from your Kubernetes Deployment name).

When you click **Analyze**, Valiant turns to Prometheus and essentially asks:
> *"Hey, show me the `http_requests_total` and `latency` for `service="payment-service"` around the time of the deployment."*

**Crucial Rule:** Your Prometheus metrics **must** have a label (like `service`, `app`, or `job`) that matches the name in the Change Event. If the names match, the correlation works automatically.

### 2. Running inside Kubernetes
If you deploy Valiant into the same Kubernetes/OpenShift cluster as your apps, here is how it gathers data without you doing anything:

*   **Gathering Deployments:** Valiant uses a standard **ServiceAccount**. It talks to the Kubernetes API (just like `kubectl` does) and watches for changes. When you run `kubectl apply` or a Helm upgrade, Valiant sees the Deployment update and records it.
*   **Gathering Metrics:** Valiant sends HTTP requests to your internal Prometheus service (e.g., `http://prometheus-k8s.monitoring.svc:9090`). It doesn't need to touch your pods directly; it just reads the data Prometheus has already scraped.

**Summary:** You don't need to change your application code. As long as you have standard Prometheus monitoring, Valiant acts as the "detective" connecting the clues (deployments) to the evidence (metrics).

---

## 📥 Ingesting Data

### Via REST API
You can push events directly to Valiant from your CI/CD pipelines or scripts.

```bash
curl -X POST http://localhost:8080/api/v1/events \
  -H "Content-Type: application/json" \
  -d '{
    "id": "prod-deploy-123",
    "source": "ci-cd",
    "change_type": "deployment",
    "timestamp": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'",
    "affected_services": ["payment-service"],
    "summary": "Deploying payment-service v2.4.0",
    "metadata": {
      "author": "dev-team",
      "commit": "a1b2c3d"
    }
  }'
```

### Via Collectors (Automated)
Valiant includes built-in collectors that can poll for changes:

- **Kubernetes Collector:** Watches for `NewReplicaSetAvailable` conditions in your cluster.
- **Git Collector:** Inspects a local repository for new tags.

*Note: Collectors require appropriate environment configuration (KUBECONFIG path, Repo path) in the backend settings.*

---

## 📊 Using the Dashboard

1.  **Timeline:** View a chronological list of all recent system changes.
2.  **Refresh:** Use the refresh button to pull the latest events from the database.
3.  **Analyze:** Click the **"Analyze"** button on any event to trigger the correlation engine.
4.  **Inspect Deltas:** Review the color-coded metric cards to see exactly which metrics shifted after the change.
    - 🔴 **Red:** Metric degraded (e.g., Error Rate increased).
    - 🟢 **Green:** Metric improved or decreased (e.g., Latency dropped).
    - ⚪ **Grey:** No significant change.

## 📈 Understanding the Metrics

When you see a **Metric Shift**, it shows the percentage change compared to the period before the event.

| Metric | What it measures | Why it matters |
| :--- | :--- | :--- |
| **Errors** | HTTP 5xx responses | **High Impact:** An increase here means your change is likely causing crashes or broken logic. |
| **Latency** | P95 Response Time | **User Experience:** If this goes up, your app is slower. Users might experience lag or timeouts. |
| **RPS** | Requests Per Second | **Traffic Flow:** A sudden drop could mean a upstream service is failing to reach your app. |
| **CPU** | Processor Usage | **Efficiency:** If this spikes without an RPS increase, your new code might have an infinite loop or high complexity. |
| **Memory** | RAM Consumption | **Stability:** If this keeps growing, your app might eventually crash due to Out-of-Memory (OOM) errors. |

---

## ⚙️ Configuration

Valiant is configured via environment variables in `docker-compose.yml`:

| Variable | Description | Default |
| :--- | :--- | :--- |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://...` |
| `PROMETHEUS_URL` | URL of your Prometheus instance | `http://localhost:9090` |
| `PORT` | Backend API port | `8080` |
| `NEXT_PUBLIC_API_URL` | API URL for the frontend | `http://localhost:8080/api/v1` |


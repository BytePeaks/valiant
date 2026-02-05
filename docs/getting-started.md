# Getting Started

Get Valiant running locally in under 5 minutes.

---

## Prerequisites

| Requirement | Minimum Version |
|:------------|:----------------|
| Docker + Docker Compose | Latest stable |
| Go | 1.25+ (only for development) |
| Node.js | 20+ (only for development) |
| PostgreSQL | 16+ (provided by Docker Compose) |
| Prometheus | 2.x+ (your existing instance) |
| Kubernetes cluster | Any (for the K8s collector) |

---

## Quick Start with Docker Compose

```bash
git clone https://github.com/BytePeaks/valiant.git
cd valiant
docker-compose up --build -d
```

This starts three containers:
- **PostgreSQL** on port `5432` (with automatic schema migrations)
- **Backend** on port `8080` (mounts `example/config.yaml`)
- **Frontend** on port `3000`

Verify everything is running:

```bash
# Backend health check
curl http://localhost:8080/health
# OK

# Open the dashboard
open http://localhost:3000
```

---

## Local Development (without Docker)

For active development, run each component separately:

```bash
# 1. Start just the database
docker-compose up -d postgres

# 2. Start the backend
cd backend
cp ../example/config.yaml config.yaml  # Edit if needed
DATABASE_URL="postgres://valiant:valiant_password@localhost:5432/valiant?sslmode=disable" go run cmd/valiant/main.go

# 3. Start the frontend (in another terminal)
cd frontend
npm install
npm run dev
```

---

## Kubernetes Deployment

For deploying to a Kubernetes or OpenShift cluster:

```bash
# Apply the manifests (adjust namespace as needed)
kubectl apply -k deploy/
```

Ensure the backend has:
- A `DATABASE_URL` environment variable pointing to your PostgreSQL instance
- Network access to your Prometheus instance
- A `ServiceAccount` with permissions to watch Deployments, ReplicaSets, ConfigMaps, and Secrets (requires a `ClusterRole` for cross-namespace visibility)

---

## Connecting Your Apps

Valiant connects your Kubernetes Deployments to Prometheus metrics using **shared names**.

### 1. Service Name Matching

Valiant reads the name of your Kubernetes Deployment (e.g., `payment-service`) and queries Prometheus for metrics with the same label value (e.g., `service="payment-service"`).

**Rule**: Ensure your Prometheus metrics have a label that matches your Deployment name.

### 2. The `valiant.io/source` Annotation (Required)

To prevent Valiant from recording accidental changes (like a human running `kubectl edit`), your Deployment **must** include this annotation:

```yaml
metadata:
  annotations:
    valiant.io/source: "argocd"  # or "helm", "cicd"
```

If this annotation is missing, Valiant ignores the rollout entirely.

### 3. Unique Fingerprint (Recommended)

For full intent-execution linking, your CI/CD tool should set a unique fingerprint on each deployment:

```yaml
metadata:
  annotations:
    valiant.io/source: "argocd"
    valiant.io/git-sha: "a1b2c3d4"
```

This allows Valiant to:
- Link CI builds to K8s rollouts via matching SHAs
- Detect orphaned deployments (no corresponding CI signal)
- Filter out manual edits where the fingerprint didn't change

---

## Sending Your First Event

If you don't have Kubernetes connected yet, submit a test event via the API:

```bash
curl -X POST http://localhost:8080/api/v1/events \
  -H "Content-Type: application/json" \
  -d '{
    "trigger_type": "CI",
    "change_type": "build_success",
    "affected_services": ["payment-service"],
    "summary": "Build payment-service v1.0.0",
    "timestamp": "'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'",
    "metadata": {
      "git_commit_sha": "a1b2c3d4e5f6",
      "image_tag": "v1.0.0"
    }
  }'
```

Then check it appeared:

```bash
curl http://localhost:8080/api/v1/events | jq '.events[0]'
```

---

## Triggering Your First Analysis

After the impact window closes (default 30 minutes after the event), trigger analysis:

**Via the UI**: Navigate to the service page and click "Analyze" on the event card.

**Via the API**:

```bash
curl -X POST http://localhost:8080/api/v1/analyze \
  -H "Content-Type: application/json" \
  -d '{"event_id": "YOUR_EVENT_ID"}'
```

If the impact window hasn't closed yet, you'll get a `422` response with `"impact_level": "PENDING"`. Wait for the window to close and try again - or let the background worker handle it automatically.

---

## Next Steps

- [How Valiant Works](how-it-works.md) - Understand the scoring engine and analysis model
- [Configuration](configuration.md) - Customize Prometheus queries, analysis windows, and K8s settings
- [API Reference](api-reference.md) - Full endpoint documentation

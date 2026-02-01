# How to Use Valiant

Valiant is a change impact radar designed to help engineering teams identify which **execution events** (successful deployments or CI runs) are correlated with service stability shifts.

---

## Table of Contents
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Understanding Docker Compose](#understanding-docker-compose)
- [Core Concepts](#core-concepts)
- [How to Connect Your Apps](#how-to-connect-your-apps)
- [Ingesting Data](#ingesting-data)
- [Using the Dashboard](#using-the-dashboard)
- [Understanding the Metrics](#understanding-the-metrics)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)

---

## Prerequisites

Before running Valiant, ensure you have the following installed:

### Required Software

#### Docker & Docker Compose
Valiant runs in containers, so Docker is required.

**Check if installed:**
```bash
docker --version
docker-compose --version
```

**Required versions:**
- Docker: 20.10 or higher
- Docker Compose: 2.0 or higher

**Installation:**
- **Windows:** [Docker Desktop for Windows](https://docs.docker.com/desktop/install/windows-install/)
- **Mac:** [Docker Desktop for Mac](https://docs.docker.com/desktop/install/mac-install/)
- **Linux:** [Docker Engine](https://docs.docker.com/engine/install/)

#### Go (Optional - for development)
Only needed if you want to run the backend without Docker.

**Check if installed:**
```bash
go version
```

**Required version:** Go 1.21 or higher

**Installation:** [Download Go](https://go.dev/doc/install)

#### Node.js & npm (Optional - for development)
Only needed if you want to run the frontend without Docker.

**Check if installed:**
```bash
node --version
npm --version
```

**Required versions:**
- Node.js: 18.0 or higher
- npm: 8.0 or higher

**Installation:** [Download Node.js](https://nodejs.org/)

### Verify Your Setup

Run these commands to ensure everything is installed correctly:
```bash
docker --version        # Should show Docker version 20.10.x or higher
docker-compose --version # Should show version 2.x.x or higher
```

If both show version numbers, you're ready! ✅

---

## 🚀 Quick Start

The fastest way to get Valiant running is using Docker Compose.

### Step 1: Clone the Repository

If you haven't already:
```bash
git clone https://github.com/BytePeaks/valiant.git
cd valiant
```

### Step 2: Start All Services
```bash
docker-compose up -d --build
```

**What this does:**
- `up`: Starts all services
- `-d`: Runs in detached mode (background)
- `--build`: Builds images from source

**You'll see output like:**
```
Creating network "valiant_default" ...
Building backend...
Building frontend...
Creating valiant_backend_1  ... done
Creating valiant_frontend_1 ... done
```

### Step 3: Access Valiant

Once services are running:
- **Dashboard (Frontend):** [http://localhost:3000](http://localhost:3000)
- **Backend API:** [http://localhost:8080](http://localhost:8080)

**First-time users:** It may take 30-60 seconds for all services to be fully ready.

### Useful Commands

**View logs:**
```bash
docker-compose logs -f
```

**View logs for specific service:**
```bash
docker-compose logs -f backend
docker-compose logs -f frontend
```

**Stop services:**
```bash
docker-compose down
```

**Restart services:**
```bash
docker-compose restart
```

**Stop and remove all data:**
```bash
docker-compose down -v
```

---

## Understanding Docker Compose

The `docker-compose.yml` file defines how Valiant's services work together.

### Services Overview

Valiant consists of multiple services:

**Backend Service:**
- Written in Go
- Connects to Kubernetes API
- Provides REST API for events
- Runs on port 8080

**Frontend Service:**
- React-based dashboard
- User interface for viewing timeline and analysis
- Runs on port 3000

### Port Mappings

| Service | Container Port | Your Computer | Purpose |
|---------|---------------|---------------|---------|
| Frontend | 3000 | localhost:3000 | Web dashboard |
| Backend | 8080 | localhost:8080 | REST API |

### Understanding docker-compose.yml

Open the file to see the configuration:
```bash
type docker-compose.yml    # Windows
cat docker-compose.yml     # Mac/Linux
```

**Key sections:**
- `services:` - Defines what containers to run
- `build:` - How to build each service
- `ports:` - Port mappings (container:host)
- `environment:` - Configuration variables
- `volumes:` - Persistent data storage

**Example customization:**

If port 3000 is already in use, you can change it:
```yaml
frontend:
  ports:
    - "3001:3000"  # Change 3001 to any available port
```

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

### 3. The "Unique Fingerprint" (Requirement)
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

When you analyze an event, Valiant compares metrics from two time windows:

### Metrics Comparison

| Metric | Measures | Why it matters |
| :--- | :--- | :--- |
| **Errors** | 5xx Rate | **Stability:** Direct signal of crashes or broken logic. |
| **Latency** | P95 Speed | **Experience:** High values mean your app is lagging. |
| **RPS** | Traffic | **Flow:** A sudden drop might mean users can't reach you. |
| **CPU/Mem** | Resources | **Efficiency:** Spikes here indicate potential leaks or regressions. |

### Interpreting Results

**Good Signs (Green):**
- Error rate stays the same or decreases
- Latency stays stable or improves
- RPS remains consistent
- CPU/Memory usage is stable

**Warning Signs (Yellow/Red):**
- Sudden spike in error rate (5xx errors)
- Latency increases significantly
- RPS drops unexpectedly
- Memory increases steadily (potential leak)

**What to do:**
- **If metrics worsen:** Consider rolling back the deployment
- **If metrics improve:** The change had a positive impact
- **If metrics are stable:** Change didn't affect stability

---

## Troubleshooting

### Common Issues and Solutions

#### "Cannot connect to Docker daemon"

**Error:**
```
Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?
```

**Solution:**
1. Make sure Docker Desktop is running
2. On Windows: Check system tray for Docker icon
3. On Mac: Check menu bar for Docker icon
4. On Linux: Run `sudo systemctl start docker`

#### "Port already in use"

**Error:**
```
Error: bind: address already in use
```

**Solution:**

**Option 1: Stop the conflicting process**
```bash
# Windows PowerShell
netstat -ano | findstr :3000
taskkill /PID <PID_NUMBER> /F

# Mac/Linux
lsof -ti:3000 | xargs kill -9
```

**Option 2: Change the port in docker-compose.yml**
```yaml
frontend:
  ports:
    - "3001:3000"  # Use 3001 instead
```

#### "docker-compose: command not found"

**Error:**
```
'docker-compose' is not recognized as an internal or external command
```

**Solution:**

Docker Compose should come with Docker Desktop. Try:
```bash
docker compose up -d --build
```

(Note: `docker compose` without the hyphen)

If still not working, reinstall Docker Desktop.

#### Services start but dashboard shows error

**Symptoms:** 
- `docker-compose up` succeeds
- Dashboard at localhost:3000 shows "Cannot connect to backend"

**Solution:**

1. **Check if backend is running:**
```bash
   docker-compose logs backend
```

2. **Look for errors in logs**

3. **Check backend is accessible:**
```bash
   curl http://localhost:8080/health
```

4. **Restart services:**
```bash
   docker-compose restart
```

#### Frontend shows blank page

**Solution:**

1. **Clear browser cache:** Ctrl+Shift+R (or Cmd+Shift+R on Mac)
2. **Check browser console:** Press F12, look for errors
3. **Verify frontend is running:**
```bash
   docker-compose logs frontend
```

#### Changes to code not reflecting

**If you modified code but don't see changes:**
```bash
# Rebuild images
docker-compose up -d --build

# Or completely rebuild
docker-compose down
docker-compose up -d --build
```

#### Permission denied errors (Linux)

**Error:**
```
Permission denied while trying to connect to the Docker daemon socket
```

**Solution:**
```bash
# Add your user to docker group
sudo usermod -aG docker $USER

# Log out and log back in, or run:
newgrp docker

# Try command again
docker-compose up -d --build
```

### Still Having Issues?

If you're still stuck:

1. **Check existing issues:** [GitHub Issues](https://github.com/BytePeaks/valiant/issues)
2. **Search for your error message**
3. **Open a new issue** with:
   - Your operating system (Windows/Mac/Linux)
   - Docker version (`docker --version`)
   - Complete error message
   - Steps you've tried

---

## Next Steps

Now that Valiant is running:

1. **Connect your Kubernetes cluster** - Follow the "How to Connect Your Apps" section
2. **Add the required annotations** to your Deployments
3. **View the timeline** at http://localhost:3000
4. **Trigger an analysis** to see impact metrics
5. **Read CONTRIBUTING.md** if you want to contribute to Valiant

---

**Need help?** Open an issue or check the documentation!
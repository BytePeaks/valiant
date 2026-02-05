# Valiant

**Change Impact Radar for Kubernetes**

[![Status](https://img.shields.io/badge/status-active-brightgreen?style=flat)](https://github.com/BytePeaks/valiant)
[![License](https://img.shields.io/github/license/BytePeaks/valiant?style=flat)](https://github.com/BytePeaks/valiant/blob/main/LICENSE)
[![Go](https://img.shields.io/badge/go-%3E%3D1.25-blue?style=flat)](https://golang.org)
[![Last Commit](https://img.shields.io/github/last-commit/BytePeaks/valiant?style=flat)](https://github.com/BytePeaks/valiant/commits)
[![Repo Size](https://img.shields.io/github/repo-size/BytePeaks/valiant?style=flat)](https://github.com/BytePeaks/valiant)

> *"Stop wasting hours asking 'which deploy broke this?'"*

---

## The Problem

It's 3am. Latency is spiking. Five teams deployed in the last hour. Your monitoring tells you *something* is broken - error rates are up, p95 is through the roof - but it can't tell you **which change caused it**.

You open Grafana, cross-reference deploy times from ArgoCD, check the CI pipeline history, compare metrics before and after each deploy... manually. For every single change.

Existing tools tell you **what** is broken. Nobody tells you **which change** broke it.

## How Valiant Solves It

Valiant watches your cluster and automatically correlates changes with metric shifts:

- **Watches Kubernetes** - Deployment rollouts, ConfigMap/Secret changes, captured the moment they go live
- **Correlates with Prometheus** - Compares baseline metrics (before change) vs impact metrics (after change) using your existing Prometheus data
- **Scores deterministically** - No ML, no black boxes. Weighted scoring across error rate, latency, RPS, CPU, and memory. Every score is explainable.
- **Ranks concurrent changes** - When 5 deploys happened in the same hour, Valiant ranks them by likelihood of being the cause

---

## Screenshots

### Dashboard
![Dashboard](docs/images/dashboard.png)
Filter by service, namespace, and change type. Search events. See analysis status at a glance.

### Service Analytics
![Service Analytics](docs/images/service_analytics.png)
Impact scores, metric shifts (baseline vs impact), confidence scoring, and orphan detection for each change event.

### Custom Metrics
![Custom Metrics](docs/images/service_analytics_custom_metrics.png)
Define business-specific PromQL queries in `config.yaml` (e.g., orders/min, payment failures). Toggle visibility per service.

---

## Key Features

- **Kubernetes native** - Watches Deployments, ConfigMaps, Secrets with annotation-based filtering
- **CI/CD webhooks** - Ingest events from any pipeline via REST API
- **Intent-execution linking** - Links CI builds to K8s rollouts via Git SHA or image tag
- **Deterministic scoring** - Weighted impact score (0-1) with NONE/LOW/MEDIUM/HIGH classification
- **Custom metrics** - Define additional PromQL queries in config, collected alongside core metrics
- **Incident investigation** - Rank concurrent changes by likelihood of causing degradation
- **Automatic analysis** - Background worker triggers analysis when impact windows close
- **Configurable retention** - Automatic event cleanup (default 90 days)
- **Immutable snapshots** - Analysis results are frozen in time, never retroactively altered
- **REST API** - Full programmatic access to events, analysis, rankings, and preferences

---

## Architecture

[![Architecture Diagram](https://mermaid.ink/img/pako:eNptUttu00AQ_ZXVSkUg5WK3buz4ASmxHVQVoTQJVMKpqk08OCb2brS7hoQk34B4440XPqLfww_AJzC-NEkR87CanTnHM3uOt3QuIqAujSVbLcjEn3KCcXZGenMtpCLPSLDWIDlLyXijNGSqQrxVIMM_P779LLO7qnjtqPDX94ffD1_JdT5DFmhQpDe8qvveleeH3lXb88kwWUGacMDOYeQ7liaMa-IJCXhkK8GB63qgymfVjjXqvlqnahYxCML2QAqugUekSd7AWrc-qvbdEdEPwj6bL6v-K1Eshu_DaRJShq89gXoiTaFUIDymdR_5J0v7sMIC8HkC_27qM83ux8h87BTh98PnQ6F0LGF88_rFycyhFFlYHKAXkP9_mic4x2USwU98IM3my11fis-Y71CHqjMIyvooGE_Kl3osTZVLBqDn1WoNMpFJHCO_h_ZuVKJ2KNHjMPSy5N8yxKOLI5RB5BoxR0GOrlZQmC2EWD5F1JhDpUR6C8ZjCD4V9p4O7Vc7ozBkBCxq38pEww41e9IuNELETQ4SRd-VwtEG_sJJRF0tc2jQDGTGiivdFtQpRUkzmFIX04jJ5ZRO-R45K8bfC2TXNCnyeEHdDyxVeMtXEdPgJwztzA5VWdgtPZFzTd1zo_wGdbd0TV2rc9GyTLN72bEd2zS6VoNuqGsbLevCtmzbNE3H7Fj7Bv1SzjRajn1pFOGYjm0Z3fP9Xw1xIKo?type=png)](https://mermaid.live/edit#pako:eNptUttu00AQ_ZXVSkUg5WK3buz4ASmxHVQVoTQJVMKpqk08OCb2brS7hoQk34B4440XPqLfww_AJzC-NEkR87CanTnHM3uOt3QuIqAujSVbLcjEn3KCcXZGenMtpCLPSLDWIDlLyXijNGSqQrxVIMM_P779LLO7qnjtqPDX94ffD1_JdT5DFmhQpDe8qvveleeH3lXb88kwWUGacMDOYeQ7liaMa-IJCXhkK8GB63qgymfVjjXqvlqnahYxCML2QAqugUekSd7AWrc-qvbdEdEPwj6bL6v-K1Eshu_DaRJShq89gXoiTaFUIDymdR_5J0v7sMIC8HkC_27qM83ux8h87BTh98PnQ6F0LGF88_rFycyhFFlYHKAXkP9_mic4x2USwU98IM3my11fis-Y71CHqjMIyvooGE_Kl3osTZVLBqDn1WoNMpFJHCO_h_ZuVKJ2KNHjMPSy5N8yxKOLI5RB5BoxR0GOrlZQmC2EWD5F1JhDpUR6C8ZjCD4V9p4O7Vc7ozBkBCxq38pEww41e9IuNELETQ4SRd-VwtEG_sJJRF0tc2jQDGTGiivdFtQpRUkzmFIX04jJ5ZRO-R45K8bfC2TXNCnyeEHdDyxVeMtXEdPgJwztzA5VWdgtPZFzTd1zo_wGdbd0TV2rc9GyTLN72bEd2zS6VoNuqGsbLevCtmzbNE3H7Fj7Bv1SzjRajn1pFOGYjm0Z3fP9Xw1xIKo)

Go backend + Next.js frontend + PostgreSQL + Prometheus (HTTP API). See [Architecture](docs/architecture.md) for details.

---

## Quick Start

```bash
git clone https://github.com/BytePeaks/valiant.git
cd valiant
docker-compose up --build -d
```

| Service | URL |
|:--------|:----|
| Dashboard | [http://localhost:3000](http://localhost:3000) |
| Backend API | [http://localhost:8080](http://localhost:8080) |
| Health Check | [http://localhost:8080/health](http://localhost:8080/health) |

Send a test event:

```bash
curl -X POST http://localhost:8080/api/v1/events \
  -H "Content-Type: application/json" \
  -d '{
    "trigger_type": "CI",
    "change_type": "build_success",
    "affected_services": ["payment-service"],
    "summary": "Build payment-service v1.0.0",
    "timestamp": "'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'",
    "metadata": {"git_commit_sha": "a1b2c3d4"}
  }'
```

See [Getting Started](docs/getting-started.md) for full setup, Kubernetes deployment, and connecting your apps.

---

## Documentation

| Document | Description |
|:---------|:------------|
| [Getting Started](docs/getting-started.md) | Installation, setup, first event, first analysis |
| [How It Works](docs/how-it-works.md) | Core concepts, scoring engine, analysis model |
| [Configuration](docs/configuration.md) | Full config reference, Prometheus queries, custom metrics |
| [API Reference](docs/api-reference.md) | All REST endpoints with examples |
| [Architecture](docs/architecture.md) | Components, data flow, design trade-offs |
| [Troubleshooting](docs/troubleshooting.md) | Common errors, performance, security |
| [Roadmap](docs/roadmap.md) | Completed features, planned work |

---

## How Valiant Compares

| | Traditional Monitoring | AIOps Platforms | Valiant |
|:--|:----------------------|:----------------|:--------|
| **Answers** | "What is broken?" | "What might be the cause?" | "Which change caused this?" |
| **Method** | Threshold alerts | ML-based correlation | Deterministic rule-based scoring |
| **Explainability** | High (simple thresholds) | Low (black box) | High (every score is traceable) |
| **Setup** | Requires alert rules | Requires training data | Watches your existing K8s + Prometheus |

---

## Roadmap

- Intent-execution linking UI ("deployment story" timeline)
- Git collector for tags and releases
- Service health pulse indicators
- RBAC manifest generation for OpenShift

See [Roadmap](docs/roadmap.md) for the full list.

---

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on development setup, code style, testing, and submitting pull requests.

---

## License

[AGPL-3.0](LICENSE) - If you **modify** and deploy Valiant as a network service, you must make your source code available.

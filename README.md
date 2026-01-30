# Valiant

Valiant is an **open-source change impact radar** for DevOps and SRE teams.  
It automatically correlates **infrastructure, deployment, and configuration changes** with **service degradation**, helping teams quickly identify which recent change caused errors, latency spikes, or traffic drops.  

Valiant is built with **Go** on the backend, **Next.js** for the frontend, **PostgreSQL** for storage, and integrates seamlessly with **Prometheus** metrics.

---

## Features

- Correlates system changes with metrics: error rate, latency, traffic, CPU/memory saturation
- Visual timeline of changes and service impact
- Deterministic and explainable impact scoring
- Lightweight, self-hosted, open-source core

---

## Getting Started

### Prerequisites
- Docker and Docker Compose
- Go 1.25+ (for local dev)
- Node.js 20+ (for local dev)

### Running with Docker Compose

To start the full stack (Frontend, Backend, Database):

```bash
docker-compose up --build
```

- Frontend: [http://localhost:3000](http://localhost:3000)
- Backend API: [http://localhost:8080/health](http://localhost:8080/health)
- Database: Postgres on port 5432

### Seeding Data

To populate the system with some mock change events (useful for testing the timeline UI):

1. Ensure the stack is running (`docker-compose up`).
2. Run the seed script from your host machine:

```bash
chmod +x backend/scripts/seed_data.sh
./backend/scripts/seed_data.sh
```

Or on Windows (PowerShell):

```powershell
./backend/scripts/seed_data.ps1
```

### Local Development

#### Backend
```bash
cd backend
go mod tidy
go run cmd/valiant/main.go
```

#### Frontend
```bash
cd frontend
npm install
npm run dev
```

## Contributing

Valiant is fully open-source. Contributions are welcome via pull requests, issues, and feature suggestions.
Please follow standard Go and Next.js coding conventions.

## License

Valiant OSS core is licensed under AGPL-3.0. [LICENSE](LICENSE)
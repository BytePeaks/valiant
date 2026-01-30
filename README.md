Valiant is a change impact radar for engineering teams, correlating system changes with service degradation.

For a detailed guide on how to set up and use the system, see [HOW_TO_USE.md](./HOW_TO_USE.md).

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
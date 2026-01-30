# Valiant

Valiant is a change impact radar for engineering teams, correlating system changes with service degradation.

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

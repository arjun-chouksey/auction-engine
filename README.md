# Auction Engine

A British Auction / reverse-auction RFQ system built as a Go modular monolith with a React frontend and PostgreSQL. The design supports open supplier bidding, deterministic rankings, automatic near-close extensions, a forced closing cap, and an auditable event history.

> Current implementation status: the project bootstrap and local development harness are complete. The repository currently provides the runnable API/UI skeleton, health endpoint, PostgreSQL service, migration runner wiring, and baseline tests. Auction business features described in the design documents are not implemented yet.

## Architecture

- **Backend:** Go, `net/http`, Chi
- **Database:** PostgreSQL with pgx planned for transactional data access
- **Frontend:** React, TypeScript, Vite
- **Local runtime:** Docker Compose
- **Live updates:** Server-Sent Events as best-effort invalidation; PostgreSQL remains authoritative
- **Structure:** Single-deployable modular monolith

The submission design documents are:

- [High-level and low-level architecture](docs/02_ARCHITECTURE.md)
- [Database schema design](docs/03_DATABASE_SCHEMA.md)

## Repository structure

```text
backend/
  cmd/server/             Go composition root
  internal/config/        Environment configuration
  internal/httpapi/       Chi router and HTTP handlers
  migrations/             Versioned SQL migration location
frontend/
  src/app/                React router shell and pages
  src/                    Entry point, tests, and styles
compose.yaml              PostgreSQL, API, UI, and migration tooling
Makefile                  Formatting, test, build, and local commands
```

## Local development

### Prerequisites

- Docker Desktop with Docker Compose
- Node.js 20.19+ for frontend checks outside containers
- Go 1.27.3 for backend checks outside containers

Copy the local environment template and start all services:

```bash
cp .env.example .env
docker compose up --build
```

Local endpoints:

- Frontend: <http://localhost:5173>
- API health: <http://localhost:8080/healthz>
- PostgreSQL: `localhost:5432`

The health endpoint returns:

```json
{"status":"ok"}
```

Stop the stack without removing database data:

```bash
docker compose down
```

## Verification

Run the complete available check suite:

```bash
make test
```

Or run checks separately:

```bash
make backend-lint
make backend-test
make frontend-install
make frontend-lint
make frontend-test
make frontend-build
```

Frontend dependencies are pinned in `package-lock.json`; Go dependencies are recorded in `backend/go.mod` and `backend/go.sum`. Container image versions are pinned in the Docker and Compose files.

## Database migrations

The migration runner is wired, while schema migrations are intentionally not present in the current bootstrap. Once versioned `*.up.sql` and `*.down.sql` files exist under `backend/migrations`, apply them with:

```bash
make migrate-up
```

Roll back one migration with:

```bash
make migrate-down
```

## Core design constraints

- Monetary values use integer minor units and decimal strings in JSON.
- Every accepted bid is immutable.
- Only the latest accepted bid per supplier participates in current ranking.
- Bid acceptance and extension decisions share one PostgreSQL transaction and RFQ row lock.
- The forced close is absolute.
- Server-Sent Events are best-effort signals to refetch canonical database state.
- No scheduler or in-memory component is required for auction correctness.

## Scope

The planned submission covers the HLD, schema design, backend, and frontend for the RFQ auction workflow. Production deployment, payments, supplier awards, external identity providers, distributed messaging, and Kubernetes are outside the project scope.

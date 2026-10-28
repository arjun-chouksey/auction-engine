# Auction Engine

A British Auction / reverse-auction RFQ system built as a Go modular monolith with a React frontend and PostgreSQL. The design supports open supplier bidding, deterministic rankings, automatic near-close extensions, a forced closing cap, and an auditable event history.

> Current implementation status: authentication, RFQ creation/list/detail, immutable quote submission, current ranking, full bid history, and all three transactional auction-extension policies are implemented. Activity/SSE endpoints and the complete frontend workflow remain future work.

## Architecture

- **Backend:** Go, `net/http`, Chi
- **Database:** PostgreSQL with pgx/pgxpool transactional data access
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
  internal/auth/          JWT issue and validation
  internal/config/        Environment configuration
  internal/domain/        Deterministic auction domain logic
  internal/httpapi/       Chi router and HTTP handlers
  internal/repository/    PostgreSQL repositories and transactions
  internal/service/       Application validation and authorization
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

If port 8080 is already in use, map the API to another host port without changing its container port:

```bash
API_HOST_PORT=18082 docker compose up --build
```

### Schema and demo accounts

Apply all pending migrations and seed the local development users:

```bash
make migrate-up
make seed
```

The seed is idempotent. These accounts are for local development only and all use the password configured by `DEMO_PASSWORD` (default: `AuctionDemo2026!`):

| Role | Email |
|---|---|
| Buyer | `buyer@example.test` |
| Supplier | `supplier.a@example.test` |
| Supplier | `supplier.b@example.test` |
| Supplier | `supplier.c@example.test` |

## Verification

Run the complete available check suite:

```bash
make test
```

Or run checks separately:

```bash
make backend-lint
make backend-test
make backend-integration-test
make frontend-install
make frontend-lint
make frontend-test
make frontend-build
```

Frontend dependencies are pinned in `package-lock.json`; Go dependencies are recorded in `backend/go.mod` and `backend/go.sum`. Container image versions are pinned in the Docker and Compose files.

## Database migrations

Versioned up/down migrations live under `backend/migrations`. Apply them with:

```bash
make migrate-up
```

Roll back one migration with:

```bash
make migrate-down
```

## HTTP API

All application endpoints use the `/api/v1` prefix. Authentication uses a one-hour HS256 bearer token. The server validates the signing algorithm, issuer, signature, and expiry; tokens are not logged.

| Method | Path | Access |
|---|---|---|
| `GET` | `/healthz` | Public |
| `POST` | `/api/v1/auth/login` | Public |
| `GET` | `/api/v1/auth/me` | Authenticated |
| `POST` | `/api/v1/rfqs` | Buyer |
| `GET` | `/api/v1/rfqs` | Authenticated |
| `GET` | `/api/v1/rfqs/{id}` | Authenticated |
| `POST` | `/api/v1/rfqs/{id}/bids` | Supplier |
| `GET` | `/api/v1/rfqs/{id}/ranking` | Authenticated |
| `GET` | `/api/v1/rfqs/{id}/bids` | Authenticated |

Login using a local demo account:

```bash
curl -sS http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"buyer@example.test","password":"AuctionDemo2026!"}'
```

Pass the returned access token as `Authorization: Bearer <token>`. RFQ list parameters are `page`, `page_size`, and optional `status` (`UPCOMING`, `ACTIVE`, `CLOSED`, or `FORCE_CLOSED`).

Quote submission requires a supplier token and a new UUID idempotency key:

```bash
curl -sS -X POST http://localhost:8080/api/v1/rfqs/<rfq-id>/bids \
  -H 'Authorization: Bearer <supplier-token>' \
  -H 'Idempotency-Key: 35b5ca37-f11c-4bf7-a4a0-51fd62cd754b' \
  -H 'Content-Type: application/json' \
  -d '{"carrier_name":"Supplier A Logistics","freight_minor":"700000","origin_minor":"150000","destination_minor":"100000","transit_time_hours":24,"quote_valid_until":"2030-01-05"}'
```

Money and bid IDs are decimal strings in JSON. Replaying the same key with the same canonical quote returns the original bid without another insert or event; reusing it with a changed quote returns `IDEMPOTENCY_KEY_REUSED`. Bid history defaults to price order and accepts `sort=newest`.

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

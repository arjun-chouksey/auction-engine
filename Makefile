.PHONY: up down backend-fmt backend-test backend-lint backend-integration-test frontend-install frontend-fmt frontend-lint frontend-test frontend-build test migrate-up migrate-down seed

up:
	docker compose up --build

down:
	docker compose down

backend-fmt:
	docker run --rm -v "$(CURDIR)/backend:/app" -w /app golang:1.27.3 gofmt -w .

backend-test:
	docker build --target test -t british-auction-backend-test ./backend

backend-integration-test:
	docker compose run --rm integration-test

backend-lint:
	docker run --rm -v "$(CURDIR)/backend:/app" -w /app golang:1.27.3 sh -c 'test -z "$$(gofmt -l .)" && go vet ./...'

frontend-install:
	cd frontend && npm ci

frontend-fmt:
	cd frontend && npm run format

frontend-lint:
	cd frontend && npm run lint

frontend-test:
	cd frontend && npm test

frontend-build:
	cd frontend && npm run build

test: backend-test frontend-lint frontend-test frontend-build

migrate-up:
	docker compose run --rm migrate -path=/migrations -database "$${DATABASE_URL:-postgres://auction:auction_local_only@db:5432/british_auction?sslmode=disable}" up

migrate-down:
	docker compose run --rm migrate -path=/migrations -database "$${DATABASE_URL:-postgres://auction:auction_local_only@db:5432/british_auction?sslmode=disable}" down 1

seed:
	docker compose run --rm --entrypoint /seed api


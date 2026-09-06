# TimeTracker

Organization-based time tracking, built as a portfolio project.

## Prerequisites

- Go 1.22+
- Node.js 20+ and pnpm
- Docker with Docker Compose

## Run locally

1. Copy `.env.example` to `.env`.
2. Start PostgreSQL and local email capture: `docker compose -f deploy/compose/compose.yaml up -d`.
3. From `apps/api`, apply migrations:

   ```sh
   go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.17.1 \
     -path ../../db/migrations -database "$DATABASE_URL" up
   ```

4. In `apps/api`, run `go run ./cmd/api`.
5. In `apps/web`, run `pnpm install` then `pnpm dev`.

Health endpoints: `http://localhost:8080/health/live` and `http://localhost:8080/health/ready`.
For local verification and password-reset messages, open Mailpit at `http://localhost:8025`.

Production deployments must use HTTPS and set `COOKIE_SECURE=true`.

## Layout

```text
apps/api          Go API
apps/web          React + TypeScript application
db/migrations     PostgreSQL migrations
deploy/compose    Local Docker Compose environment
docs              Requirements and design documents
```

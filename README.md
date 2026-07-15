# hello-go

pnpm monorepo with a Go API and a Vite + React client.

```
apps/
├── api/   # Go REST API (PostgreSQL)
└── web/   # Vite + React client
```

Live at **https://hello-go.pages.dev** (API on https://hello-go-hail.onrender.com).

## Setup

```bash
pnpm install
```

Requires a local PostgreSQL server. The API creates its tables on startup in the `hellodb` database:

```bash
createdb hellodb
```

## Run the server (API)

```bash
pnpm dev:api
# or
cd apps/api && go run .
```

Runs on http://localhost:8080. Environment variables:

| Variable         | Default                             |
| ---------------- | ----------------------------------- |
| `PORT`           | `8080`                              |
| `DATABASE_URL`   | `postgres://localhost:5432/hellodb` |
| `ALLOWED_ORIGIN` | `http://localhost:5173` (CORS)      |

## Run the client (web)

```bash
pnpm dev:web
# or
cd apps/web && pnpm dev
```

Runs on http://localhost:5173. The API base URL comes from `VITE_API_URL`
(see `apps/web/.env.production`), falling back to http://localhost:8080 in dev.

## Test

```bash
createdb hellodb_test   # once
pnpm test:api
```

Tests run against the `hellodb_test` database (override with `TEST_DATABASE_URL`)
and skip if PostgreSQL is unavailable.

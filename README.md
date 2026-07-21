# parole

Bun monorepo: **Alle Cinque** ("at five"), a letter-reveal trainer for beginner Italian
vocabulary, built as a Go REST API plus a Vite + ReScript (React) client. Each
round shows five Italian words on the left with their English translations
hidden on the right — only the word lengths are visible. A 🔊 button next to
each Italian word speaks its pronunciation via the browser's Italian
text-to-speech voice. Drag a keyboard key
onto the exact tile where you think that letter goes (or tap the key, then the
tile): a correct placement opens that tile plus every other occurrence of the
same character across all five words, and the key leaves the keyboard. A wrong
spot is a mistake, tracked by an on-screen counter — the fifth mistake ends
the round. Lost words are flagged to come back for review, and Retry replays
the same five right away.

It leans on research-backed techniques: retrieval practice (you recall the
English from the Italian), immediate feedback, and spaced repetition (flagged
words return three rounds later). The word pool holds 800+ essential beginner
words — nouns, verbs, adjectives, numbers, months — and every new round draws
five of them at random.

Each user gets their own progress behind cookie-session auth; letters are
checked server-side so the answers never reach the browser until the round is
over.

```
apps/
├── api/   # Go REST API (PostgreSQL, cookie-session auth)
└── web/   # Vite + ReScript (React) client
```

Live at **https://hello-go.pages.dev** (API on https://hello-go-hail.onrender.com).

## Setup

```bash
bun install
```

Requires a local PostgreSQL server. The API creates its tables on startup in the `hellodb` database:

```bash
createdb hellodb
```

## Run the server (API)

```bash
bun run dev:api
# or
cd apps/api && go run .
```

Runs on http://localhost:8080. Environment variables:

| Variable         | Default                             |
| ---------------- | ----------------------------------- |
| `PORT`           | `8080`                              |
| `DATABASE_URL`   | `postgres://localhost:5432/hellodb` |
| `ALLOWED_ORIGIN` | `http://localhost:5173` (CORS)      |

Endpoints: `POST /signup`, `POST /login`, `POST /logout` (public), and — with a
session cookie, scoped to the logged-in user:

- `GET /game` — the latest finished round, or a freshly dealt one: visiting
  mid-round abandons it, so every visit brings five new words
- `POST /game` — new round, once the current one is finished
- `GET /me` — the signed-in username and how many distinct words they have
  learned (a word counts once it appears in any won round)
- `POST /game/retry` — replay the just-finished round's five words
- `POST /game/guess` — place one letter on one tile as
  `{"guess": "a", "word": 0, "position": 2}`; a correct placement reveals
  every occurrence of that letter, a wrong one is a mistake (the fifth loses
  the round), and the full answers appear only when the round ends

The word pool (800+ Italian words with English translations) lives in
`apps/api/handlers/words.go`. New rounds pick five words server-side: words
from a round flagged at least three rounds ago come back for review first,
and the remaining slots are filled at random from words the user has not
played yet.

## Run the client (web)

```bash
bun run dev:web
# or
cd apps/web && bun run dev
```

Runs on http://localhost:5173. While editing `.res` files, keep the compiler
running in a second terminal:

```bash
cd apps/web && bun run res:watch
```

The API base URL comes from `VITE_API_URL` (see `apps/web/.env.production`),
falling back to http://localhost:8080 in dev.

## Lint & format

```bash
cd apps/web
bun run format   # rescript format -all
bun run lint     # format check + compile with warnings as errors
```

## Test

```bash
createdb hellodb_test   # once
bun run test:api
```

Tests run against the `hellodb_test` database (override with `TEST_DATABASE_URL`)
and skip if PostgreSQL is unavailable. CI runs them against a real Postgres
service container, plus the web lint and build.

## Deploy

- **Web** — Cloudflare Pages: root directory `apps/web`, build command
  `bun install && bun run build`, output `dist`.
- **API** — Render, built from `apps/api/Dockerfile` with `DATABASE_URL` and
  `ALLOWED_ORIGIN` set in the dashboard.

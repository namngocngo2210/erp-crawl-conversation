# Pancake Conversation Crawler

Crawls all Pancake (pages.fm) pages, conversations, messages, tags and customers into PostgreSQL. Runs daily at 21:00 (Asia/Ho_Chi_Minh); supports manual trigger.

## Chạy full stack (docker compose)

**Setup:**
1. Copy `.env.example` → `.env` and set:
   - `PANCAKE_ACCESS_TOKEN` (Pancake API key)
   - `JWT_SECRET` (for auth token signing)
   - `ADMIN_PASSWORD` (login password; `ADMIN_EMAIL` defaults to `admin@example.com`)

2. Run full stack:
   ```bash
   docker compose -f deploy/docker-compose.yml up --build
   ```
   Services (host ports, non-default — override via `*_HOST_PORT` env): PostgreSQL (15432) · Crawler API (18080) · Nginx + SPA (18081)

**Access:**
- **Frontend (SPA):** http://localhost:18081 → Login with `ADMIN_EMAIL`/`ADMIN_PASSWORD`
- **API:** http://localhost:18080/api/v1
  - Health check: `curl localhost:18080/healthz`
  - Trigger crawl: `curl -X POST localhost:18080/crawl`

**Architecture:** Nginx (web service) serves the React SPA and proxies `/api/*` → crawler:8080, so frontend and API share origin in production.

## Develop
- `make test` — unit tests. Set `TEST_DATABASE_URL` to run repository integration tests.
- `make migrate-up` — apply migrations.
- Architecture & sprint status: `docs/2026-06-26-pancake-crawler-design.md`.

## Behavior
- First run per page = full backfill (pgx COPY). Later runs = incremental from checkpoint (overlap window).
- De-dup invariant: one `pancake_message_id` → one row, enforced by `UNIQUE(pancake_message_id, inserted_at)` + `ON CONFLICT DO NOTHING`.

## Read API

Auth + read endpoints under `/api/v1` (JWT required except login):
- `POST /api/v1/auth/login` `{email,password}` → `{token,user}`
- `GET /api/v1/auth/me`
- `GET /api/v1/pages`
- `GET /api/v1/conversations?page_id=&cursor=&limit=`
- `GET /api/v1/conversations/:id`
- `GET /api/v1/conversations/:id/messages?direction=in|out&cursor=&limit=`

Admin seeded at startup via `ADMIN_EMAIL` / `ADMIN_PASSWORD`; set `JWT_SECRET` for token signing. In production, set `CORS_ORIGIN` to the concrete FE origin (e.g. `https://app.cong-ty.vn`) — do not leave it as `*`.

## Frontend (React SPA)

**Dev mode (separate terminal):**
```bash
cd web
npm install
npm run dev
```
Runs at http://localhost:15173 with proxy `/api` → backend :18080. Login with `ADMIN_EMAIL`/`ADMIN_PASSWORD`.

**Production build:**
```bash
npm run build  # Output: web/dist
```
(When using docker compose, Nginx serves the built SPA automatically.)

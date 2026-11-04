# Sprint 5 — Deploy (FE build + Nginx + compose + CORS) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Đóng gói FE thành ảnh tĩnh phục vụ qua Nginx (reverse-proxy `/api` → crawler), thêm service `web` vào docker-compose, cấu hình CORS/env production, cập nhật README để toàn hệ thống chạy bằng `docker compose up`.

**Architecture:** FE build tĩnh (`web/dist`) trong Docker multi-stage (node builder → nginx alpine). Nginx phục vụ SPA (fallback `index.html`) và proxy `/api` sang service `crawler:8080`. CORS backend đặt theo origin của web.

**Tech Stack:** Docker multi-stage, Nginx alpine, docker-compose.

## Global Constraints
- Multi-stage Dockerfile; ảnh runtime nhẹ (nginx:alpine).
- SPA fallback: mọi route không match file tĩnh → `index.html`.
- Production: FE gọi API qua cùng origin (`/api/v1`) nhờ Nginx proxy → CORS không cần `*` (đặt theo nhu cầu).
- Không commit `web/node_modules`, `web/dist`.
- Commit message tiếng Anh, ngắn gọn.

---

## File Structure
**Create:**
- `web/Dockerfile` — multi-stage build FE → nginx.
- `web/nginx.conf` — SPA fallback + proxy `/api`.
- `web/.dockerignore`
- `web/.env.production` — `VITE_API_BASE_URL=/api/v1`

**Modify:**
- `deploy/docker-compose.yml` — thêm service `web`; thêm env auth cho `crawler`.
- `.env.example` — thêm `JWT_SECRET`, `ADMIN_EMAIL`, `ADMIN_PASSWORD`, `CORS_ORIGIN`, `JWT_TTL`.
- `.gitignore` — đảm bảo ignore `web/node_modules`, `web/dist`.
- `README.md` — hướng dẫn chạy full stack.
- `docs/.../2026-06-29-read-api-fe-spa-design.md` — Sprint 5 progress.

---

## Task 1: Nginx config + dockerignore + production env

**Files:**
- Create: `web/nginx.conf`, `web/.dockerignore`, `web/.env.production`

- [ ] **Step 1: Nginx config (SPA fallback + API proxy)**

Create `web/nginx.conf`:
```nginx
server {
    listen       80;
    server_name  _;
    root   /usr/share/nginx/html;
    index  index.html;

    # SPA: route không match file -> index.html
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Proxy API sang service crawler trong cùng compose network
    location /api/ {
        proxy_pass         http://crawler:8080;
        proxy_set_header   Host $host;
        proxy_set_header   X-Real-IP $remote_addr;
        proxy_set_header   X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
    }

    # Cache asset tĩnh có hash
    location /assets/ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
```

- [ ] **Step 2: dockerignore**

Create `web/.dockerignore`:
```
node_modules
dist
.env
.env.local
npm-debug.log
```

- [ ] **Step 3: Production env**

Create `web/.env.production`:
```
VITE_API_BASE_URL=/api/v1
```

- [ ] **Step 4: Commit**
```bash
git add web/nginx.conf web/.dockerignore web/.env.production
git commit -m "chore: nginx config and production env for web"
```

---

## Task 2: Web Dockerfile (multi-stage)

**Files:**
- Create: `web/Dockerfile`

- [ ] **Step 1: Write Dockerfile**

Create `web/Dockerfile`:
```dockerfile
# --- build stage ---
FROM node:22-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# --- runtime stage ---
FROM nginx:alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

- [ ] **Step 2: Build the image locally**

Run (từ thư mục `web/`):
```bash
docker build -t pancake-web:local .
```
Expected: build thành công (stage builder chạy `npm run build`, runtime copy `dist`).

> Implementer note: nếu `npm ci` lỗi do thiếu `package-lock.json`, chạy `npm install` trong `web/` trước để sinh lock, commit nó, rồi build lại.

- [ ] **Step 3: Commit**
```bash
git add web/Dockerfile web/package-lock.json
git commit -m "feat: multi-stage dockerfile for web spa"
```

---

## Task 3: Compose service `web` + auth env cho crawler

**Files:**
- Modify: `deploy/docker-compose.yml`

- [ ] **Step 1: Add auth env to crawler service**

In `deploy/docker-compose.yml`, trong block `crawler.environment`, thêm:
```yaml
      JWT_SECRET: ${JWT_SECRET}
      JWT_TTL: ${JWT_TTL:-12h}
      ADMIN_EMAIL: ${ADMIN_EMAIL:-admin@local}
      ADMIN_PASSWORD: ${ADMIN_PASSWORD}
      CORS_ORIGIN: ${CORS_ORIGIN:-*}
```

- [ ] **Step 2: Add web service**

In `deploy/docker-compose.yml`, thêm service `web` (trước `volumes:`):
```yaml
  web:
    build:
      context: ../web
      dockerfile: Dockerfile
    ports: ["8081:80"]
    depends_on:
      crawler: { condition: service_started }
```

- [ ] **Step 3: Validate compose**

Run (từ thư mục gốc repo):
```bash
docker compose -f deploy/docker-compose.yml config
```
Expected: in ra config hợp lệ, không lỗi cú pháp; thấy service `web` và env JWT trong `crawler`.

- [ ] **Step 4: Commit**
```bash
git add deploy/docker-compose.yml
git commit -m "feat: web service and auth env in docker compose"
```

---

## Task 4: Env example + .gitignore

**Files:**
- Modify: `.env.example`, `.gitignore`

- [ ] **Step 1: Add auth + web vars to .env.example**

Append to `.env.example`:
```
JWT_SECRET=change-me-in-prod
JWT_TTL=12h
ADMIN_EMAIL=admin@local
ADMIN_PASSWORD=change-me
CORS_ORIGIN=*
```

> Implementer note: nếu Sprint 1 Task 12 đã thêm các biến này thì bỏ qua trùng lặp — kiểm tra trước bằng `grep JWT_SECRET .env.example`.

- [ ] **Step 2: Ensure gitignore covers web build artifacts**

Confirm `.gitignore` (gốc repo) chứa:
```
web/node_modules/
web/dist/
```
Nếu chưa có thì thêm.

- [ ] **Step 3: Commit**
```bash
git add .env.example .gitignore
git commit -m "chore: env example and gitignore for web"
```

---

## Task 5: Full-stack smoke + README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Bring up the full stack**

Run (từ gốc repo, cần `.env` có `PANCAKE_ACCESS_TOKEN`, `JWT_SECRET`, `ADMIN_PASSWORD`):
```bash
docker compose -f deploy/docker-compose.yml up --build -d
```
Expected: `postgres`, `redis`, `crawler`, `web` đều `Up`.

- [ ] **Step 2: Verify endpoints**

Run:
```bash
curl -s localhost:8080/healthz
curl -s -XPOST localhost:8080/api/v1/auth/login -H 'Content-Type: application/json' -d '{"email":"admin@local","password":"<ADMIN_PASSWORD>"}'
curl -s localhost:8081/ -o /dev/null -w "%{http_code}\n"
```
Expected: healthz `{"status":"ok"}`; login trả `{"token":...}`; web `200`.

> Implementer note: login chỉ thành công khi backend đã seed admin (Sprint 1) và migrations đã chạy. Nếu DB rỗng, `/api/v1/pages` trả `{"items":[]}` — vẫn 200.

- [ ] **Step 3: Update README full-stack section**

Replace/augment the run section in `README.md`:
```markdown
## Chạy full stack (docker compose)
1. `cp .env.example .env`, set `PANCAKE_ACCESS_TOKEN`, `JWT_SECRET`, `ADMIN_PASSWORD`.
2. `docker compose -f deploy/docker-compose.yml up --build`
3. FE: http://localhost:8081 (đăng nhập bằng `ADMIN_EMAIL`/`ADMIN_PASSWORD`).
4. API: http://localhost:8080/api/v1 · Health: `/healthz` · Trigger crawl: `POST /crawl`.

Nginx (service `web`) phục vụ SPA và proxy `/api` → `crawler:8080`, nên FE và API cùng origin trong production.
```

- [ ] **Step 4: Tear down**

Run:
```bash
docker compose -f deploy/docker-compose.yml down
```

- [ ] **Step 5: Commit**
```bash
git add README.md
git commit -m "docs: full stack run guide"
```

---

## Task 6: Progress note + final design status
- [ ] Đổi `### Sprint 5 — Deploy (tiến độ: ☐)` → `(tiến độ: ✅)` trong spec; tick box; note "đã làm / chưa làm". Cập nhật trạng thái tổng ở đầu spec thành "Hoàn thành".
```bash
git add docs/
git commit -m "docs: sprint5 progress and final status"
```

---

## Self-Review notes (đã kiểm)
- **Coverage Sprint 5:** nginx SPA+proxy ✅(T1), web Dockerfile multi-stage ✅(T2), compose service web + auth env ✅(T3), env/gitignore ✅(T4), smoke+README ✅(T5).
- **CORS:** production cùng origin nhờ Nginx proxy; `CORS_ORIGIN` vẫn cấu hình được cho trường hợp FE khác origin.
- **Dependency:** service `web` proxy tới `crawler:8080` (tên service trong compose network) — khớp `proxy_pass`.
- **Placeholder scan:** không có TODO/TBD; "Implementer note" là hướng dẫn kiểm tra trùng/lock file.
- **Lưu ý:** port web `8081:80` để không đụng `8080` của API; README phản ánh đúng.

# Thiết kế: Read API + FE SPA xem dữ liệu hội thoại đã crawl

> Ngày: 2026-06-29
> Trạng thái: Hoàn thành — cả 5 sprint đã implement & verify (smoke pass)
> Liên quan: `docs/2026-06-26-pancake-crawler-design.md` (crawler nền)

## 1. Mục tiêu & phạm vi

Xây dựng giao diện để **xem được dữ liệu hội thoại Pancake đã crawl** vào PostgreSQL. Gồm 2 phần:

1. **Read API** — mở rộng service Go hiện có (modular monolith) thêm module đọc dữ liệu (chưa tồn tại; HTTP hiện chỉ có `/healthz`, `/metrics`, `/crawl`).
2. **FE SPA** — ứng dụng React + Vite + TypeScript hiển thị dữ liệu.

Ba nhóm chức năng:
- **Inbox**: duyệt page → conversations → đọc luồng tin nhắn.
- **Search**: tìm kiếm tin nhắn, khách hàng, lọc theo nhiều tiêu chí.
- **Dashboard**: thống kê tổng quan + giám sát tình trạng crawl.

Dữ liệu chứa **PII khách hàng** (SĐT, email, nội dung chat) → bắt buộc có xác thực.

### Ngoài phạm vi (YAGNI)
- Không sửa/ghi dữ liệu hội thoại từ FE (read-only; chỉ trigger crawl đã có sẵn).
- Không realtime/websocket.
- Không phân quyền chi tiết theo page ở giai đoạn này (chỉ cần đăng nhập là xem được; `role` để dành mở rộng).
- Không export CSV ở giai đoạn này.

## 2. Quyết định kiến trúc

| Yếu tố | Lựa chọn | Lý do |
|---|---|---|
| Mô hình | **Mở rộng monolith hiện có** (Hướng A) | Vừa đủ quy mô; tái dùng entity/repository; vẫn giữ đường tách microservice sau nhờ đã tách layer |
| FE stack | **React 19 + Vite + TypeScript** (SPA thuần) | Theo yêu cầu "SPA"; nhẹ, build nhanh |
| UI | **shadcn/ui** (cài qua CLI) + **lucide-react**, light theme tông xanh da trời | Theo chuẩn dự án (CLAUDE.md) |
| Data fetching | **TanStack Query** | Cache + infinite scroll khớp keyset pagination |
| Routing | **TanStack Router** | Type-safe routing cho SPA |
| HTTP client | **axios** + interceptor JWT | Tự gắn token, xử lý 401 tập trung |
| Charts | **recharts** (qua shadcn chart) | Dashboard |
| Auth | **JWT (HS256)** + bảng `users` (bcrypt) | Đủ an toàn nội bộ, không phụ thuộc hạ tầng ngoài |
| Pagination | **Keyset (cursor)** | Dữ liệu chục triệu message; OFFSET chậm |
| Search | **Postgres FTS** (`tsvector` + GIN index) | Tránh `ILIKE %...%` quét toàn bảng |
| Query | **SQLC** cho query phức tạp (search, stats, keyset), **GORM** cho list đơn giản | Theo chuẩn dự án |

## 3. Cấu trúc thư mục

### 3.1 Backend (mở rộng `/internal`)
```
/internal
  /business/read         # ReadService: PageQuery, ConversationQuery, MessageQuery,
                         #   SearchQuery, StatsQuery; AuthService (login, verify)
  /repository/read       # read-only queries (SQLC: search/stats/keyset; GORM: list đơn giản)
  /transport/http/api    # REST handlers + JWT middleware + CORS
  /dto/response          # response DTO (KHÔNG trả raw jsonb/PII thừa)
/db/query                # *.sql cho SQLC (search, aggregation, pagination)
/db/migrations           # thêm: users; tsvector + GIN cho messages.content
```
Hướng phụ thuộc giữ nguyên: `transport → business → repository → entity`. Module read độc lập với crawler, không phụ thuộc ngược.

### 3.2 Frontend (`/web`)
```
/web
  /src
    /app            # router config (TanStack Router), root layout, providers
    /features
      /auth         # login page, useAuth, token store
      /pages-list   # chọn page
      /inbox        # 3-pane: conversation list | message thread | customer panel
      /search       # search tin nhắn + kết quả
      /dashboard    # stats, charts, crawl-runs
    /components/ui  # shadcn/ui (cài qua CLI, không tự code)
    /services       # api client (axios + interceptor JWT, xử lý 401)
    /hooks          # TanStack Query hooks (useConversations, useMessages, ...)
    /types          # types khớp DTO backend
    /lib            # utils, formatters (date, phone)
  vite.config.ts    # proxy /api -> backend khi dev
  .env              # VITE_API_BASE_URL
```

## 4. API (prefix `/api/v1`; mọi route trừ `/auth/login` cần JWT)

| Method | Path | Mục đích | Ghi chú |
|---|---|---|---|
| POST | `/auth/login` | Đăng nhập → JWT | body `{email, password}` → `{token, user}` |
| GET | `/auth/me` | User hiện tại | từ JWT |
| GET | `/pages` | Danh sách page (+ số conversation) | |
| GET | `/conversations` | List theo `page_id`, filter tag/customer/time | keyset, sort `updated_at DESC` |
| GET | `/conversations/:id` | Chi tiết hội thoại + customer + tags | |
| GET | `/conversations/:id/messages` | Tin nhắn trong hội thoại | keyset theo `inserted_at`; filter `direction=in\|out` |
| GET | `/search/messages` | Full-text search nội dung | filter page/time/`direction`; FTS |
| GET | `/customers` | List/search khách hàng | theo tên/SĐT/email |
| GET | `/stats/overview` | Tổng page/conv/msg theo ngày/page/tag | aggregation |
| GET | `/stats/crawl-runs` | Lịch sử & trạng thái crawl | từ bảng `crawl_runs` |

### 4.1 Quy ước
- **Keyset pagination**: response `{ items: [...], next_cursor: string|null }`. Cursor mã hoá `(sort_key, id)`. Tận dụng index sẵn có `(conversation_id, inserted_at DESC)` và `(page_id, updated_at DESC)`.
- **Filter direction**: query param `?direction=in|out`, mặc định trả cả hai. Cột `direction` đã có trong `messages` → không cần migration.
- DTO response chỉ phơi field cần thiết; **không** đổ `raw` jsonb ra FE.
- Lỗi trả JSON nhất quán `{ error: { code, message } }`.

## 5. Auth (JWT đơn giản)

- Migration `users`: `id`, `email` (UK), `password_hash` (bcrypt), `name`, `role` (default `viewer`), `created_at`, `updated_at`.
- Seed 1 admin lúc khởi động từ env (`ADMIN_EMAIL`, `ADMIN_PASSWORD`) nếu chưa tồn tại.
- `POST /auth/login`: verify bcrypt → JWT HS256, TTL ~12h, secret `JWT_SECRET`.
- Middleware Gin verify JWT cho `/api/v1/*` (trừ `/auth/login`).
- FE: lưu token (localStorage), axios interceptor gắn `Authorization: Bearer`; 401 → xoá token → redirect `/login`.
- `role` để dành phân quyền tương lai; giai đoạn này đăng nhập là đủ quyền xem.

## 6. Luồng dữ liệu (ví dụ inbox)
```
FE (TanStack Query useInfiniteQuery)
   │  GET /api/v1/conversations?page_id=X&cursor=…
   ▼
Gin handler  → JWT middleware
   → ConversationQuery (business)
   → read repo (SQLC keyset)
   → DTO {items, next_cursor}
   ▼
FE render danh sách + prefetch trang kế
```
Tương tự cho messages (keyset theo `inserted_at`, filter direction), search (FTS), stats (aggregation).

## 7. Search (Postgres FTS)
- Migration: thêm cột `content_tsv tsvector` (generated hoặc cập nhật qua trigger) + **GIN index** trên `messages`.
- `GET /search/messages`: `to_tsquery`/`plainto_tsquery` trên `content_tsv`, filter thêm `page_id`, khoảng thời gian, `direction`. Keyset theo `inserted_at`.
- Lưu ý: bảng `messages` partition theo tháng → index GIN tạo trên bảng cha, áp cho mọi partition.

## 8. Dev / Deploy
- **Dev**: Vite proxy `/api` → `http://localhost:8080`; chạy song song crawler service.
- **Prod**: build FE tĩnh; phục vụ qua **Nginx** (hoặc embed vào Go binary bằng `embed`). Thêm service `web` vào `deploy/docker-compose.yml`. Bật **CORS** cho origin FE.
- Biến môi trường thêm: `JWT_SECRET`, `ADMIN_EMAIL`, `ADMIN_PASSWORD`, (FE) `VITE_API_BASE_URL`.

## 9. Test
- **Business**: unit test cho `AuthService`, `ConversationQuery`, `MessageQuery`, `SearchQuery`, `StatsQuery` (mock repo qua interface).
- **Transport**: endpoint test (httptest) cho auth (login/401), keyset pagination, filter direction.
- **Repository**: integration test (Postgres) cho keyset + FTS + partition routing.
- **FE**: test component chính (login flow, render message thread, infinite scroll hook) ở mức hợp lý.

## 10. Phân chia Sprint

> Mỗi sprint kết thúc note "đã làm / chưa làm". Cập nhật trạng thái tại đây.

### Sprint 1 — Read API nền (tiến độ: ✅)
- [x] Migration `users`; seed admin từ env.
- [x] Auth: `POST /auth/login`, `GET /auth/me`, JWT middleware.
- [x] Endpoint `GET /pages`, `/conversations`, `/conversations/:id`, `/conversations/:id/messages` (keyset + filter direction).
- [x] DTO response; CORS.
- [x] Unit test business + endpoint test.
- **Output**: API đọc hội thoại chạy được, có auth.

**Đã làm:** Users migration + seed admin (T1, T11); auth service + login/me endpoints + JWT middleware (T7, T9, T10); read endpoints với keyset pagination + direction filter (T5, T8, T10); response DTOs + CORS (T6, T9); business layer tests + endpoint tests (T8, T10).

**Chưa làm:** Sprint 1 hoàn thành đủ scope, không có work item chưa làm.

### Sprint 2 — FE khung + Inbox (tiến độ: ✅)
- [x] Scaffold Vite + React + TS; cài shadcn/ui, lucide-react, TanStack Query/Router, axios.
- [x] Login page + token store + interceptor (xử lý 401).
- [x] Màn Inbox 3-pane: conversation list (infinite scroll), message thread (bubble in/out, filter direction), customer panel + tags.
- **Output**: đăng nhập + duyệt/đọc hội thoại end-to-end.

**Đã làm:** Scaffold & config Vite/React/TS + shadcn/ui theme xanh (T1, T2); types & utils shared (T3); token store + axios 401 interceptor (T4); API service + infinite scroll hooks (T5); login page + router guard (T6); inbox 3-pane UI với message bubbles, direction filter, tags (T7).

**Chưa làm:** Sprint 2 hoàn thành đủ scope, không có work item chưa làm.

### Sprint 3 — Search (tiến độ: ✅)
- [x] Migration `tsvector` + GIN index trên `messages`.
- [x] `GET /search/messages`, `GET /customers` + SQLC queries.
- [x] Màn Search + filter (page, thời gian, direction), click nhảy vào hội thoại.
- **Output**: tìm kiếm tin nhắn/khách hàng.

**Đã làm:** FTS migration với tsvector + GIN index (T1); search messages & customers endpoints với SQLC (T2, T3, T4); FE search service + hooks (T5); search page + filter UI + route + nav (T6).

**Chưa làm:** Sprint 3 hoàn thành đủ scope, không có work item chưa làm.

### Sprint 4 — Dashboard (tiến độ: ✅)
- [x] `GET /stats/overview`, `GET /stats/crawl-runs` + SQLC aggregation.
- [x] Màn Dashboard: card tổng quan + charts (message theo ngày/page/tag) + bảng crawl-runs.
- **Output**: thống kê + giám sát crawl.

**Đã làm:** Stats repository aggregation queries (T1); stats service + DTO response types (T2); `/stats/overview` and `/stats/crawl-runs` endpoints + DI wiring (T3); FE stats API service + React hooks (T4); dashboard page with overview cards, charts (daily/page/tag), and crawl-runs table (T5).

**Chưa làm:** Sprint 4 hoàn thành đủ scope, không có work item chưa làm.

### Sprint 5 — Deploy (tiến độ: ✅)
- [x] Build FE tĩnh; Nginx (hoặc embed); service `web` trong docker-compose.
- [x] CORS prod, env mẫu, README vận hành FE + API.
- **Output**: toàn hệ thống chạy bằng docker.

**Đã làm:** Nginx config SPA proxy + fallback (T1); web Dockerfile multi-stage với Node builder + distroless (T2); compose service `web` với auth env variables + port mapping (T3); .env template + .gitignore update (T4); full-stack docker-compose smoke test verified + README full-stack guide (T5).

**Chưa làm:** Sprint 5 hoàn thành đủ scope, không có work item chưa làm.

## 11. Rủi ro & lưu ý
- GIN index trên bảng partition lớn → tạo index có thể nặng lần đầu; cân nhắc `CREATE INDEX CONCURRENTLY` hoặc tạo theo partition.
- Keyset cursor cần ổn định khi có bản ghi cùng timestamp → cursor gồm `(sort_key, id)` để phá hoà.
- Read API và crawler chung process → tải đọc lớn có thể ảnh hưởng crawl; nếu cần, tách replica sau (đã tách layer nên dễ).
- Không trả `raw` jsonb để tránh lộ PII thừa và giảm payload.

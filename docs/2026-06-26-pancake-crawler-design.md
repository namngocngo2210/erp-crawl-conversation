# Thiết kế hệ thống: Pancake Conversation Crawler

> Ngày: 2026-06-26
> Trạng thái: Spec đã chốt yêu cầu, chờ review trước khi lập plan thực thi

## 1. Mục tiêu & phạm vi

Xây dựng một service Go (modular monolith) crawl toàn bộ dữ liệu hội thoại từ Pancake (pages.fm) về PostgreSQL:

- Toàn bộ **page** mà tài khoản quản lý.
- Toàn bộ **conversation** của mỗi page.
- Toàn bộ **message** trong mỗi conversation.
- **Tag** (định nghĩa của page + tag gán cho conversation).
- **Thông tin khách hàng** (customer) gắn với conversation.

Chế độ chạy:

- **Backfill**: lần đầu kéo toàn bộ lịch sử.
- **Incremental**: các lần sau chỉ kéo dữ liệu thay đổi kể từ checkpoint.
- **Lịch**: chạy tự động **mỗi ngày 21:00** giờ `Asia/Ho_Chi_Minh`, cộng tùy chọn trigger thủ công.

Không dùng webhook / không realtime.

### Bất biến hệ thống (system invariants)
1. **Một `pancake_message_id` chỉ tồn tại đúng 1 dòng trong DB**, bất kể crawl bao nhiêu lần (chống trùng tuyệt đối).
2. Crash giữa chừng → lần chạy sau **resume từ checkpoint**, không kéo lại từ đầu.
3. Tôn trọng **rate limit** của Pancake theo từng page.

## 2. Quyết định kiến trúc

| Yếu tố | Lựa chọn | Lý do |
|---|---|---|
| Ngôn ngữ | Go 1.25+, clean architecture, interface-driven | Tận dụng goroutines, idiomatic |
| DI | `samber/do` | Theo chuẩn dự án |
| Mô hình triển khai | **Modular monolith** (1 binary, worker nội bộ) | Đúng quy mô, scale bằng replica |
| HTTP framework | **Gin** | `/healthz`, `/metrics`, để mở rộng API tra cứu sau |
| ORM | **GORM** | Theo yêu cầu; CRUD + upsert idempotent |
| Bulk load | **pgx `CopyFrom`** cho backfill | Throughput cao cho lô lớn |
| Database | PostgreSQL; bảng `messages` **partition RANGE theo tháng** | Quy mô ~chục triệu message |
| Cache | **Redis** chỉ lưu `page_access_token` (persist + TTL) | Theo yêu cầu |
| Scheduler | `robfig/cron` (timezone-aware) | Cron 21:00 hằng ngày |
| Điều phối job | **Channel in-process** (không Redis Streams) | 1 batch/ngày, durability đã có checkpoint Postgres |
| Migration | `goose` (hoặc `golang-migrate`) | Quản lý schema/partition |
| Deploy | Docker multi-stage + docker-compose (app, postgres, redis) | Theo yêu cầu |
| Observability | `slog` structured log, Prometheus `/metrics`, `/healthz` | Vận hành |

Hạ tầng runtime: **app + PostgreSQL + Redis**.

## 3. Cấu trúc thư mục (clean architecture)

```
/cmd/crawler                 # main: bootstrap DI, start cron + http server
/internal
  /config                    # load & validate env (12-factor)
  /entity                    # domain: Page, Conversation, Message, Customer, Tag, CrawlCheckpoint
  /dto                       # struct map response Pancake API
  /pancake                   # API client: http, rate-limit/page, retry/backoff, pagination
  /token                     # PageTokenManager: generate/refresh, cache Redis, mutex/page
  /repository                # GORM (+ pgx pool cho COPY); 1 repo/aggregate
  /business                  # usecase: CrawlService, BackfillService, IncrementalService, TokenService
  /transport
    /http                    # Gin router: /healthz, /metrics
    /scheduler               # cron 21:00 + manual trigger
    /worker                  # worker pool (goroutine + channel)
/db
  /migrations                # goose
/docs
/deploy                      # Dockerfile, docker-compose.yml, .env.example
```

Hướng phụ thuộc: `transport → business → repository → entity`. Business chỉ phụ thuộc **interface** của `pancake`, `token`, `repository`.

## 4. Mô hình dữ liệu (PostgreSQL)

### 4.1 Bảng

`pages`
- `id` (PK, bigint identity), `pancake_page_id` (text, **UK**), `platform`, `name`, `status`, `last_synced_at`, `raw` (jsonb), `created_at`, `updated_at`.

`customers`
- `id` (PK), `page_id` (FK), `pancake_customer_id` (text), `name`, `phone`, `email`, `fb_id`, `gender`, `raw` (jsonb), timestamps.
- **UK** `(page_id, pancake_customer_id)`.

`tags`
- `id` (PK), `page_id` (FK), `pancake_tag_id` (text), `name`, `color`, timestamps.
- **UK** `(page_id, pancake_tag_id)`.

`conversations`
- `id` (PK), `page_id` (FK), `pancake_conversation_id` (text), `customer_id` (FK nullable), `type`, `snippet`, `message_count`, `last_message_at`, `updated_at` (từ Pancake), `raw` (jsonb), timestamps.
- **UK** `(page_id, pancake_conversation_id)`.
- **Index** `(page_id, updated_at DESC)` — phục vụ incremental.
- `last_message_at` dùng làm checkpoint cấp conversation.

`conversation_tags` (N-N)
- `conversation_id` (FK), `tag_id` (FK). PK `(conversation_id, tag_id)`.

`messages` — **PARTITION BY RANGE (`inserted_at`)** theo tháng
- `id` (bigint identity), `page_id`, `conversation_id`, `pancake_message_id` (text), `sender_id`, `direction` (`in`/`out`), `type`, `content` (text), `attachments` (jsonb), `inserted_at` (timestamptz, **từ Pancake**), `raw` (jsonb), `created_at`.
- **UK** `(pancake_message_id, inserted_at)` — bắt buộc chứa partition key; là conflict target chống trùng.
- **Index** `(conversation_id, inserted_at DESC)` — truy vấn theo hội thoại.
- **DEFAULT partition** làm lưới an toàn (không rớt dữ liệu nếu thiếu partition tháng).

`crawl_runs` — lịch sử mỗi lần chạy
- `id` (PK), `started_at`, `finished_at`, `trigger` (`cron`/`manual`), `status` (`running`/`success`/`failed`), `stats` (jsonb: số page/conv/msg), `error`.

`crawl_checkpoints` — vị trí đồng bộ theo page
- `page_id` (PK/FK), `job_type` (`conversations`), `cursor` (timestamptz = `updated_at` lớn nhất đã xử lý), `status`, `last_run_at`.

### 4.2 Quản lý partition
- Hàm `ensurePartition(month)`: `CREATE TABLE IF NOT EXISTS messages_yYYYYmMM PARTITION OF messages FOR VALUES FROM (...) TO (...)`, cache set tháng đã tạo trong process để tránh DDL lặp.
- Gọi `ensurePartition` trước mỗi lần ghi message vào tháng mới.
- DEFAULT partition + job bảo trì định kỳ dọn về đúng partition (phòng hờ).

## 5. Chống crawl trùng tin nhắn (2 lớp)

**Lớp 1 — DB (chống trùng tuyệt đối):** `UNIQUE (pancake_message_id, inserted_at)` + `ON CONFLICT DO NOTHING`. Mọi nguồn gây trùng (poll chồng lấn, retry, resume) đều bị loại. `inserted_at` bất biến (từ Pancake) → 1 message luôn rơi đúng 1 partition, đúng 1 key.

**Lớp 2 — Checkpoint (giảm fetch thừa):** mỗi conversation lưu `last_message_at`; incremental chỉ kéo message `inserted_at > last_message_at`. Dùng **overlap nhỏ** (kéo lùi vài giây) để zero-miss ở ranh giới cùng timestamp; phần thừa bị Lớp 1 loại.

## 6. Ghi message — 2 đường sau cùng một interface

```go
type MessageRepository interface {
    UpsertBatch(ctx context.Context, msgs []entity.Message) error              // incremental: lô nhỏ
    BulkLoad(ctx context.Context, msgs []entity.Message) (inserted int64, err error) // backfill: lô lớn
}
```

- **`UpsertBatch`** (GORM): `Clauses(clause.OnConflict{Columns: pancake_message_id+inserted_at, DoNothing: true}).CreateInBatches(msgs, 500)`.
- **`BulkLoad`** (pgx, trong 1 transaction):
  ```sql
  CREATE TEMP TABLE _stg_messages (LIKE messages INCLUDING DEFAULTS) ON COMMIT DROP;
  -- pgx CopyFrom -> _stg_messages
  INSERT INTO messages
  SELECT DISTINCT ON (pancake_message_id, inserted_at) *
  FROM _stg_messages
  ORDER BY pancake_message_id, inserted_at
  ON CONFLICT (pancake_message_id, inserted_at) DO NOTHING;
  ```
  TEMP table `ON COMMIT DROP` → mỗi worker/transaction độc lập, không tranh chấp staging.

Repository giữ song song `*gorm.DB` và `*pgxpool.Pool` (cùng Postgres), ẩn hoàn toàn sau interface — business không biết khác biệt.

## 7. Mô hình concurrency

```
Cron 21:00 (Asia/Ho_Chi_Minh)
   │  (hoặc manual trigger)
   ▼
CrawlService.Run()  ──> tạo crawl_run
   │
   ├─ TokenManager: đảm bảo page_access_token (Redis cache, mutex/page, refresh khi 401)
   │
   ├─ fan-out: mỗi page -> job vào channel
   │
   ▼
Worker Pool (N goroutine consumer)
   │   - rate-limit/page (x/time/rate token bucket keyed by page_id)
   │   - backfill: phân trang conversations -> messages -> BulkLoad (pgx COPY)
   │   - incremental: lọc theo checkpoint -> UpsertBatch (GORM)
   │   - upsert customer + tag + conversation_tags
   │   - cập nhật crawl_checkpoints
   ▼
PostgreSQL
```

- Concurrency theo page (mỗi page 1 luồng xử lý tuần tự để giữ thứ tự phân trang & rate limit), song song giữa các page tới giới hạn pool.
- Tránh N+1: batch theo conversation, prefetch tag/customer theo page, `CreateInBatches`/COPY.
- Resilience: exponential backoff + retry có giới hạn; lỗi 1 page không làm hỏng page khác; ghi lỗi vào `crawl_runs.error`.

## 8. Tích hợp Pancake API (GIẢ ĐỊNH — verify ở Sprint 0)

> Cần xác minh bằng credential thật trước khi code client.

- Base URL: `https://pages.fm/api/public_api/v1/` (hoặc `pancake.vn/api/public_api/v2/`).
- Xác thực: `access_token` cấp tài khoản → `POST /pages/{page_id}/generate_page_access_token` lấy `page_access_token` cho từng endpoint của page.
- `GET /pages` — danh sách page.
- `GET /pages/{page_id}/conversations` — phân trang, sort theo `updated_at`, hỗ trợ filter thời gian (`since`/`until`).
- `GET /pages/{page_id}/conversations/{conv_id}/messages` — phân trang.
- `GET /pages/{page_id}/tags` — tag của page; tag gán cho conversation nằm trong payload conversation.
- Customer info: nằm trong payload conversation (`customers`) hoặc endpoint riêng.
- Rate limit: theo `page_access_token` — cần token bucket/page (giá trị cụ thể verify ở Sprint 0).

**Client xử lý:** phân trang tự động, retry/backoff (429/5xx), timeout, map sang DTO.

## 9. Cấu hình (env)

```
PANCAKE_ACCESS_TOKEN=...
DATABASE_URL=postgres://...
REDIS_URL=redis://...
CRON_SCHEDULE=0 21 * * *
CRON_TIMEZONE=Asia/Ho_Chi_Minh
WORKER_POOL_SIZE=8
RATE_LIMIT_PER_PAGE_RPS=...
TOKEN_TTL=...
LOG_LEVEL=info
```

## 10. Deploy

- **Dockerfile** multi-stage: builder (go 1.25) → distroless/alpine runtime.
- **docker-compose**: `crawler`, `postgres:16`, `redis:7`. Volume cho Postgres; healthcheck.
- Migration chạy lúc khởi động (goose up) hoặc job riêng.

## 11. Test

- Unit test business layer: `CrawlService`, `IncrementalService` (mock pancake client + repo qua interface).
- Test dedup: chạy `UpsertBatch`/`BulkLoad` 2 lần cùng dữ liệu → đảm bảo số dòng không đổi (kiểm chứng bất biến #1).
- Integration test repository: Postgres test container; verify partition routing + ON CONFLICT.
- Test rate limiter & retry/backoff.

---

## 12. Phân chia Sprint

> Mỗi sprint kết thúc cần note "đã làm / chưa làm". Cập nhật trạng thái ngay tại đây.

### Sprint 0 — Khảo sát API & khởi tạo (tiến độ: ☐)
- [ ] Verify endpoint, auth, pagination, filter thời gian, rate limit, format response của Pancake bằng credential thật.
- [ ] Khởi tạo repo: go module, cấu trúc thư mục, `samber/do`, config loader, `slog`.
- [ ] docker-compose (postgres, redis) cho môi trường dev.
- **Output:** tài liệu API thực tế + skeleton chạy được `/healthz`.

### Sprint 1 — Schema & migration (tiến độ: ☐)
- [ ] Migration goose: `pages`, `customers`, `tags`, `conversations`, `conversation_tags`, `messages` (partitioned + DEFAULT), `crawl_runs`, `crawl_checkpoints`.
- [ ] Hàm `ensurePartition` + index theo thiết kế.
- [ ] Entity + GORM model + pgx pool.
- **Output:** DB sẵn sàng, migrate up/down chạy được.

### Sprint 2 — Token & Pancake client (tiến độ: ☐)
- [ ] `PageTokenManager`: generate/refresh, cache Redis, mutex/page, auto-refresh khi 401.
- [ ] `pancake` client: list pages/conversations/messages/tags, phân trang, retry/backoff, rate-limit/page.
- [ ] Unit test client (httptest) + token manager.
- **Output:** gọi được API thật, lấy 1 page mẫu.

### Sprint 3 — Repository & ghi dữ liệu (tiến độ: ☐)
- [ ] `MessageRepository` (`UpsertBatch` GORM + `BulkLoad` pgx COPY/staging).
- [ ] Repo cho page/conversation/customer/tag (upsert idempotent).
- [ ] Test dedup (bất biến #1) + integration test partition.
- **Output:** ghi/đọc dữ liệu idempotent, chống trùng đã kiểm chứng.

### Sprint 4 — CrawlService (backfill + incremental) (tiến độ: ☐)
- [ ] Worker pool (goroutine + channel), fan-out theo page.
- [ ] `BackfillService` (COPY) + `IncrementalService` (checkpoint + overlap).
- [ ] `crawl_runs`/`crawl_checkpoints` + resume sau crash.
- [ ] Unit test business layer.
- **Output:** chạy backfill 1 page end-to-end + incremental.

### Sprint 5 — Scheduler, observability, deploy (tiến độ: ☐)
- [ ] Cron 21:00 `Asia/Ho_Chi_Minh` + manual trigger.
- [ ] Prometheus `/metrics`, structured log, graceful shutdown.
- [ ] Dockerfile multi-stage + docker-compose hoàn chỉnh + `.env.example`.
- [ ] README vận hành.
- **Output:** service chạy tự động hằng ngày, deploy bằng docker.

---

## 13. Rủi ro & lưu ý
- API Pancake thực tế có thể khác giả định ở mục 8 → Sprint 0 phải verify trước.
- Backfill lần đầu nặng → giới hạn concurrency + rate limit để tránh bị Pancake throttle/ban.
- Đảm bảo `inserted_at` luôn có giá trị hợp lệ (nếu Pancake trả thiếu → fallback an toàn để không vỡ partition routing).

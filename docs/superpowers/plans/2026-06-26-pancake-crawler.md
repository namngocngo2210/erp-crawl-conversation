# Pancake Conversation Crawler Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go modular-monolith service that crawls all Pancake (pages.fm) pages, conversations, messages, tags and customers into PostgreSQL, running automatically once per day at 21:00 Asia/Ho_Chi_Minh with idempotent de-duplication.

**Architecture:** Single Go binary, clean architecture (`transport → business → repository → entity`). Dependencies are wired with the **`samber/do/v2`** DI container in `main` (providers register each component; the container resolves the graph). A cron scheduler triggers a daily crawl run; a worker pool (goroutines + channels) processes one job per page, rate-limited per page. Backfill uses pgx `COPY` into a temp staging table; incremental uses GORM upsert. Both paths target a `messages` table partitioned by month and de-duplicate on `(pancake_message_id, inserted_at)`. Redis caches per-page access tokens.

**Tech Stack:** Go 1.25, Gin, GORM (`gorm.io/driver/postgres`), pgx/v5 (`pgxpool`, `CopyFrom`), `samber/do/v2` (DI), `robfig/cron/v3`, `redis/go-redis/v9`, `golang.org/x/time/rate`, `goose` (migrations), `log/slog`, `prometheus/client_golang`, `stretchr/testify`, `alicebob/miniredis` (token tests).

## Global Constraints

- Go version floor: **1.25** (set in `go.mod`).
- Clean architecture layering: `transport → business → repository → entity`. Business depends only on **interfaces** of `pancake`, `token`, `repository`.
- **DI:** `samber/do/v2` container in `main`. Each component (DB, redis client, rate limiter, pancake client, token manager, repositories, page processor, crawl service) is registered as a provider; `main` invokes the top-level `business.CrawlService` and `scheduler.Scheduler` from the container. Provider functions stay in `main` (composition root) — the inner layers remain framework-free and depend only on interfaces.
- ORM: **GORM** for all CRUD/upsert; **pgx `CopyFrom`** only for backfill bulk load. Both hidden behind the repository interface.
- Database: **PostgreSQL 16**; `messages` table **PARTITION BY RANGE (`inserted_at`)** monthly, plus one DEFAULT partition.
- Message de-dup key (system invariant): **`UNIQUE (pancake_message_id, inserted_at)`** + `ON CONFLICT DO NOTHING`. One `pancake_message_id` → exactly one row, regardless of crawl count.
- Redis stores **only** `page_access_token` (cache + TTL). No Redis Streams.
- Job dispatch: **in-process channels**. Durability via `crawl_checkpoints` / `crawl_runs` in Postgres.
- Schedule: cron `0 21 * * *`, timezone `Asia/Ho_Chi_Minh`. Plus manual trigger.
- Secrets (`PANCAKE_ACCESS_TOKEN`, DB/Redis URLs) come from env only. `.env` is gitignored; `.env.example` holds placeholders only.
- Deploy: Docker multi-stage + docker-compose (`crawler`, `postgres:16`, `redis:7`).
- Commit messages in English, short.
- **API-dependent tasks (Sprint 2 client, Sprint 4 crawl mapping)** are written against the Pancake API assumptions in spec §8. Sprint 0 Task 0.5 verifies them; adjust DTO field names / endpoints if verification differs, keeping the documented interfaces stable.

---

## File Structure

```
/go.mod /go.sum
/cmd/crawler/main.go                 # bootstrap DI, start cron + http server, graceful shutdown
/internal/config/config.go           # env load + validate
/internal/config/config_test.go
/internal/entity/*.go                # domain structs (Page, Conversation, Message, Customer, Tag, CrawlRun, CrawlCheckpoint)
/internal/dto/pancake.go             # Pancake API response structs
/internal/dto/mapper.go              # DTO -> entity mappers
/internal/dto/mapper_test.go
/internal/pancake/client.go          # interface + http impl (list pages/conversations/messages/tags)
/internal/pancake/client_test.go     # httptest-based
/internal/pancake/ratelimit.go       # per-page token bucket
/internal/token/manager.go           # PageTokenManager interface + redis-backed impl
/internal/token/manager_test.go
/internal/repository/db.go           # gorm.DB + pgxpool construction, partition helper
/internal/repository/message_repo.go # MessageRepository (UpsertBatch GORM + BulkLoad pgx COPY)
/internal/repository/message_repo_test.go
/internal/repository/page_repo.go    # PageRepository, ConversationRepository, etc.
/internal/repository/checkpoint_repo.go
/internal/repository/*_test.go
/internal/business/token_service.go
/internal/business/crawl_service.go  # orchestrates a run
/internal/business/backfill.go       # per-page backfill
/internal/business/incremental.go    # per-page incremental
/internal/business/*_test.go
/internal/transport/http/server.go   # Gin: /healthz, /metrics
/internal/transport/scheduler/scheduler.go  # cron + manual trigger
/internal/transport/worker/pool.go   # worker pool
/internal/observability/metrics.go   # prometheus collectors
/db/migrations/*.sql                 # goose
/deploy/Dockerfile
/deploy/docker-compose.yml
/.env.example /.gitignore /README.md
```

---

## Sprint 0 — Scaffold, config, docker-compose, API verification

**Goal of sprint:** A buildable repo with config loading, a Gin `/healthz` server, dev docker-compose, and a verified, documented Pancake API contract.

### Task 0.1: Initialize Go module and tooling

**Files:**
- Create: `go.mod`, `.gitignore`, `.env.example`, `Makefile`

**Interfaces:**
- Produces: module path `github.com/desi/crawl-conversations` used by all imports.

- [ ] **Step 1: Init module**

Run:
```bash
go mod init github.com/desi/crawl-conversations
go mod edit -go=1.25
```

- [ ] **Step 2: Create `.gitignore`**

```gitignore
/bin/
*.exe
.env
.env.local
/tmp/
coverage.out
```

- [ ] **Step 3: Create `.env.example`**

```dotenv
PANCAKE_ACCESS_TOKEN=replace_me
PANCAKE_BASE_URL=https://pages.fm/api/public_api/v1
DATABASE_URL=postgres://crawler:crawler@localhost:5432/crawler?sslmode=disable
REDIS_URL=redis://localhost:6379/0
CRON_SCHEDULE=0 21 * * *
CRON_TIMEZONE=Asia/Ho_Chi_Minh
WORKER_POOL_SIZE=8
RATE_LIMIT_PER_PAGE_RPS=5
RATE_LIMIT_PER_PAGE_BURST=5
TOKEN_TTL=3600s
HTTP_ADDR=:8080
LOG_LEVEL=info
```

- [ ] **Step 4: Create `Makefile`**

```makefile
.PHONY: build test lint run migrate-up
build:
	go build -o bin/crawler ./cmd/crawler
test:
	go test ./... -race -count=1
lint:
	golangci-lint run
run:
	go run ./cmd/crawler
migrate-up:
	goose -dir db/migrations postgres "$$DATABASE_URL" up
```

- [ ] **Step 5: Commit**

```bash
git add go.mod .gitignore .env.example Makefile
git commit -m "chore: init go module and tooling"
```

### Task 0.2: Config loader

**Files:**
- Create: `internal/config/config.go`, `internal/config/config_test.go`

**Interfaces:**
- Produces:
  - `type Config struct { PancakeAccessToken, PancakeBaseURL, DatabaseURL, RedisURL, CronSchedule, CronTimezone, HTTPAddr, LogLevel string; WorkerPoolSize int; RateLimitPerPageRPS float64; RateLimitPerPageBurst int; TokenTTL time.Duration }`
  - `func Load() (Config, error)` — reads env, validates required fields, returns typed config.

- [ ] **Step 1: Write the failing test**

`internal/config/config_test.go`:
```go
package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoad_Valid(t *testing.T) {
	t.Setenv("PANCAKE_ACCESS_TOKEN", "tok")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("REDIS_URL", "redis://x")
	t.Setenv("WORKER_POOL_SIZE", "4")
	t.Setenv("RATE_LIMIT_PER_PAGE_RPS", "5")
	t.Setenv("TOKEN_TTL", "30m")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "tok", cfg.PancakeAccessToken)
	require.Equal(t, 4, cfg.WorkerPoolSize)
	require.Equal(t, 30*time.Minute, cfg.TokenTTL)
	require.Equal(t, "0 21 * * *", cfg.CronSchedule) // default
	require.Equal(t, "Asia/Ho_Chi_Minh", cfg.CronTimezone) // default
}

func TestLoad_MissingRequired(t *testing.T) {
	t.Setenv("PANCAKE_ACCESS_TOKEN", "")
	t.Setenv("DATABASE_URL", "")
	_, err := Load()
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestLoad -v`
Expected: FAIL (package/function not defined / build error).

- [ ] **Step 3: Write minimal implementation**

`internal/config/config.go`:
```go
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	PancakeAccessToken    string
	PancakeBaseURL        string
	DatabaseURL           string
	RedisURL              string
	CronSchedule          string
	CronTimezone          string
	HTTPAddr              string
	LogLevel              string
	WorkerPoolSize        int
	RateLimitPerPageRPS   float64
	RateLimitPerPageBurst int
	TokenTTL              time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		PancakeAccessToken:    os.Getenv("PANCAKE_ACCESS_TOKEN"),
		PancakeBaseURL:        getDefault("PANCAKE_BASE_URL", "https://pages.fm/api/public_api/v1"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		RedisURL:              os.Getenv("REDIS_URL"),
		CronSchedule:          getDefault("CRON_SCHEDULE", "0 21 * * *"),
		CronTimezone:          getDefault("CRON_TIMEZONE", "Asia/Ho_Chi_Minh"),
		HTTPAddr:              getDefault("HTTP_ADDR", ":8080"),
		LogLevel:              getDefault("LOG_LEVEL", "info"),
		WorkerPoolSize:        getInt("WORKER_POOL_SIZE", 8),
		RateLimitPerPageRPS:   getFloat("RATE_LIMIT_PER_PAGE_RPS", 5),
		RateLimitPerPageBurst: getInt("RATE_LIMIT_PER_PAGE_BURST", 5),
		TokenTTL:              getDuration("TOKEN_TTL", time.Hour),
	}
	if cfg.PancakeAccessToken == "" {
		return Config{}, fmt.Errorf("PANCAKE_ACCESS_TOKEN is required")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return Config{}, fmt.Errorf("REDIS_URL is required")
	}
	return cfg, nil
}

func getDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

func getFloat(key string, def float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(key)); err == nil {
		return v
	}
	return def
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestLoad -v`
Expected: PASS (add `go get github.com/stretchr/testify` if needed).

- [ ] **Step 5: Commit**

```bash
git add internal/config go.mod go.sum
git commit -m "feat: config loader with validation"
```

### Task 0.3: Structured logger + Gin health server

**Files:**
- Create: `internal/observability/logger.go`, `internal/transport/http/server.go`, `internal/transport/http/server_test.go`

**Interfaces:**
- Produces:
  - `func NewLogger(level string) *slog.Logger`
  - `func NewServer(addr string, log *slog.Logger) *http.Server` — Gin engine with `GET /healthz` returning `200 {"status":"ok"}` and `GET /metrics` (added in Sprint 5; for now `/healthz` only).

- [ ] **Step 1: Write the failing test**

`internal/transport/http/server_test.go`:
```go
package http

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHealthz(t *testing.T) {
	srv := NewServer(":0", slog.Default())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	srv.Handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "ok")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/transport/http/ -v`
Expected: FAIL (NewServer undefined).

- [ ] **Step 3: Write minimal implementation**

`internal/observability/logger.go`:
```go
package observability

import (
	"log/slog"
	"os"
)

func NewLogger(level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
```

`internal/transport/http/server.go`:
```go
package http

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func NewServer(addr string, log *slog.Logger) *http.Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return &http.Server{Addr: addr, Handler: r}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/transport/http/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/observability internal/transport/http go.mod go.sum
git commit -m "feat: slog logger and gin health server"
```

### Task 0.4: main.go wiring + dev docker-compose

**Files:**
- Create: `cmd/crawler/main.go`, `deploy/docker-compose.yml`

**Interfaces:**
- Consumes: `config.Load`, `observability.NewLogger`, `http.NewServer`.

- [ ] **Step 1: Write `cmd/crawler/main.go`**

```go
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/desi/crawl-conversations/internal/config"
	"github.com/desi/crawl-conversations/internal/observability"
	httpx "github.com/desi/crawl-conversations/internal/transport/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := observability.NewLogger(cfg.LogLevel)
	srv := httpx.NewServer(cfg.HTTPAddr, log)

	go func() {
		log.Info("http server starting", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server failed", "err", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Info("shutdown complete")
}
```

- [ ] **Step 2: Create `deploy/docker-compose.yml`**

```yaml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: crawler
      POSTGRES_PASSWORD: crawler
      POSTGRES_DB: crawler
    ports: ["5432:5432"]
    volumes: ["pgdata:/var/lib/postgresql/data"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U crawler"]
      interval: 5s
      timeout: 3s
      retries: 10
  redis:
    image: redis:7
    ports: ["6379:6379"]
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 10
volumes:
  pgdata:
```

- [ ] **Step 3: Verify build and run**

Run:
```bash
go build ./...
docker compose -f deploy/docker-compose.yml up -d
PANCAKE_ACCESS_TOKEN=x DATABASE_URL=postgres://crawler:crawler@localhost:5432/crawler?sslmode=disable REDIS_URL=redis://localhost:6379/0 go run ./cmd/crawler &
sleep 2 && curl -s localhost:8080/healthz
```
Expected: `{"status":"ok"}`; postgres and redis healthy.

- [ ] **Step 4: Commit**

```bash
git add cmd/crawler deploy/docker-compose.yml
git commit -m "feat: main wiring and dev docker-compose"
```

### Task 0.5: Verify Pancake API contract (spike, documented)

**Files:**
- Create: `docs/pancake-api-verified.md`

**Interfaces:**
- Produces: a verified contract document that Sprint 2's client conforms to. No code; this gates Sprint 2.

- [ ] **Step 1: Probe endpoints with the real token**

Using `PANCAKE_ACCESS_TOKEN`, call and capture sample JSON (redact tokens). Record exact base URL, auth placement (query param vs header), and field names:
```bash
# list pages
curl -s "$PANCAKE_BASE_URL/pages?access_token=$PANCAKE_ACCESS_TOKEN" | jq '.' | head -60
```

- [ ] **Step 2: Document the verified contract**

In `docs/pancake-api-verified.md`, fill a table for each endpoint with: HTTP method, path, auth param name, request params (pagination: page_number vs cursor; time filter param names like `since`/`until`), and the **exact JSON field names** for: page id/name, conversation id/updated_at/customer/tags, message id/inserted_at/sender/type/content/attachments, tag id/name/color, customer id/name/phone/email. Record observed rate-limit headers / 429 behavior and the `generate_page_access_token` endpoint shape.

- [ ] **Step 3: Reconcile with spec §8 and the plan**

For every field whose real name differs from the assumption, note `assumed -> actual`. Sprint 2 DTO (`internal/dto/pancake.go`) and Sprint 4 mappers use these actual names. Interfaces (method names/signatures) stay unchanged.

- [ ] **Step 4: Commit**

```bash
git add docs/pancake-api-verified.md
git commit -m "docs: verified pancake api contract"
```

**Sprint 0 done note:** record under spec §12 Sprint 0 what was done / deferred.

---

## Sprint 1 — Schema, migrations, entities, DB construction

**Goal of sprint:** Migrations create all tables (incl. partitioned `messages` + DEFAULT partition), the partition helper works, and `repository.New` builds both a `*gorm.DB` and a `*pgxpool.Pool`.

### Task 1.1: Entity structs

**Files:**
- Create: `internal/entity/entity.go`, `internal/entity/entity_test.go`

**Interfaces:**
- Produces (GORM-tagged structs; `Raw` is `json.RawMessage`):
  - `Page{ ID int64; PancakePageID, Platform, Name, Status string; LastSyncedAt *time.Time; Raw json.RawMessage; CreatedAt, UpdatedAt time.Time }`
  - `Customer{ ID int64; PageID int64; PancakeCustomerID, Name, Phone, Email, FbID, Gender string; Raw json.RawMessage; CreatedAt, UpdatedAt time.Time }`
  - `Tag{ ID int64; PageID int64; PancakeTagID, Name, Color string; CreatedAt, UpdatedAt time.Time }`
  - `Conversation{ ID int64; PageID int64; PancakeConversationID string; CustomerID *int64; Type, Snippet string; MessageCount int; LastMessageAt, UpdatedAt time.Time; Raw json.RawMessage; CreatedAt time.Time }`
  - `Message{ ID int64; PageID int64; ConversationID int64; PancakeMessageID, SenderID, Direction, Type, Content string; Attachments json.RawMessage; InsertedAt time.Time; Raw json.RawMessage; CreatedAt time.Time }`
  - `CrawlRun{ ID int64; StartedAt time.Time; FinishedAt *time.Time; Trigger, Status string; Stats json.RawMessage; Error string }`
  - `CrawlCheckpoint{ PageID int64; JobType string; Cursor *time.Time; Status string; LastRunAt *time.Time }`
  - Each gets a `TableName() string` method.

- [ ] **Step 1: Write the failing test**

`internal/entity/entity_test.go`:
```go
package entity

import "testing"

func TestTableNames(t *testing.T) {
	cases := map[string]string{
		Page{}.TableName():            "pages",
		Message{}.TableName():         "messages",
		Conversation{}.TableName():    "conversations",
		Customer{}.TableName():        "customers",
		Tag{}.TableName():             "tags",
		CrawlRun{}.TableName():        "crawl_runs",
		CrawlCheckpoint{}.TableName(): "crawl_checkpoints",
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("table name mismatch: got %q want %q", got, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/entity/ -v`
Expected: FAIL (types undefined).

- [ ] **Step 3: Write minimal implementation**

`internal/entity/entity.go`:
```go
package entity

import (
	"encoding/json"
	"time"
)

type Page struct {
	ID             int64 `gorm:"primaryKey"`
	PancakePageID  string
	Platform       string
	Name           string
	Status         string
	LastSyncedAt   *time.Time
	Raw            json.RawMessage `gorm:"type:jsonb"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (Page) TableName() string { return "pages" }

type Customer struct {
	ID                int64 `gorm:"primaryKey"`
	PageID            int64
	PancakeCustomerID string
	Name              string
	Phone             string
	Email             string
	FbID              string
	Gender            string
	Raw               json.RawMessage `gorm:"type:jsonb"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (Customer) TableName() string { return "customers" }

type Tag struct {
	ID           int64 `gorm:"primaryKey"`
	PageID       int64
	PancakeTagID string
	Name         string
	Color        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (Tag) TableName() string { return "tags" }

type Conversation struct {
	ID                    int64 `gorm:"primaryKey"`
	PageID                int64
	PancakeConversationID string
	CustomerID            *int64
	Type                  string
	Snippet               string
	MessageCount          int
	LastMessageAt         time.Time
	UpdatedAt             time.Time
	Raw                   json.RawMessage `gorm:"type:jsonb"`
	CreatedAt             time.Time
}

func (Conversation) TableName() string { return "conversations" }

type Message struct {
	ID               int64 `gorm:"primaryKey"`
	PageID           int64
	ConversationID   int64
	PancakeMessageID string
	SenderID         string
	Direction        string
	Type             string
	Content          string
	Attachments      json.RawMessage `gorm:"type:jsonb"`
	InsertedAt       time.Time
	Raw              json.RawMessage `gorm:"type:jsonb"`
	CreatedAt        time.Time
}

func (Message) TableName() string { return "messages" }

type CrawlRun struct {
	ID         int64 `gorm:"primaryKey"`
	StartedAt  time.Time
	FinishedAt *time.Time
	Trigger    string
	Status     string
	Stats      json.RawMessage `gorm:"type:jsonb"`
	Error      string
}

func (CrawlRun) TableName() string { return "crawl_runs" }

type CrawlCheckpoint struct {
	PageID    int64 `gorm:"primaryKey"`
	JobType   string
	Cursor    *time.Time
	Status    string
	LastRunAt *time.Time
}

func (CrawlCheckpoint) TableName() string { return "crawl_checkpoints" }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/entity/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/entity
git commit -m "feat: domain entities"
```

### Task 1.2: Migration — base tables

**Files:**
- Create: `db/migrations/00001_base_tables.sql`

**Interfaces:**
- Produces: tables `pages`, `customers`, `tags`, `conversations`, `conversation_tags`, `crawl_runs`, `crawl_checkpoints` with the unique keys/indexes from spec §4.

- [ ] **Step 1: Write the migration**

`db/migrations/00001_base_tables.sql`:
```sql
-- +goose Up
CREATE TABLE pages (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    pancake_page_id TEXT NOT NULL UNIQUE,
    platform        TEXT NOT NULL DEFAULT '',
    name            TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'active',
    last_synced_at  TIMESTAMPTZ,
    raw             JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE customers (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    page_id             BIGINT NOT NULL REFERENCES pages(id),
    pancake_customer_id TEXT NOT NULL,
    name                TEXT NOT NULL DEFAULT '',
    phone               TEXT NOT NULL DEFAULT '',
    email               TEXT NOT NULL DEFAULT '',
    fb_id               TEXT NOT NULL DEFAULT '',
    gender              TEXT NOT NULL DEFAULT '',
    raw                 JSONB,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (page_id, pancake_customer_id)
);

CREATE TABLE tags (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    page_id        BIGINT NOT NULL REFERENCES pages(id),
    pancake_tag_id TEXT NOT NULL,
    name           TEXT NOT NULL DEFAULT '',
    color          TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (page_id, pancake_tag_id)
);

CREATE TABLE conversations (
    id                      BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    page_id                 BIGINT NOT NULL REFERENCES pages(id),
    pancake_conversation_id TEXT NOT NULL,
    customer_id             BIGINT REFERENCES customers(id),
    type                    TEXT NOT NULL DEFAULT '',
    snippet                 TEXT NOT NULL DEFAULT '',
    message_count           INT NOT NULL DEFAULT 0,
    last_message_at         TIMESTAMPTZ NOT NULL DEFAULT 'epoch',
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    raw                     JSONB,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (page_id, pancake_conversation_id)
);
CREATE INDEX idx_conversations_page_updated ON conversations (page_id, updated_at DESC);

CREATE TABLE conversation_tags (
    conversation_id BIGINT NOT NULL REFERENCES conversations(id),
    tag_id          BIGINT NOT NULL REFERENCES tags(id),
    PRIMARY KEY (conversation_id, tag_id)
);

CREATE TABLE crawl_runs (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    trigger     TEXT NOT NULL DEFAULT 'cron',
    status      TEXT NOT NULL DEFAULT 'running',
    stats       JSONB,
    error       TEXT NOT NULL DEFAULT ''
);

CREATE TABLE crawl_checkpoints (
    page_id     BIGINT PRIMARY KEY REFERENCES pages(id),
    job_type    TEXT NOT NULL DEFAULT 'conversations',
    cursor      TIMESTAMPTZ,
    status      TEXT NOT NULL DEFAULT '',
    last_run_at TIMESTAMPTZ
);

-- +goose Down
DROP TABLE crawl_checkpoints;
DROP TABLE crawl_runs;
DROP TABLE conversation_tags;
DROP TABLE conversations;
DROP TABLE tags;
DROP TABLE customers;
DROP TABLE pages;
```

- [ ] **Step 2: Apply and verify up/down**

Run:
```bash
export DATABASE_URL=postgres://crawler:crawler@localhost:5432/crawler?sslmode=disable
goose -dir db/migrations postgres "$DATABASE_URL" up
goose -dir db/migrations postgres "$DATABASE_URL" status
goose -dir db/migrations postgres "$DATABASE_URL" down
goose -dir db/migrations postgres "$DATABASE_URL" up
```
Expected: up applies cleanly; `status` shows 00001 applied; down/up round-trips with no error.

- [ ] **Step 3: Commit**

```bash
git add db/migrations/00001_base_tables.sql
git commit -m "feat: base table migrations"
```

### Task 1.3: Migration — partitioned messages + DEFAULT partition + ensure-partition function

**Files:**
- Create: `db/migrations/00002_messages_partitioned.sql`

**Interfaces:**
- Produces: partitioned `messages` table, `messages_default` partition, SQL function `ensure_messages_partition(month_start date)`.

- [ ] **Step 1: Write the migration**

`db/migrations/00002_messages_partitioned.sql`:
```sql
-- +goose Up
CREATE TABLE messages (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY,
    page_id            BIGINT NOT NULL,
    conversation_id    BIGINT NOT NULL,
    pancake_message_id TEXT NOT NULL,
    sender_id          TEXT NOT NULL DEFAULT '',
    direction          TEXT NOT NULL DEFAULT '',
    type               TEXT NOT NULL DEFAULT '',
    content            TEXT NOT NULL DEFAULT '',
    attachments        JSONB,
    inserted_at        TIMESTAMPTZ NOT NULL,
    raw                JSONB,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, inserted_at),
    UNIQUE (pancake_message_id, inserted_at)
) PARTITION BY RANGE (inserted_at);

CREATE INDEX idx_messages_conv_inserted ON messages (conversation_id, inserted_at DESC);

CREATE TABLE messages_default PARTITION OF messages DEFAULT;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION ensure_messages_partition(month_start date)
RETURNS void AS $$
DECLARE
    part_name text := 'messages_y' || to_char(month_start, 'YYYY') || 'm' || to_char(month_start, 'MM');
    month_end date := (month_start + INTERVAL '1 month')::date;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = part_name) THEN
        EXECUTE format(
            'CREATE TABLE %I PARTITION OF messages FOR VALUES FROM (%L) TO (%L)',
            part_name, month_start, month_end
        );
    END IF;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS ensure_messages_partition(date);
DROP TABLE messages;
```

- [ ] **Step 2: Apply and verify partition routing**

Run:
```bash
goose -dir db/migrations postgres "$DATABASE_URL" up
psql "$DATABASE_URL" -c "SELECT ensure_messages_partition('2026-06-01');"
psql "$DATABASE_URL" -c "\d+ messages" | grep -E "partition|messages_y2026m06|messages_default"
```
Expected: `messages_y2026m06` and `messages_default` listed as partitions.

- [ ] **Step 3: Verify dedup constraint behavior**

Run:
```bash
psql "$DATABASE_URL" -c "INSERT INTO messages (page_id,conversation_id,pancake_message_id,inserted_at) VALUES (1,1,'m1','2026-06-10T00:00:00Z');"
psql "$DATABASE_URL" -c "INSERT INTO messages (page_id,conversation_id,pancake_message_id,inserted_at) VALUES (1,1,'m1','2026-06-10T00:00:00Z') ON CONFLICT (pancake_message_id, inserted_at) DO NOTHING;"
psql "$DATABASE_URL" -c "SELECT count(*) FROM messages WHERE pancake_message_id='m1';"
```
Expected: count = 1 (second insert ignored). Then `goose ... down && goose ... up` to reset.

- [ ] **Step 4: Commit**

```bash
git add db/migrations/00002_messages_partitioned.sql
git commit -m "feat: partitioned messages table and ensure-partition function"
```

### Task 1.4: DB construction (gorm + pgxpool) and partition helper

**Files:**
- Create: `internal/repository/db.go`, `internal/repository/db_test.go`

**Interfaces:**
- Produces:
  - `type DB struct { Gorm *gorm.DB; Pool *pgxpool.Pool }`
  - `func New(ctx context.Context, dsn string) (*DB, error)` — opens both, pings.
  - `func (d *DB) Close()`
  - `func (d *DB) EnsurePartition(ctx context.Context, ts time.Time) error` — calls `ensure_messages_partition(date_trunc('month', ts))`, caches done months in a `sync.Map` to skip repeat DDL.

- [ ] **Step 1: Write the failing test** (integration; requires DB)

`internal/repository/db_test.go`:
```go
package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func dsn(t *testing.T) string {
	v := os.Getenv("TEST_DATABASE_URL")
	if v == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	return v
}

func TestEnsurePartition(t *testing.T) {
	ctx := context.Background()
	db, err := New(ctx, dsn(t))
	require.NoError(t, err)
	defer db.Close()

	ts := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.EnsurePartition(ctx, ts))
	// idempotent second call (served from cache, no error)
	require.NoError(t, db.EnsurePartition(ctx, ts))

	var n int
	row := db.Pool.QueryRow(ctx, "SELECT count(*) FROM pg_class WHERE relname='messages_y2026m07'")
	require.NoError(t, row.Scan(&n))
	require.Equal(t, 1, n)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `TEST_DATABASE_URL=$DATABASE_URL go test ./internal/repository/ -run TestEnsurePartition -v`
Expected: FAIL (New/EnsurePartition undefined).

- [ ] **Step 3: Write minimal implementation**

`internal/repository/db.go`:
```go
package repository

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	Gorm     *gorm.DB
	Pool     *pgxpool.Pool
	partDone sync.Map // month string -> struct{}
}

func New(ctx context.Context, dsn string) (*DB, error) {
	g, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return &DB{Gorm: g, Pool: pool}, nil
}

func (d *DB) Close() {
	d.Pool.Close()
	if sqlDB, err := d.Gorm.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

func (d *DB) EnsurePartition(ctx context.Context, ts time.Time) error {
	month := time.Date(ts.Year(), ts.Month(), 1, 0, 0, 0, 0, time.UTC)
	key := month.Format("2006-01")
	if _, ok := d.partDone.Load(key); ok {
		return nil
	}
	if _, err := d.Pool.Exec(ctx, "SELECT ensure_messages_partition($1)", month); err != nil {
		return err
	}
	d.partDone.Store(key, struct{}{})
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `TEST_DATABASE_URL=$DATABASE_URL go test ./internal/repository/ -run TestEnsurePartition -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/db.go internal/repository/db_test.go go.mod go.sum
git commit -m "feat: db construction and partition helper"
```

**Sprint 1 done note:** record under spec §12 Sprint 1 what was done / deferred.

---

## Sprint 2 — Token manager and Pancake client

**Goal of sprint:** Real API calls succeed: list pages, generate per-page tokens (cached in Redis), list conversations/messages/tags with pagination and per-page rate limiting, retry on 429/5xx.

> **API-dependent:** field names in `internal/dto/pancake.go` and query params in `client.go` must match `docs/pancake-api-verified.md` (Task 0.5). Method names/signatures below stay fixed.

### Task 2.1: DTO structs and pagination params

**Files:**
- Create: `internal/dto/pancake.go`

**Interfaces:**
- Produces:
  - `type ListParams struct { PageNumber int; Since, Until *time.Time }`
  - `type PageDTO struct { ID, Name, Platform string; Raw json.RawMessage }`
  - `type CustomerDTO struct { ID, Name, Phone, Email, FbID, Gender string; Raw json.RawMessage }`
  - `type TagDTO struct { ID, Name, Color string }`
  - `type ConversationDTO struct { ID, Type, Snippet string; MessageCount int; UpdatedAt, LastMessageAt time.Time; Customer CustomerDTO; TagIDs []string; Raw json.RawMessage }`
  - `type MessageDTO struct { ID, SenderID, Direction, Type, Content string; Attachments json.RawMessage; InsertedAt time.Time; Raw json.RawMessage }`
  - `type ConversationsResponse struct { Conversations []ConversationDTO; HasNext bool }`
  - `type MessagesResponse struct { Messages []MessageDTO; HasNext bool }`

- [ ] **Step 1: Write the struct file**

`internal/dto/pancake.go`:
```go
package dto

import (
	"encoding/json"
	"time"
)

type ListParams struct {
	PageNumber int
	Since      *time.Time
	Until      *time.Time
}

type PageDTO struct {
	ID       string
	Name     string
	Platform string
	Raw      json.RawMessage
}

type CustomerDTO struct {
	ID     string
	Name   string
	Phone  string
	Email  string
	FbID   string
	Gender string
	Raw    json.RawMessage
}

type TagDTO struct {
	ID    string
	Name  string
	Color string
}

type ConversationDTO struct {
	ID            string
	Type          string
	Snippet       string
	MessageCount  int
	UpdatedAt     time.Time
	LastMessageAt time.Time
	Customer      CustomerDTO
	TagIDs        []string
	Raw           json.RawMessage
}

type MessageDTO struct {
	ID          string
	SenderID    string
	Direction   string
	Type        string
	Content     string
	Attachments json.RawMessage
	InsertedAt  time.Time
	Raw         json.RawMessage
}

type ConversationsResponse struct {
	Conversations []ConversationDTO
	HasNext       bool
}

type MessagesResponse struct {
	Messages []MessageDTO
	HasNext  bool
}
```

- [ ] **Step 2: Build**

Run: `go build ./internal/dto/`
Expected: builds.

- [ ] **Step 3: Commit**

```bash
git add internal/dto/pancake.go
git commit -m "feat: pancake dto structs"
```

### Task 2.2: Per-page rate limiter

**Files:**
- Create: `internal/pancake/ratelimit.go`, `internal/pancake/ratelimit_test.go`

**Interfaces:**
- Produces:
  - `type RateLimiter struct { ... }`
  - `func NewRateLimiter(rps float64, burst int) *RateLimiter`
  - `func (r *RateLimiter) Wait(ctx context.Context, pageID string) error` — one token-bucket per pageID (`sync.Map` of `*rate.Limiter`).

- [ ] **Step 1: Write the failing test**

`internal/pancake/ratelimit_test.go`:
```go
package pancake

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRateLimiter_PerPageThrottles(t *testing.T) {
	rl := NewRateLimiter(10, 1) // 10 rps, burst 1
	ctx := context.Background()
	start := time.Now()
	require.NoError(t, rl.Wait(ctx, "p1")) // immediate (burst)
	require.NoError(t, rl.Wait(ctx, "p1")) // must wait ~100ms
	require.GreaterOrEqual(t, time.Since(start), 80*time.Millisecond)
}

func TestRateLimiter_IndependentPerPage(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	ctx := context.Background()
	start := time.Now()
	require.NoError(t, rl.Wait(ctx, "p1"))
	require.NoError(t, rl.Wait(ctx, "p2")) // different page, own bucket, immediate
	require.Less(t, time.Since(start), 200*time.Millisecond)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/pancake/ -run TestRateLimiter -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Write minimal implementation**

`internal/pancake/ratelimit.go`:
```go
package pancake

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	rps      rate.Limit
	burst    int
	limiters sync.Map // pageID -> *rate.Limiter
}

func NewRateLimiter(rps float64, burst int) *RateLimiter {
	if burst < 1 {
		burst = 1
	}
	return &RateLimiter{rps: rate.Limit(rps), burst: burst}
}

func (r *RateLimiter) Wait(ctx context.Context, pageID string) error {
	l, _ := r.limiters.LoadOrStore(pageID, rate.NewLimiter(r.rps, r.burst))
	return l.(*rate.Limiter).Wait(ctx)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/pancake/ -run TestRateLimiter -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/pancake/ratelimit.go internal/pancake/ratelimit_test.go go.mod go.sum
git commit -m "feat: per-page rate limiter"
```

### Task 2.3: Pancake HTTP client

**Files:**
- Create: `internal/pancake/client.go`, `internal/pancake/client_test.go`

**Interfaces:**
- Produces:
  - ```go
    type Client interface {
        ListPages(ctx context.Context) ([]dto.PageDTO, error)
        GeneratePageToken(ctx context.Context, pageID string) (string, error)
        ListConversations(ctx context.Context, pageID, pageToken string, p dto.ListParams) (dto.ConversationsResponse, error)
        ListMessages(ctx context.Context, pageID, conversationID, pageToken string, p dto.ListParams) (dto.MessagesResponse, error)
        ListTags(ctx context.Context, pageID, pageToken string) ([]dto.TagDTO, error)
    }
    ```
  - `func NewClient(baseURL, accessToken string, rl *RateLimiter, httpClient *http.Client) Client`
- Consumes: `dto`, `RateLimiter` (2.2). On 429/5xx retries with exponential backoff (max 4 attempts). Rate-limits per page before each page-scoped call.

- [ ] **Step 1: Write the failing test** (httptest server returning fixture JSON)

`internal/pancake/client_test.go`:
```go
package pancake

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/pages", r.URL.Path)
		require.Equal(t, "acc", r.URL.Query().Get("access_token"))
		_, _ = w.Write([]byte(`{"data":[{"id":"p1","name":"Shop","platform":"facebook"}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "acc", NewRateLimiter(100, 10), srv.Client())
	pages, err := c.ListPages(context.Background())
	require.NoError(t, err)
	require.Len(t, pages, 1)
	require.Equal(t, "p1", pages[0].ID)
	require.Equal(t, "Shop", pages[0].Name)
}

func TestListConversations_PaginationFlag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "ptok", r.URL.Query().Get("page_access_token"))
		_, _ = w.Write([]byte(`{"conversations":[{"id":"c1","updated_at":"2026-06-10T00:00:00Z"}],"has_next":true}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "acc", NewRateLimiter(100, 10), srv.Client())
	resp, err := c.ListConversations(context.Background(), "p1", "ptok", dto.ListParams{PageNumber: 1})
	require.NoError(t, err)
	require.Len(t, resp.Conversations, 1)
	require.True(t, resp.HasNext)
}

func TestRetryOn500(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "acc", NewRateLimiter(100, 10), srv.Client())
	_, err := c.ListPages(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, calls)
}
```

> Add `import "github.com/desi/crawl-conversations/internal/dto"` to the test file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/pancake/ -run 'TestListPages|TestListConversations|TestRetry' -v`
Expected: FAIL (NewClient undefined).

- [ ] **Step 3: Write minimal implementation**

`internal/pancake/client.go` (parse the **raw** API shape into DTOs; adjust JSON tags to match Task 0.5 verified names):
```go
package pancake

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/desi/crawl-conversations/internal/dto"
)

type Client interface {
	ListPages(ctx context.Context) ([]dto.PageDTO, error)
	GeneratePageToken(ctx context.Context, pageID string) (string, error)
	ListConversations(ctx context.Context, pageID, pageToken string, p dto.ListParams) (dto.ConversationsResponse, error)
	ListMessages(ctx context.Context, pageID, conversationID, pageToken string, p dto.ListParams) (dto.MessagesResponse, error)
	ListTags(ctx context.Context, pageID, pageToken string) ([]dto.TagDTO, error)
}

type client struct {
	baseURL     string
	accessToken string
	rl          *RateLimiter
	hc          *http.Client
}

func NewClient(baseURL, accessToken string, rl *RateLimiter, hc *http.Client) Client {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &client{baseURL: baseURL, accessToken: accessToken, rl: rl, hc: hc}
}

func (c *client) doJSON(ctx context.Context, pageID, fullURL string, out any) error {
	const maxAttempts = 4
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if pageID != "" {
			if err := c.rl.Wait(ctx, pageID); err != nil {
				return err
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
		if err != nil {
			return err
		}
		resp, err := c.hc.Do(req)
		if err != nil {
			lastErr = err
			backoff(ctx, attempt)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("pancake status %d: %s", resp.StatusCode, string(body))
			backoff(ctx, attempt)
			continue
		}
		if resp.StatusCode >= 400 {
			return fmt.Errorf("pancake status %d: %s", resp.StatusCode, string(body))
		}
		return json.Unmarshal(body, out)
	}
	return lastErr
}

func backoff(ctx context.Context, attempt int) {
	d := time.Duration(1<<attempt) * 200 * time.Millisecond
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

func (c *client) ListPages(ctx context.Context) ([]dto.PageDTO, error) {
	u := fmt.Sprintf("%s/pages?access_token=%s", c.baseURL, url.QueryEscape(c.accessToken))
	var raw struct {
		Data []struct {
			ID       string          `json:"id"`
			Name     string          `json:"name"`
			Platform string          `json:"platform"`
			Raw      json.RawMessage `json:"-"`
		} `json:"data"`
	}
	if err := c.doJSON(ctx, "", u, &raw); err != nil {
		return nil, err
	}
	out := make([]dto.PageDTO, 0, len(raw.Data))
	for _, p := range raw.Data {
		out = append(out, dto.PageDTO{ID: p.ID, Name: p.Name, Platform: p.Platform})
	}
	return out, nil
}

func (c *client) GeneratePageToken(ctx context.Context, pageID string) (string, error) {
	u := fmt.Sprintf("%s/pages/%s/generate_page_access_token?access_token=%s",
		c.baseURL, url.PathEscape(pageID), url.QueryEscape(c.accessToken))
	var raw struct {
		PageAccessToken string `json:"page_access_token"`
	}
	if err := c.doJSON(ctx, "", u, &raw); err != nil {
		return "", err
	}
	return raw.PageAccessToken, nil
}

func (c *client) ListConversations(ctx context.Context, pageID, pageToken string, p dto.ListParams) (dto.ConversationsResponse, error) {
	q := url.Values{}
	q.Set("page_access_token", pageToken)
	q.Set("page_number", strconv.Itoa(p.PageNumber))
	if p.Since != nil {
		q.Set("since", strconv.FormatInt(p.Since.Unix(), 10))
	}
	if p.Until != nil {
		q.Set("until", strconv.FormatInt(p.Until.Unix(), 10))
	}
	u := fmt.Sprintf("%s/pages/%s/conversations?%s", c.baseURL, url.PathEscape(pageID), q.Encode())
	var raw struct {
		Conversations []struct {
			ID           string    `json:"id"`
			Type         string    `json:"type"`
			Snippet      string    `json:"snippet"`
			MessageCount int       `json:"message_count"`
			UpdatedAt    time.Time `json:"updated_at"`
		} `json:"conversations"`
		HasNext bool `json:"has_next"`
	}
	if err := c.doJSON(ctx, pageID, u, &raw); err != nil {
		return dto.ConversationsResponse{}, err
	}
	out := dto.ConversationsResponse{HasNext: raw.HasNext}
	for _, c := range raw.Conversations {
		out.Conversations = append(out.Conversations, dto.ConversationDTO{
			ID: c.ID, Type: c.Type, Snippet: c.Snippet,
			MessageCount: c.MessageCount, UpdatedAt: c.UpdatedAt, LastMessageAt: c.UpdatedAt,
		})
	}
	return out, nil
}

func (c *client) ListMessages(ctx context.Context, pageID, conversationID, pageToken string, p dto.ListParams) (dto.MessagesResponse, error) {
	q := url.Values{}
	q.Set("page_access_token", pageToken)
	q.Set("page_number", strconv.Itoa(p.PageNumber))
	u := fmt.Sprintf("%s/pages/%s/conversations/%s/messages?%s",
		c.baseURL, url.PathEscape(pageID), url.PathEscape(conversationID), q.Encode())
	var raw struct {
		Messages []struct {
			ID         string          `json:"id"`
			SenderID   string          `json:"sender_id"`
			Type       string          `json:"type"`
			Content    string          `json:"content"`
			InsertedAt time.Time       `json:"inserted_at"`
			Attach     json.RawMessage `json:"attachments"`
		} `json:"messages"`
		HasNext bool `json:"has_next"`
	}
	if err := c.doJSON(ctx, pageID, u, &raw); err != nil {
		return dto.MessagesResponse{}, err
	}
	out := dto.MessagesResponse{HasNext: raw.HasNext}
	for _, m := range raw.Messages {
		out.Messages = append(out.Messages, dto.MessageDTO{
			ID: m.ID, SenderID: m.SenderID, Type: m.Type, Content: m.Content,
			InsertedAt: m.InsertedAt, Attachments: m.Attach,
		})
	}
	return out, nil
}

func (c *client) ListTags(ctx context.Context, pageID, pageToken string) ([]dto.TagDTO, error) {
	u := fmt.Sprintf("%s/pages/%s/tags?page_access_token=%s",
		c.baseURL, url.PathEscape(pageID), url.QueryEscape(pageToken))
	var raw struct {
		Tags []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Color string `json:"color"`
		} `json:"tags"`
	}
	if err := c.doJSON(ctx, pageID, u, &raw); err != nil {
		return nil, err
	}
	out := make([]dto.TagDTO, 0, len(raw.Tags))
	for _, t := range raw.Tags {
		out = append(out, dto.TagDTO{ID: t.ID, Name: t.Name, Color: t.Color})
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/pancake/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/pancake/client.go internal/pancake/client_test.go go.mod go.sum
git commit -m "feat: pancake http client with retry"
```

### Task 2.4: Redis-backed PageTokenManager

**Files:**
- Create: `internal/token/manager.go`, `internal/token/manager_test.go`

**Interfaces:**
- Produces:
  - ```go
    type Manager interface {
        Get(ctx context.Context, pageID string) (string, error)
        Invalidate(ctx context.Context, pageID string) error
    }
    ```
  - `func NewManager(rdb *redis.Client, gen TokenGenerator, ttl time.Duration) Manager`
  - `type TokenGenerator interface { GeneratePageToken(ctx context.Context, pageID string) (string, error) }` — satisfied by `pancake.Client`.
- Behavior: `Get` returns cached token from Redis key `pancake:ptoken:<pageID>`; on miss, calls `gen.GeneratePageToken`, stores with `ttl`. Per-page `sync.Map` mutex avoids stampede. `Invalidate` deletes the key (called on 401).

- [ ] **Step 1: Write the failing test** (miniredis + fake generator)

`internal/token/manager_test.go`:
```go
package token

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type fakeGen struct{ calls int }

func (f *fakeGen) GeneratePageToken(ctx context.Context, pageID string) (string, error) {
	f.calls++
	return "tok-" + pageID, nil
}

func TestGet_CachesAfterFirstCall(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	gen := &fakeGen{}
	m := NewManager(rdb, gen, time.Hour)

	ctx := context.Background()
	tok1, err := m.Get(ctx, "p1")
	require.NoError(t, err)
	require.Equal(t, "tok-p1", tok1)
	tok2, err := m.Get(ctx, "p1")
	require.NoError(t, err)
	require.Equal(t, "tok-p1", tok2)
	require.Equal(t, 1, gen.calls) // second served from cache
}

func TestInvalidate(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	gen := &fakeGen{}
	m := NewManager(rdb, gen, time.Hour)
	ctx := context.Background()
	_, _ = m.Get(ctx, "p1")
	require.NoError(t, m.Invalidate(ctx, "p1"))
	_, _ = m.Get(ctx, "p1")
	require.Equal(t, 2, gen.calls) // regenerated after invalidate
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/token/ -v`
Expected: FAIL (NewManager undefined).

- [ ] **Step 3: Write minimal implementation**

`internal/token/manager.go`:
```go
package token

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type TokenGenerator interface {
	GeneratePageToken(ctx context.Context, pageID string) (string, error)
}

type Manager interface {
	Get(ctx context.Context, pageID string) (string, error)
	Invalidate(ctx context.Context, pageID string) error
}

type manager struct {
	rdb   *redis.Client
	gen   TokenGenerator
	ttl   time.Duration
	locks sync.Map // pageID -> *sync.Mutex
}

func NewManager(rdb *redis.Client, gen TokenGenerator, ttl time.Duration) Manager {
	return &manager{rdb: rdb, gen: gen, ttl: ttl}
}

func key(pageID string) string { return "pancake:ptoken:" + pageID }

func (m *manager) lockFor(pageID string) *sync.Mutex {
	l, _ := m.locks.LoadOrStore(pageID, &sync.Mutex{})
	return l.(*sync.Mutex)
}

func (m *manager) Get(ctx context.Context, pageID string) (string, error) {
	if v, err := m.rdb.Get(ctx, key(pageID)).Result(); err == nil {
		return v, nil
	} else if !errors.Is(err, redis.Nil) {
		return "", err
	}
	mu := m.lockFor(pageID)
	mu.Lock()
	defer mu.Unlock()
	// double-check after acquiring lock
	if v, err := m.rdb.Get(ctx, key(pageID)).Result(); err == nil {
		return v, nil
	}
	tok, err := m.gen.GeneratePageToken(ctx, pageID)
	if err != nil {
		return "", err
	}
	if err := m.rdb.Set(ctx, key(pageID), tok, m.ttl).Err(); err != nil {
		return "", err
	}
	return tok, nil
}

func (m *manager) Invalidate(ctx context.Context, pageID string) error {
	return m.rdb.Del(ctx, key(pageID)).Err()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/token/ -v`
Expected: PASS (`go get github.com/alicebob/miniredis/v2 github.com/redis/go-redis/v9`).

- [ ] **Step 5: Commit**

```bash
git add internal/token go.mod go.sum
git commit -m "feat: redis-backed page token manager"
```

**Sprint 2 done note:** record under spec §12 Sprint 2 what was done / deferred.

---

## Sprint 3 — Repository layer and idempotent writes

**Goal of sprint:** Idempotent upserts for page/customer/tag/conversation; message writes via both `UpsertBatch` (GORM) and `BulkLoad` (pgx COPY); the de-dup invariant proven by test.

> All repository tests are integration tests gated on `TEST_DATABASE_URL` (skipped otherwise). Run migrations against that DB first.

### Task 3.1: Aggregate repositories (page, customer, tag, conversation)

**Files:**
- Create: `internal/repository/aggregate_repo.go`, `internal/repository/aggregate_repo_test.go`

**Interfaces:**
- Produces:
  - ```go
    type PageRepository interface { Upsert(ctx context.Context, p entity.Page) (int64, error) }
    type CustomerRepository interface { Upsert(ctx context.Context, c entity.Customer) (int64, error) }
    type TagRepository interface { Upsert(ctx context.Context, t entity.Tag) (int64, error) }
    type ConversationRepository interface {
        Upsert(ctx context.Context, c entity.Conversation) (int64, error)
        SetTags(ctx context.Context, conversationID int64, tagIDs []int64) error
    }
    ```
  - Constructors: `NewPageRepository(db *DB) PageRepository`, etc.
- Behavior: upsert on the natural unique key, `DoUpdates` mutable columns, `Returning` `id`.

- [ ] **Step 1: Write the failing test**

`internal/repository/aggregate_repo_test.go`:
```go
package repository

import (
	"context"
	"testing"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/stretchr/testify/require"
)

func TestPageUpsert_Idempotent(t *testing.T) {
	ctx := context.Background()
	db, err := New(ctx, dsn(t))
	require.NoError(t, err)
	defer db.Close()
	repo := NewPageRepository(db)

	id1, err := repo.Upsert(ctx, entity.Page{PancakePageID: "pg-1", Name: "A"})
	require.NoError(t, err)
	id2, err := repo.Upsert(ctx, entity.Page{PancakePageID: "pg-1", Name: "B"})
	require.NoError(t, err)
	require.Equal(t, id1, id2) // same row reused

	var name string
	require.NoError(t, db.Pool.QueryRow(ctx, "SELECT name FROM pages WHERE id=$1", id1).Scan(&name))
	require.Equal(t, "B", name) // updated
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `TEST_DATABASE_URL=$DATABASE_URL go test ./internal/repository/ -run TestPageUpsert -v`
Expected: FAIL (NewPageRepository undefined).

- [ ] **Step 3: Write minimal implementation**

`internal/repository/aggregate_repo.go`:
```go
package repository

import (
	"context"

	"github.com/desi/crawl-conversations/internal/entity"
	"gorm.io/gorm/clause"
)

type PageRepository interface {
	Upsert(ctx context.Context, p entity.Page) (int64, error)
}
type CustomerRepository interface {
	Upsert(ctx context.Context, c entity.Customer) (int64, error)
}
type TagRepository interface {
	Upsert(ctx context.Context, t entity.Tag) (int64, error)
}
type ConversationRepository interface {
	Upsert(ctx context.Context, c entity.Conversation) (int64, error)
	SetTags(ctx context.Context, conversationID int64, tagIDs []int64) error
}

type pageRepo struct{ db *DB }
type customerRepo struct{ db *DB }
type tagRepo struct{ db *DB }
type conversationRepo struct{ db *DB }

func NewPageRepository(db *DB) PageRepository                 { return &pageRepo{db} }
func NewCustomerRepository(db *DB) CustomerRepository         { return &customerRepo{db} }
func NewTagRepository(db *DB) TagRepository                   { return &tagRepo{db} }
func NewConversationRepository(db *DB) ConversationRepository { return &conversationRepo{db} }

func (r *pageRepo) Upsert(ctx context.Context, p entity.Page) (int64, error) {
	err := r.db.Gorm.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "pancake_page_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "platform", "status", "raw", "updated_at"}),
	}).Create(&p).Error
	return p.ID, err
}

func (r *customerRepo) Upsert(ctx context.Context, c entity.Customer) (int64, error) {
	err := r.db.Gorm.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "page_id"}, {Name: "pancake_customer_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "phone", "email", "fb_id", "gender", "raw", "updated_at"}),
	}).Create(&c).Error
	return c.ID, err
}

func (r *tagRepo) Upsert(ctx context.Context, t entity.Tag) (int64, error) {
	err := r.db.Gorm.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "page_id"}, {Name: "pancake_tag_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "color", "updated_at"}),
	}).Create(&t).Error
	return t.ID, err
}

func (r *conversationRepo) Upsert(ctx context.Context, c entity.Conversation) (int64, error) {
	err := r.db.Gorm.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "page_id"}, {Name: "pancake_conversation_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"customer_id", "type", "snippet", "message_count", "last_message_at", "updated_at", "raw"}),
	}).Create(&c).Error
	return c.ID, err
}

func (r *conversationRepo) SetTags(ctx context.Context, conversationID int64, tagIDs []int64) error {
	return r.db.Gorm.WithContext(ctx).Transaction(func(tx *gormTx) error {
		if err := tx.Exec("DELETE FROM conversation_tags WHERE conversation_id = ?", conversationID).Error; err != nil {
			return err
		}
		for _, tid := range tagIDs {
			if err := tx.Exec("INSERT INTO conversation_tags (conversation_id, tag_id) VALUES (?, ?) ON CONFLICT DO NOTHING", conversationID, tid).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
```

> Add alias `import ( gorm "gorm.io/gorm" )` and define `type gormTx = gorm.DB` at top of the file (GORM's transaction callback receives `*gorm.DB`).

- [ ] **Step 4: Run test to verify it passes**

Run: `TEST_DATABASE_URL=$DATABASE_URL go test ./internal/repository/ -run TestPageUpsert -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/aggregate_repo.go internal/repository/aggregate_repo_test.go
git commit -m "feat: aggregate repositories with idempotent upsert"
```

### Task 3.2: MessageRepository — UpsertBatch (GORM)

**Files:**
- Create: `internal/repository/message_repo.go`, `internal/repository/message_repo_test.go`

**Interfaces:**
- Produces:
  - ```go
    type MessageRepository interface {
        UpsertBatch(ctx context.Context, msgs []entity.Message) error
        BulkLoad(ctx context.Context, msgs []entity.Message) (int64, error) // Task 3.3
    }
    func NewMessageRepository(db *DB) MessageRepository
    ```
- Behavior of `UpsertBatch`: ensure partition for each distinct month, then `CreateInBatches(500)` with `OnConflict{Columns: pancake_message_id+inserted_at, DoNothing: true}`.

- [ ] **Step 1: Write the failing test**

`internal/repository/message_repo_test.go`:
```go
package repository

import (
	"context"
	"testing"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/stretchr/testify/require"
)

func msg(id string, ts time.Time) entity.Message {
	return entity.Message{PageID: 1, ConversationID: 1, PancakeMessageID: id, InsertedAt: ts, Type: "text", Content: "hi"}
}

func TestUpsertBatch_DedupInvariant(t *testing.T) {
	ctx := context.Background()
	db, err := New(ctx, dsn(t))
	require.NoError(t, err)
	defer db.Close()
	repo := NewMessageRepository(db)

	ts := time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC)
	batch := []entity.Message{msg("dup-1", ts), msg("dup-2", ts)}

	require.NoError(t, repo.UpsertBatch(ctx, batch))
	require.NoError(t, repo.UpsertBatch(ctx, batch)) // second time: no new rows

	var n int
	require.NoError(t, db.Pool.QueryRow(ctx,
		"SELECT count(*) FROM messages WHERE pancake_message_id IN ('dup-1','dup-2')").Scan(&n))
	require.Equal(t, 2, n)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `TEST_DATABASE_URL=$DATABASE_URL go test ./internal/repository/ -run TestUpsertBatch -v`
Expected: FAIL (NewMessageRepository undefined).

- [ ] **Step 3: Write minimal implementation**

`internal/repository/message_repo.go`:
```go
package repository

import (
	"context"

	"github.com/desi/crawl-conversations/internal/entity"
	"gorm.io/gorm/clause"
)

type MessageRepository interface {
	UpsertBatch(ctx context.Context, msgs []entity.Message) error
	BulkLoad(ctx context.Context, msgs []entity.Message) (int64, error)
}

type messageRepo struct{ db *DB }

func NewMessageRepository(db *DB) MessageRepository { return &messageRepo{db} }

func (r *messageRepo) ensurePartitions(ctx context.Context, msgs []entity.Message) error {
	for _, m := range msgs {
		if err := r.db.EnsurePartition(ctx, m.InsertedAt); err != nil {
			return err
		}
	}
	return nil
}

func (r *messageRepo) UpsertBatch(ctx context.Context, msgs []entity.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	if err := r.ensurePartitions(ctx, msgs); err != nil {
		return err
	}
	return r.db.Gorm.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "pancake_message_id"}, {Name: "inserted_at"}},
		DoNothing: true,
	}).CreateInBatches(msgs, 500).Error
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `TEST_DATABASE_URL=$DATABASE_URL go test ./internal/repository/ -run TestUpsertBatch -v`
Expected: PASS (count stays 2 after the second upsert).

- [ ] **Step 5: Commit**

```bash
git add internal/repository/message_repo.go internal/repository/message_repo_test.go
git commit -m "feat: message upsert batch with dedup"
```

### Task 3.3: MessageRepository — BulkLoad (pgx COPY + staging)

**Files:**
- Modify: `internal/repository/message_repo.go`
- Modify: `internal/repository/message_repo_test.go`

**Interfaces:**
- Implements `BulkLoad` declared in Task 3.2. Returns rows actually inserted.
- Behavior: ensure partitions; in one tx on a single pooled conn: create `_stg_messages` temp table `ON COMMIT DROP`, `CopyFrom` rows, then `INSERT ... SELECT DISTINCT ON (pancake_message_id, inserted_at) ... ON CONFLICT DO NOTHING`.

- [ ] **Step 1: Write the failing test** (append)

Append to `internal/repository/message_repo_test.go`:
```go
func TestBulkLoad_DedupAcrossBatchAndExisting(t *testing.T) {
	ctx := context.Background()
	db, err := New(ctx, dsn(t))
	require.NoError(t, err)
	defer db.Close()
	repo := NewMessageRepository(db)

	ts := time.Date(2026, 6, 11, 8, 0, 0, 0, time.UTC)
	// intra-batch duplicate ("bl-1" twice) + a unique one
	batch := []entity.Message{msg("bl-1", ts), msg("bl-1", ts), msg("bl-2", ts)}

	n1, err := repo.BulkLoad(ctx, batch)
	require.NoError(t, err)
	require.Equal(t, int64(2), n1) // intra-batch dup collapsed

	n2, err := repo.BulkLoad(ctx, batch)
	require.NoError(t, err)
	require.Equal(t, int64(0), n2) // all already exist

	var n int
	require.NoError(t, db.Pool.QueryRow(ctx,
		"SELECT count(*) FROM messages WHERE pancake_message_id IN ('bl-1','bl-2')").Scan(&n))
	require.Equal(t, 2, n)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `TEST_DATABASE_URL=$DATABASE_URL go test ./internal/repository/ -run TestBulkLoad -v`
Expected: FAIL (BulkLoad not implemented / returns 0,nil panic).

- [ ] **Step 3: Write minimal implementation** (append to `message_repo.go`)

```go
import (
	"github.com/jackc/pgx/v5"
)

var stgColumns = []string{
	"page_id", "conversation_id", "pancake_message_id", "sender_id",
	"direction", "type", "content", "attachments", "inserted_at", "raw",
}

const createStg = `CREATE TEMP TABLE _stg_messages (
	page_id bigint, conversation_id bigint, pancake_message_id text, sender_id text,
	direction text, type text, content text, attachments jsonb, inserted_at timestamptz, raw jsonb
) ON COMMIT DROP`

const insertFromStg = `INSERT INTO messages
	(page_id, conversation_id, pancake_message_id, sender_id, direction, type, content, attachments, inserted_at, raw)
SELECT DISTINCT ON (pancake_message_id, inserted_at)
	page_id, conversation_id, pancake_message_id, sender_id, direction, type, content, attachments, inserted_at, raw
FROM _stg_messages
ORDER BY pancake_message_id, inserted_at
ON CONFLICT (pancake_message_id, inserted_at) DO NOTHING`

func (r *messageRepo) BulkLoad(ctx context.Context, msgs []entity.Message) (int64, error) {
	if len(msgs) == 0 {
		return 0, nil
	}
	if err := r.ensurePartitions(ctx, msgs); err != nil {
		return 0, err
	}
	conn, err := r.db.Pool.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, createStg); err != nil {
		return 0, err
	}
	rows := make([][]any, 0, len(msgs))
	for _, m := range msgs {
		rows = append(rows, []any{
			m.PageID, m.ConversationID, m.PancakeMessageID, m.SenderID,
			m.Direction, m.Type, m.Content, jsonOrNil(m.Attachments), m.InsertedAt, jsonOrNil(m.Raw),
		})
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"_stg_messages"}, stgColumns, pgx.CopyFromRows(rows)); err != nil {
		return 0, err
	}
	tag, err := tx.Exec(ctx, insertFromStg)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func jsonOrNil(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `TEST_DATABASE_URL=$DATABASE_URL go test ./internal/repository/ -run TestBulkLoad -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/message_repo.go internal/repository/message_repo_test.go go.mod go.sum
git commit -m "feat: message bulk load via pgx copy with dedup"
```

### Task 3.4: Checkpoint and CrawlRun repositories

**Files:**
- Create: `internal/repository/checkpoint_repo.go`, `internal/repository/checkpoint_repo_test.go`

**Interfaces:**
- Produces:
  - ```go
    type CheckpointRepository interface {
        Get(ctx context.Context, pageID int64) (entity.CrawlCheckpoint, bool, error)
        Save(ctx context.Context, cp entity.CrawlCheckpoint) error
    }
    type CrawlRunRepository interface {
        Start(ctx context.Context, trigger string) (int64, error)
        Finish(ctx context.Context, id int64, status, errMsg string, stats []byte) error
    }
    func NewCheckpointRepository(db *DB) CheckpointRepository
    func NewCrawlRunRepository(db *DB) CrawlRunRepository
    ```

- [ ] **Step 1: Write the failing test**

`internal/repository/checkpoint_repo_test.go`:
```go
package repository

import (
	"context"
	"testing"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/stretchr/testify/require"
)

func TestCheckpoint_SaveGet(t *testing.T) {
	ctx := context.Background()
	db, err := New(ctx, dsn(t))
	require.NoError(t, err)
	defer db.Close()

	pr := NewPageRepository(db)
	pid, err := pr.Upsert(ctx, entity.Page{PancakePageID: "cp-page"})
	require.NoError(t, err)

	repo := NewCheckpointRepository(db)
	_, found, err := repo.Get(ctx, pid)
	require.NoError(t, err)
	require.False(t, found)

	cur := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.Save(ctx, entity.CrawlCheckpoint{PageID: pid, JobType: "conversations", Cursor: &cur, Status: "ok"}))
	got, found, err := repo.Get(ctx, pid)
	require.NoError(t, err)
	require.True(t, found)
	require.WithinDuration(t, cur, *got.Cursor, time.Second)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `TEST_DATABASE_URL=$DATABASE_URL go test ./internal/repository/ -run TestCheckpoint -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Write minimal implementation**

`internal/repository/checkpoint_repo.go`:
```go
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CheckpointRepository interface {
	Get(ctx context.Context, pageID int64) (entity.CrawlCheckpoint, bool, error)
	Save(ctx context.Context, cp entity.CrawlCheckpoint) error
}

type CrawlRunRepository interface {
	Start(ctx context.Context, trigger string) (int64, error)
	Finish(ctx context.Context, id int64, status, errMsg string, stats []byte) error
}

type checkpointRepo struct{ db *DB }
type crawlRunRepo struct{ db *DB }

func NewCheckpointRepository(db *DB) CheckpointRepository { return &checkpointRepo{db} }
func NewCrawlRunRepository(db *DB) CrawlRunRepository     { return &crawlRunRepo{db} }

func (r *checkpointRepo) Get(ctx context.Context, pageID int64) (entity.CrawlCheckpoint, bool, error) {
	var cp entity.CrawlCheckpoint
	err := r.db.Gorm.WithContext(ctx).First(&cp, "page_id = ?", pageID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.CrawlCheckpoint{}, false, nil
	}
	if err != nil {
		return entity.CrawlCheckpoint{}, false, err
	}
	return cp, true, nil
}

func (r *checkpointRepo) Save(ctx context.Context, cp entity.CrawlCheckpoint) error {
	now := time.Now()
	cp.LastRunAt = &now
	return r.db.Gorm.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "page_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"job_type", "cursor", "status", "last_run_at"}),
	}).Create(&cp).Error
}

func (r *crawlRunRepo) Start(ctx context.Context, trigger string) (int64, error) {
	run := entity.CrawlRun{StartedAt: time.Now(), Trigger: trigger, Status: "running"}
	err := r.db.Gorm.WithContext(ctx).Create(&run).Error
	return run.ID, err
}

func (r *crawlRunRepo) Finish(ctx context.Context, id int64, status, errMsg string, stats []byte) error {
	now := time.Now()
	return r.db.Gorm.WithContext(ctx).Model(&entity.CrawlRun{}).Where("id = ?", id).
		Updates(map[string]any{"status": status, "error": errMsg, "stats": stats, "finished_at": &now}).Error
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `TEST_DATABASE_URL=$DATABASE_URL go test ./internal/repository/ -run TestCheckpoint -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/checkpoint_repo.go internal/repository/checkpoint_repo_test.go
git commit -m "feat: checkpoint and crawl run repositories"
```

**Sprint 3 done note:** record under spec §12 Sprint 3 what was done / deferred.

---

## Sprint 4 — Crawl orchestration (backfill + incremental + worker pool)

**Goal of sprint:** Process a page end-to-end (tags → conversations → customers → messages), choosing backfill (`BulkLoad`) on first run and incremental (`UpsertBatch` + checkpoint + overlap) afterward; run pages concurrently via a worker pool; one `CrawlService.Run` wires it all with a `crawl_run` record.

### Task 4.1: DTO → entity mappers

**Files:**
- Create: `internal/dto/mapper.go`, `internal/dto/mapper_test.go`

**Interfaces:**
- Produces:
  - `func MapCustomer(pageID int64, c CustomerDTO) entity.Customer`
  - `func MapTag(pageID int64, t TagDTO) entity.Tag`
  - `func MapConversation(pageID int64, customerID *int64, c ConversationDTO) entity.Conversation`
  - `func MapMessage(pageID, conversationID int64, m MessageDTO) entity.Message`

- [ ] **Step 1: Write the failing test**

`internal/dto/mapper_test.go`:
```go
package dto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMapMessage(t *testing.T) {
	ts := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	m := MapMessage(7, 9, MessageDTO{ID: "m1", SenderID: "s1", Type: "text", Content: "hi", InsertedAt: ts})
	require.Equal(t, int64(7), m.PageID)
	require.Equal(t, int64(9), m.ConversationID)
	require.Equal(t, "m1", m.PancakeMessageID)
	require.Equal(t, ts, m.InsertedAt)
}

func TestMapConversation_CustomerFK(t *testing.T) {
	cid := int64(3)
	c := MapConversation(7, &cid, ConversationDTO{ID: "c1", Type: "inbox", MessageCount: 5})
	require.Equal(t, int64(7), c.PageID)
	require.Equal(t, "c1", c.PancakeConversationID)
	require.Equal(t, &cid, c.CustomerID)
	require.Equal(t, 5, c.MessageCount)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/dto/ -run TestMap -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Write minimal implementation**

`internal/dto/mapper.go`:
```go
package dto

import "github.com/desi/crawl-conversations/internal/entity"

func MapCustomer(pageID int64, c CustomerDTO) entity.Customer {
	return entity.Customer{
		PageID: pageID, PancakeCustomerID: c.ID, Name: c.Name, Phone: c.Phone,
		Email: c.Email, FbID: c.FbID, Gender: c.Gender, Raw: c.Raw,
	}
}

func MapTag(pageID int64, t TagDTO) entity.Tag {
	return entity.Tag{PageID: pageID, PancakeTagID: t.ID, Name: t.Name, Color: t.Color}
}

func MapConversation(pageID int64, customerID *int64, c ConversationDTO) entity.Conversation {
	return entity.Conversation{
		PageID: pageID, PancakeConversationID: c.ID, CustomerID: customerID,
		Type: c.Type, Snippet: c.Snippet, MessageCount: c.MessageCount,
		LastMessageAt: c.LastMessageAt, UpdatedAt: c.UpdatedAt, Raw: c.Raw,
	}
}

func MapMessage(pageID, conversationID int64, m MessageDTO) entity.Message {
	return entity.Message{
		PageID: pageID, ConversationID: conversationID, PancakeMessageID: m.ID,
		SenderID: m.SenderID, Direction: m.Direction, Type: m.Type, Content: m.Content,
		Attachments: m.Attachments, InsertedAt: m.InsertedAt, Raw: m.Raw,
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/dto/ -run TestMap -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/dto/mapper.go internal/dto/mapper_test.go
git commit -m "feat: dto to entity mappers"
```

### Task 4.2: PageProcessor (backfill + incremental for one page)

**Files:**
- Create: `internal/business/ports.go`, `internal/business/page_processor.go`, `internal/business/page_processor_test.go`

**Interfaces:**
- `internal/business/ports.go` re-declares the **dependency interfaces** the business layer consumes (so business depends on local interfaces, satisfied by `pancake`/`token`/`repository`):
  ```go
  type PancakeClient interface {
      ListPages(ctx context.Context) ([]dto.PageDTO, error)
      ListConversations(ctx context.Context, pageID, pageToken string, p dto.ListParams) (dto.ConversationsResponse, error)
      ListMessages(ctx context.Context, pageID, conversationID, pageToken string, p dto.ListParams) (dto.MessagesResponse, error)
      ListTags(ctx context.Context, pageID, pageToken string) ([]dto.TagDTO, error)
  }
  type TokenManager interface { Get(ctx context.Context, pageID string) (string, error) }
  type Stats struct { Conversations, Messages, Customers int }
  ```
- Produces:
  - `type PageProcessor interface { Process(ctx context.Context, page entity.Page) (Stats, error) }`
  - `func NewPageProcessor(client PancakeClient, tokens TokenManager, repos Repos, overlap time.Duration) PageProcessor`
  - `type Repos struct { Page repository.PageRepository; Customer repository.CustomerRepository; Tag repository.TagRepository; Conversation repository.ConversationRepository; Message repository.MessageRepository; Checkpoint repository.CheckpointRepository }`
- Behavior:
  - Resolve page token; upsert page-level tags into a `map[pancakeTagID]int64`.
  - Read checkpoint: if absent → **backfill mode** (`BulkLoad`, `Since=nil`); else **incremental mode** (`UpsertBatch`, `Since = cursor - overlap`).
  - Paginate conversations (page_number from 1); for each: upsert customer (if present) → upsert conversation → `SetTags` → fetch all its messages (paginated) → write messages via the mode's method.
  - Track max `UpdatedAt` seen; on success `Checkpoint.Save(cursor=maxUpdated, status="ok")`.

- [ ] **Step 1: Write the failing test** (fakes count which write path is used)

`internal/business/page_processor_test.go`:
```go
package business

import (
	"context"
	"testing"
	"time"

	"github.com/desi/crawl-conversations/internal/dto"
	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/stretchr/testify/require"
)

// --- fakes ---
type fakeClient struct{}

func (fakeClient) ListPages(ctx context.Context) ([]dto.PageDTO, error) { return nil, nil }
func (fakeClient) ListTags(ctx context.Context, pageID, tok string) ([]dto.TagDTO, error) {
	return []dto.TagDTO{{ID: "t1", Name: "VIP"}}, nil
}
func (fakeClient) ListConversations(ctx context.Context, pageID, tok string, p dto.ListParams) (dto.ConversationsResponse, error) {
	if p.PageNumber > 1 {
		return dto.ConversationsResponse{}, nil
	}
	return dto.ConversationsResponse{Conversations: []dto.ConversationDTO{
		{ID: "c1", UpdatedAt: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), TagIDs: []string{"t1"},
			Customer: dto.CustomerDTO{ID: "cust1", Name: "Anh"}},
	}, HasNext: false}, nil
}
func (fakeClient) ListMessages(ctx context.Context, pageID, convID, tok string, p dto.ListParams) (dto.MessagesResponse, error) {
	if p.PageNumber > 1 {
		return dto.MessagesResponse{}, nil
	}
	return dto.MessagesResponse{Messages: []dto.MessageDTO{
		{ID: "m1", Type: "text", Content: "hi", InsertedAt: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)},
	}, HasNext: false}, nil
}

type fakeTokens struct{}

func (fakeTokens) Get(ctx context.Context, pageID string) (string, error) { return "ptok", nil }

// spy repos
type spyMsg struct{ bulk, upsert int }

func (s *spyMsg) UpsertBatch(ctx context.Context, m []entity.Message) error { s.upsert++; return nil }
func (s *spyMsg) BulkLoad(ctx context.Context, m []entity.Message) (int64, error) {
	s.bulk++
	return int64(len(m)), nil
}

// minimal stub repos returning incrementing ids
type stubPage struct{}
func (stubPage) Upsert(ctx context.Context, p entity.Page) (int64, error) { return 1, nil }
type stubCustomer struct{}
func (stubCustomer) Upsert(ctx context.Context, c entity.Customer) (int64, error) { return 2, nil }
type stubTag struct{}
func (stubTag) Upsert(ctx context.Context, t entity.Tag) (int64, error) { return 3, nil }
type stubConv struct{}
func (stubConv) Upsert(ctx context.Context, c entity.Conversation) (int64, error) { return 4, nil }
func (stubConv) SetTags(ctx context.Context, id int64, ids []int64) error          { return nil }

type stubCheckpoint struct {
	cp    entity.CrawlCheckpoint
	found bool
	saved *entity.CrawlCheckpoint
}

func (s *stubCheckpoint) Get(ctx context.Context, pageID int64) (entity.CrawlCheckpoint, bool, error) {
	return s.cp, s.found, nil
}
func (s *stubCheckpoint) Save(ctx context.Context, cp entity.CrawlCheckpoint) error {
	s.saved = &cp
	return nil
}

func newRepos(msg *spyMsg, cp *stubCheckpoint) Repos {
	return Repos{Page: stubPage{}, Customer: stubCustomer{}, Tag: stubTag{}, Conversation: stubConv{}, Message: msg, Checkpoint: cp}
}

func TestProcess_BackfillUsesBulkLoad(t *testing.T) {
	msg := &spyMsg{}
	cp := &stubCheckpoint{found: false}
	p := NewPageProcessor(fakeClient{}, fakeTokens{}, newRepos(msg, cp), 5*time.Second)
	st, err := p.Process(context.Background(), entity.Page{ID: 1, PancakePageID: "pg1"})
	require.NoError(t, err)
	require.Equal(t, 1, msg.bulk)
	require.Equal(t, 0, msg.upsert)
	require.Equal(t, 1, st.Messages)
	require.NotNil(t, cp.saved)
	require.WithinDuration(t, time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), *cp.saved.Cursor, time.Second)
}

func TestProcess_IncrementalUsesUpsert(t *testing.T) {
	msg := &spyMsg{}
	cur := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	cp := &stubCheckpoint{found: true, cp: entity.CrawlCheckpoint{PageID: 1, Cursor: &cur}}
	p := NewPageProcessor(fakeClient{}, fakeTokens{}, newRepos(msg, cp), 5*time.Second)
	_, err := p.Process(context.Background(), entity.Page{ID: 1, PancakePageID: "pg1"})
	require.NoError(t, err)
	require.Equal(t, 0, msg.bulk)
	require.Equal(t, 1, msg.upsert)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/business/ -run TestProcess -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Write minimal implementation**

`internal/business/ports.go`:
```go
package business

import (
	"context"

	"github.com/desi/crawl-conversations/internal/dto"
	"github.com/desi/crawl-conversations/internal/repository"
)

type PancakeClient interface {
	ListPages(ctx context.Context) ([]dto.PageDTO, error)
	ListConversations(ctx context.Context, pageID, pageToken string, p dto.ListParams) (dto.ConversationsResponse, error)
	ListMessages(ctx context.Context, pageID, conversationID, pageToken string, p dto.ListParams) (dto.MessagesResponse, error)
	ListTags(ctx context.Context, pageID, pageToken string) ([]dto.TagDTO, error)
}

type TokenManager interface {
	Get(ctx context.Context, pageID string) (string, error)
}

type Stats struct {
	Conversations int
	Messages      int
	Customers     int
}

type Repos struct {
	Page         repository.PageRepository
	Customer     repository.CustomerRepository
	Tag          repository.TagRepository
	Conversation repository.ConversationRepository
	Message      repository.MessageRepository
	Checkpoint   repository.CheckpointRepository
}
```

`internal/business/page_processor.go`:
```go
package business

import (
	"context"
	"time"

	"github.com/desi/crawl-conversations/internal/dto"
	"github.com/desi/crawl-conversations/internal/entity"
)

type PageProcessor interface {
	Process(ctx context.Context, page entity.Page) (Stats, error)
}

type pageProcessor struct {
	client  PancakeClient
	tokens  TokenManager
	repos   Repos
	overlap time.Duration
}

func NewPageProcessor(client PancakeClient, tokens TokenManager, repos Repos, overlap time.Duration) PageProcessor {
	return &pageProcessor{client: client, tokens: tokens, repos: repos, overlap: overlap}
}

func (p *pageProcessor) Process(ctx context.Context, page entity.Page) (Stats, error) {
	var st Stats
	tok, err := p.tokens.Get(ctx, page.PancakePageID)
	if err != nil {
		return st, err
	}

	// page-level tags
	tags, err := p.client.ListTags(ctx, page.PancakePageID, tok)
	if err != nil {
		return st, err
	}
	tagID := make(map[string]int64, len(tags))
	for _, t := range tags {
		id, err := p.repos.Tag.Upsert(ctx, dto.MapTag(page.ID, t))
		if err != nil {
			return st, err
		}
		tagID[t.ID] = id
	}

	// mode selection
	cp, found, err := p.repos.Checkpoint.Get(ctx, page.ID)
	if err != nil {
		return st, err
	}
	var since *time.Time
	if found && cp.Cursor != nil {
		s := cp.Cursor.Add(-p.overlap)
		since = &s
	}
	backfill := !found

	maxUpdated := time.Time{}
	pageNum := 1
	for {
		resp, err := p.client.ListConversations(ctx, page.PancakePageID, tok, dto.ListParams{PageNumber: pageNum, Since: since})
		if err != nil {
			return st, err
		}
		if len(resp.Conversations) == 0 {
			break
		}
		for _, c := range resp.Conversations {
			var custID *int64
			if c.Customer.ID != "" {
				id, err := p.repos.Customer.Upsert(ctx, dto.MapCustomer(page.ID, c.Customer))
				if err != nil {
					return st, err
				}
				custID = &id
				st.Customers++
			}
			convID, err := p.repos.Conversation.Upsert(ctx, dto.MapConversation(page.ID, custID, c))
			if err != nil {
				return st, err
			}
			st.Conversations++

			tagIDs := make([]int64, 0, len(c.TagIDs))
			for _, pt := range c.TagIDs {
				if id, ok := tagID[pt]; ok {
					tagIDs = append(tagIDs, id)
				}
			}
			if err := p.repos.Conversation.SetTags(ctx, convID, tagIDs); err != nil {
				return st, err
			}

			msgs, err := p.fetchMessages(ctx, page, convID, c.ID, tok)
			if err != nil {
				return st, err
			}
			if len(msgs) > 0 {
				if backfill {
					if _, err := p.repos.Message.BulkLoad(ctx, msgs); err != nil {
						return st, err
					}
				} else {
					if err := p.repos.Message.UpsertBatch(ctx, msgs); err != nil {
						return st, err
					}
				}
				st.Messages += len(msgs)
			}
			if c.UpdatedAt.After(maxUpdated) {
				maxUpdated = c.UpdatedAt
			}
		}
		if !resp.HasNext {
			break
		}
		pageNum++
	}

	if maxUpdated.IsZero() && found && cp.Cursor != nil {
		maxUpdated = *cp.Cursor // nothing new; keep prior cursor
	}
	if !maxUpdated.IsZero() {
		cur := maxUpdated
		if err := p.repos.Checkpoint.Save(ctx, entity.CrawlCheckpoint{
			PageID: page.ID, JobType: "conversations", Cursor: &cur, Status: "ok",
		}); err != nil {
			return st, err
		}
	}
	return st, nil
}

func (p *pageProcessor) fetchMessages(ctx context.Context, page entity.Page, convID int64, pancakeConvID, tok string) ([]entity.Message, error) {
	var out []entity.Message
	pageNum := 1
	for {
		resp, err := p.client.ListMessages(ctx, page.PancakePageID, pancakeConvID, tok, dto.ListParams{PageNumber: pageNum})
		if err != nil {
			return nil, err
		}
		if len(resp.Messages) == 0 {
			break
		}
		for _, m := range resp.Messages {
			out = append(out, dto.MapMessage(page.ID, convID, m))
		}
		if !resp.HasNext {
			break
		}
		pageNum++
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/business/ -run TestProcess -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/business/ports.go internal/business/page_processor.go internal/business/page_processor_test.go
git commit -m "feat: page processor with backfill and incremental modes"
```

### Task 4.3: Worker pool

**Files:**
- Create: `internal/transport/worker/pool.go`, `internal/transport/worker/pool_test.go`

**Interfaces:**
- Produces:
  - `type Job func(ctx context.Context) error`
  - `func Run(ctx context.Context, size int, jobs []Job) []error` — runs jobs over `size` goroutines, returns per-job errors (nil entries for success), order-independent. A job panic is recovered into an error.

- [ ] **Step 1: Write the failing test**

`internal/transport/worker/pool_test.go`:
```go
package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun_AllExecuteAndCollectErrors(t *testing.T) {
	var count int32
	jobs := []Job{
		func(ctx context.Context) error { atomic.AddInt32(&count, 1); return nil },
		func(ctx context.Context) error { atomic.AddInt32(&count, 1); return errors.New("boom") },
		func(ctx context.Context) error { atomic.AddInt32(&count, 1); return nil },
	}
	errs := Run(context.Background(), 2, jobs)
	require.Equal(t, int32(3), count)
	var failures int
	for _, e := range errs {
		if e != nil {
			failures++
		}
	}
	require.Equal(t, 1, failures)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/transport/worker/ -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Write minimal implementation**

`internal/transport/worker/pool.go`:
```go
package worker

import (
	"context"
	"fmt"
	"sync"
)

type Job func(ctx context.Context) error

func Run(ctx context.Context, size int, jobs []Job) []error {
	if size < 1 {
		size = 1
	}
	errs := make([]error, len(jobs))
	type item struct {
		idx int
		job Job
	}
	ch := make(chan item)
	var wg sync.WaitGroup
	for i := 0; i < size; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for it := range ch {
				errs[it.idx] = runOne(ctx, it.job)
			}
		}()
	}
	for i, j := range jobs {
		ch <- item{idx: i, job: j}
	}
	close(ch)
	wg.Wait()
	return errs
}

func runOne(ctx context.Context, job Job) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return job(ctx)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/transport/worker/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/transport/worker
git commit -m "feat: worker pool"
```

### Task 4.4: CrawlService.Run

**Files:**
- Create: `internal/business/crawl_service.go`, `internal/business/crawl_service_test.go`

**Interfaces:**
- Produces:
  - `type CrawlService interface { Run(ctx context.Context, trigger string) error }`
  - `func NewCrawlService(client PancakeClient, processor PageProcessor, repos Repos, runs repository.CrawlRunRepository, poolSize int, log *slog.Logger) CrawlService`
- Behavior: `runs.Start(trigger)` → `client.ListPages` → upsert each page (resolve entity ID) → build one `worker.Job` per page calling `processor.Process` → `worker.Run` → aggregate stats/errors → `runs.Finish(status, errMsg, statsJSON)`. Page-level errors do not abort other pages; overall status is `failed` if any page errored, else `success`.

- [ ] **Step 1: Write the failing test**

`internal/business/crawl_service_test.go`:
```go
package business

import (
	"context"
	"log/slog"
	"testing"

	"github.com/desi/crawl-conversations/internal/dto"
	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/stretchr/testify/require"
)

type listPagesClient struct{ fakeClient }

func (listPagesClient) ListPages(ctx context.Context) ([]dto.PageDTO, error) {
	return []dto.PageDTO{{ID: "pg1", Name: "Shop"}}, nil
}

type spyProcessor struct{ calls int }

func (s *spyProcessor) Process(ctx context.Context, page entity.Page) (Stats, error) {
	s.calls++
	return Stats{Conversations: 1, Messages: 2}, nil
}

type spyRuns struct {
	started  bool
	finished string
}

func (s *spyRuns) Start(ctx context.Context, trigger string) (int64, error) { s.started = true; return 1, nil }
func (s *spyRuns) Finish(ctx context.Context, id int64, status, errMsg string, stats []byte) error {
	s.finished = status
	return nil
}

func TestCrawlService_Run_Success(t *testing.T) {
	proc := &spyProcessor{}
	runs := &spyRuns{}
	repos := newRepos(&spyMsg{}, &stubCheckpoint{})
	svc := NewCrawlService(listPagesClient{}, proc, repos, runs, 4, slog.Default())
	require.NoError(t, svc.Run(context.Background(), "manual"))
	require.True(t, runs.started)
	require.Equal(t, "success", runs.finished)
	require.Equal(t, 1, proc.calls)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/business/ -run TestCrawlService -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Write minimal implementation**

`internal/business/crawl_service.go`:
```go
package business

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/desi/crawl-conversations/internal/repository"
	"github.com/desi/crawl-conversations/internal/transport/worker"
)

type CrawlService interface {
	Run(ctx context.Context, trigger string) error
}

type crawlService struct {
	client    PancakeClient
	processor PageProcessor
	repos     Repos
	runs      repository.CrawlRunRepository
	poolSize  int
	log       *slog.Logger
}

func NewCrawlService(client PancakeClient, processor PageProcessor, repos Repos, runs repository.CrawlRunRepository, poolSize int, log *slog.Logger) CrawlService {
	return &crawlService{client: client, processor: processor, repos: repos, runs: runs, poolSize: poolSize, log: log}
}

func (s *crawlService) Run(ctx context.Context, trigger string) error {
	runID, err := s.runs.Start(ctx, trigger)
	if err != nil {
		return err
	}

	pages, err := s.client.ListPages(ctx)
	if err != nil {
		_ = s.runs.Finish(ctx, runID, "failed", err.Error(), nil)
		return err
	}

	entities := make([]entity.Page, 0, len(pages))
	for _, p := range pages {
		raw, _ := json.Marshal(p.Raw)
		id, err := s.repos.Page.Upsert(ctx, entity.Page{PancakePageID: p.ID, Name: p.Name, Platform: p.Platform, Raw: raw, Status: "active"})
		if err != nil {
			_ = s.runs.Finish(ctx, runID, "failed", err.Error(), nil)
			return err
		}
		entities = append(entities, entity.Page{ID: id, PancakePageID: p.ID, Name: p.Name})
	}

	var total Stats
	jobs := make([]worker.Job, len(entities))
	stats := make([]Stats, len(entities))
	for i := range entities {
		i := i
		jobs[i] = func(ctx context.Context) error {
			st, err := s.processor.Process(ctx, entities[i])
			stats[i] = st
			if err != nil {
				s.log.Error("page process failed", "page", entities[i].PancakePageID, "err", err)
			}
			return err
		}
	}
	errs := worker.Run(ctx, s.poolSize, jobs)

	status := "success"
	errMsg := ""
	for i, e := range errs {
		total.Conversations += stats[i].Conversations
		total.Messages += stats[i].Messages
		total.Customers += stats[i].Customers
		if e != nil {
			status = "failed"
			errMsg = e.Error()
		}
	}
	statsJSON, _ := json.Marshal(total)
	return s.runs.Finish(ctx, runID, status, errMsg, statsJSON)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/business/ -run TestCrawlService -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/business/crawl_service.go internal/business/crawl_service_test.go
git commit -m "feat: crawl service orchestration"
```

**Sprint 4 done note:** record under spec §12 Sprint 4 what was done / deferred.

---

## Sprint 5 — Scheduler, observability, DI wiring, deploy

**Goal of sprint:** Daily 21:00 cron + manual trigger (non-overlapping), Prometheus metrics, full DI wiring in `main`, migrations on startup, multi-stage Docker image, complete docker-compose and README.

### Task 5.1: Scheduler (cron + manual trigger, non-overlapping)

**Files:**
- Create: `internal/transport/scheduler/scheduler.go`, `internal/transport/scheduler/scheduler_test.go`

**Interfaces:**
- Produces:
  - `type RunFunc func(ctx context.Context, trigger string) error`
  - `type Scheduler interface { Start() error; Stop(); Trigger() bool }`
  - `func NewScheduler(spec, tz string, run RunFunc, log *slog.Logger) (Scheduler, error)`
- Behavior: cron fires `run(ctx,"cron")` on `spec` in timezone `tz`. `Trigger()` runs `run(ctx,"manual")` in a goroutine and returns `false` if a run is already in flight (an `atomic.Bool` guards overlap; cron firing while busy is skipped too).

- [ ] **Step 1: Write the failing test**

`internal/transport/scheduler/scheduler_test.go`:
```go
package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTrigger_NonOverlapping(t *testing.T) {
	var running sync.WaitGroup
	running.Add(1)
	release := make(chan struct{})
	var calls int32
	run := func(ctx context.Context, trigger string) error {
		atomic.AddInt32(&calls, 1)
		running.Done()
		<-release // hold the run open
		return nil
	}
	s, err := NewScheduler("0 21 * * *", "Asia/Ho_Chi_Minh", run, slog.Default())
	require.NoError(t, err)

	require.True(t, s.Trigger())  // starts
	running.Wait()                // ensure first run is in-flight
	require.False(t, s.Trigger()) // second rejected while busy
	close(release)
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestNewScheduler_BadTimezone(t *testing.T) {
	_, err := NewScheduler("0 21 * * *", "Not/AZone", func(context.Context, string) error { return nil }, slog.Default())
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/transport/scheduler/ -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Write minimal implementation**

`internal/transport/scheduler/scheduler.go`:
```go
package scheduler

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
)

type RunFunc func(ctx context.Context, trigger string) error

type Scheduler interface {
	Start() error
	Stop()
	Trigger() bool
}

type scheduler struct {
	cron    *cron.Cron
	spec    string
	run     RunFunc
	log     *slog.Logger
	running atomic.Bool
}

func NewScheduler(spec, tz string, run RunFunc, log *slog.Logger) (Scheduler, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, err
	}
	return &scheduler{
		cron: cron.New(cron.WithLocation(loc)),
		spec: spec,
		run:  run,
		log:  log,
	}, nil
}

func (s *scheduler) Start() error {
	_, err := s.cron.AddFunc(s.spec, func() { s.execute("cron") })
	if err != nil {
		return err
	}
	s.cron.Start()
	s.log.Info("scheduler started", "spec", s.spec)
	return nil
}

func (s *scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}

func (s *scheduler) Trigger() bool {
	if !s.running.CompareAndSwap(false, true) {
		return false
	}
	go func() {
		defer s.running.Store(false)
		if err := s.run(context.Background(), "manual"); err != nil {
			s.log.Error("manual run failed", "err", err)
		}
	}()
	return true
}

func (s *scheduler) execute(trigger string) {
	if !s.running.CompareAndSwap(false, true) {
		s.log.Warn("crawl already running, skipping", "trigger", trigger)
		return
	}
	defer s.running.Store(false)
	if err := s.run(context.Background(), trigger); err != nil {
		s.log.Error("scheduled run failed", "trigger", trigger, "err", err)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/transport/scheduler/ -v`
Expected: PASS (`go get github.com/robfig/cron/v3`).

- [ ] **Step 5: Commit**

```bash
git add internal/transport/scheduler go.mod go.sum
git commit -m "feat: cron scheduler with manual trigger"
```

### Task 5.2: Prometheus metrics + HTTP routes (/metrics, manual trigger)

**Files:**
- Create: `internal/observability/metrics.go`
- Modify: `internal/transport/http/server.go`, `internal/transport/http/server_test.go`

**Interfaces:**
- Produces:
  - `var (CrawlRunsTotal *prometheus.CounterVec; MessagesIngested prometheus.Counter; CrawlDuration prometheus.Histogram)` registered on init.
  - `func NewServer(addr string, log *slog.Logger, trigger func() bool) *http.Server` — adds `GET /metrics` and `POST /crawl` (calls `trigger()`; returns `202` if started, `409` if a run is in flight).

- [ ] **Step 1: Write the failing test** (extend server test)

Replace `internal/transport/http/server_test.go` with:
```go
package http

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHealthz(t *testing.T) {
	srv := NewServer(":0", slog.Default(), func() bool { return true })
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestMetricsExposed(t *testing.T) {
	srv := NewServer(":0", slog.Default(), func() bool { return true })
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestManualTrigger(t *testing.T) {
	srv := NewServer(":0", slog.Default(), func() bool { return true })
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/crawl", nil))
	require.Equal(t, http.StatusAccepted, rec.Code)

	busy := NewServer(":0", slog.Default(), func() bool { return false })
	rec2 := httptest.NewRecorder()
	busy.Handler.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/crawl", nil))
	require.Equal(t, http.StatusConflict, rec2.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/transport/http/ -v`
Expected: FAIL (signature mismatch / /metrics missing).

- [ ] **Step 3: Write minimal implementation**

`internal/observability/metrics.go`:
```go
package observability

import "github.com/prometheus/client_golang/prometheus"

var (
	CrawlRunsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "crawl_runs_total", Help: "Total crawl runs by status."},
		[]string{"status"},
	)
	MessagesIngested = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "messages_ingested_total", Help: "Total messages ingested."},
	)
	CrawlDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{Name: "crawl_duration_seconds", Help: "Crawl run duration.", Buckets: prometheus.ExponentialBuckets(1, 2, 12)},
	)
)

func init() {
	prometheus.MustRegister(CrawlRunsTotal, MessagesIngested, CrawlDuration)
}
```

`internal/transport/http/server.go`:
```go
package http

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewServer(addr string, log *slog.Logger, trigger func() bool) *http.Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.POST("/crawl", func(c *gin.Context) {
		if trigger() {
			c.JSON(http.StatusAccepted, gin.H{"status": "started"})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"status": "already running"})
	})
	return &http.Server{Addr: addr, Handler: r}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/transport/http/ -v`
Expected: PASS (`go get github.com/prometheus/client_golang/prometheus/promhttp`).

- [ ] **Step 5: Commit**

```bash
git add internal/observability/metrics.go internal/transport/http go.mod go.sum
git commit -m "feat: prometheus metrics and manual trigger route"
```

### Task 5.3: DI wiring in main + migrations on startup

**Files:**
- Modify: `cmd/crawler/main.go`
- Create: `internal/repository/migrate.go`

**Interfaces:**
- Produces: `func RunMigrations(dsn string) error` (embeds `db/migrations` via `goose`).
- Consumes: everything from Sprints 0–5. Wires `config` → `DB` → repos → `pancake.Client` → `token.Manager` → `business.PageProcessor` → `business.CrawlService` → `scheduler` → `http.NewServer`.

First add a `MigrationsDir` field to `config.Config` and its loader (default `db/migrations`):
- In `internal/config/config.go`, add `MigrationsDir string` to the struct and `MigrationsDir: getDefault("MIGRATIONS_DIR", "db/migrations"),` in `Load`.

- [ ] **Step 1: Write `internal/repository/migrate.go`**

```go
package repository

import (
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// RunMigrations applies all up migrations found in dir (filesystem path).
// dir is a directory on disk: "db/migrations" for local runs, "/db/migrations"
// inside the container (the Dockerfile copies migrations there and WORKDIR is /).
func RunMigrations(dsn, dir string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, dir)
}
```

> No `go:embed` — `embed` patterns cannot reference parent directories (`..`), and the package lives under `internal/repository` while migrations live in `db/migrations`. Reading from a filesystem dir avoids that limitation. The Dockerfile already `COPY`s `db/migrations` into the image.

- [ ] **Step 2: Write `cmd/crawler/inject.go`** (samber/do providers — the composition root)

```go
package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/samber/do/v2"

	"github.com/desi/crawl-conversations/internal/business"
	"github.com/desi/crawl-conversations/internal/config"
	"github.com/desi/crawl-conversations/internal/observability"
	"github.com/desi/crawl-conversations/internal/pancake"
	"github.com/desi/crawl-conversations/internal/repository"
	"github.com/desi/crawl-conversations/internal/token"
	"github.com/desi/crawl-conversations/internal/transport/scheduler"
)

// buildInjector registers every component as a provider and returns the container.
func buildInjector(cfg config.Config) *do.RootScope {
	i := do.New()

	do.ProvideValue(i, cfg)
	do.ProvideValue(i, observability.NewLogger(cfg.LogLevel))

	do.Provide(i, func(in do.Injector) (*repository.DB, error) {
		return repository.New(context.Background(), do.MustInvoke[config.Config](in).DatabaseURL)
	})
	do.Provide(i, func(in do.Injector) (*redis.Client, error) {
		opt, err := redis.ParseURL(do.MustInvoke[config.Config](in).RedisURL)
		if err != nil {
			return nil, err
		}
		return redis.NewClient(opt), nil
	})
	do.Provide(i, func(in do.Injector) (*pancake.RateLimiter, error) {
		c := do.MustInvoke[config.Config](in)
		return pancake.NewRateLimiter(c.RateLimitPerPageRPS, c.RateLimitPerPageBurst), nil
	})
	do.Provide(i, func(in do.Injector) (pancake.Client, error) {
		c := do.MustInvoke[config.Config](in)
		rl := do.MustInvoke[*pancake.RateLimiter](in)
		// NewClient(pageBaseURL, accountBaseURL, accessToken, rl, hc) — see api-addendum
		return pancake.NewClient(c.PancakeBaseURL, c.PancakeAccountBaseURL, c.PancakeAccessToken, rl, nil), nil
	})
	do.Provide(i, func(in do.Injector) (token.Manager, error) {
		c := do.MustInvoke[config.Config](in)
		return token.NewManager(do.MustInvoke[*redis.Client](in), do.MustInvoke[pancake.Client](in), c.TokenTTL), nil
	})
	do.Provide(i, func(in do.Injector) (business.Repos, error) {
		db := do.MustInvoke[*repository.DB](in)
		return business.Repos{
			Page:         repository.NewPageRepository(db),
			Customer:     repository.NewCustomerRepository(db),
			Tag:          repository.NewTagRepository(db),
			Conversation: repository.NewConversationRepository(db),
			Message:      repository.NewMessageRepository(db),
			Checkpoint:   repository.NewCheckpointRepository(db),
		}, nil
	})
	do.Provide(i, func(in do.Injector) (business.PageProcessor, error) {
		return business.NewPageProcessor(
			do.MustInvoke[pancake.Client](in),
			do.MustInvoke[token.Manager](in),
			do.MustInvoke[business.Repos](in),
			5*time.Second,
		), nil
	})
	do.Provide(i, func(in do.Injector) (business.CrawlService, error) {
		c := do.MustInvoke[config.Config](in)
		db := do.MustInvoke[*repository.DB](in)
		return business.NewCrawlService(
			do.MustInvoke[pancake.Client](in),
			do.MustInvoke[business.PageProcessor](in),
			do.MustInvoke[business.Repos](in),
			repository.NewCrawlRunRepository(db),
			c.WorkerPoolSize,
			do.MustInvoke[*slog.Logger](in),
		), nil
	})
	do.Provide(i, func(in do.Injector) (scheduler.Scheduler, error) {
		c := do.MustInvoke[config.Config](in)
		svc := do.MustInvoke[business.CrawlService](in)
		return scheduler.NewScheduler(c.CronSchedule, c.CronTimezone, svc.Run, do.MustInvoke[*slog.Logger](in))
	})

	return i
}
```

> `*repository.DB` and `*redis.Client` expose `Close()`; if you want `do.Shutdown` to close them, have `repository.DB` and a thin redis wrapper implement `do.Shutdownable`. For this service, explicit `defer` in `main` (below) is sufficient.

- [ ] **Step 3: Rewrite `cmd/crawler/main.go`** (resolve from the container, run server + scheduler)

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/samber/do/v2"

	"github.com/desi/crawl-conversations/internal/business"
	"github.com/desi/crawl-conversations/internal/config"
	"github.com/desi/crawl-conversations/internal/repository"
	"github.com/desi/crawl-conversations/internal/transport/scheduler"
	httpx "github.com/desi/crawl-conversations/internal/transport/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	if err := repository.RunMigrations(cfg.DatabaseURL, cfg.MigrationsDir); err != nil {
		panic(err)
	}

	injector := buildInjector(cfg)
	defer func() { _ = injector.Shutdown() }()

	log := do.MustInvoke[*slog.Logger](injector)
	db := do.MustInvoke[*repository.DB](injector)
	defer db.Close()

	sched := do.MustInvoke[scheduler.Scheduler](injector)
	if err := sched.Start(); err != nil {
		log.Error("scheduler start failed", "err", err)
		os.Exit(1)
	}
	defer sched.Stop()

	// touch crawl service so a wiring error surfaces at startup, not at 21:00
	_ = do.MustInvoke[business.CrawlService](injector)

	srv := httpx.NewServer(cfg.HTTPAddr, log, sched.Trigger)
	go func() {
		log.Info("http server starting", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server failed", "err", err)
			os.Exit(1)
		}
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-sigCtx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Info("shutdown complete")
}
```

> `go get github.com/samber/do/v2`. The provider for `scheduler.Scheduler` builds the `CrawlService` transitively, so resolving the scheduler wires the whole graph.

- [ ] **Step 4: Build and smoke-test against compose**

Run:
```bash
docker compose -f deploy/docker-compose.yml up -d postgres redis
set -a; source .env; set +a   # .env must hold a real PANCAKE_ACCESS_TOKEN
go build ./... && go run ./cmd/crawler &
sleep 3
curl -s localhost:8080/healthz
curl -s -XPOST localhost:8080/crawl   # 202 then crawl runs; check logs + DB row counts
```
Expected: healthz ok, `/crawl` returns `202`, `crawl_runs` row created with status `success` (or `failed` with a logged error), data rows appear.

- [ ] **Step 5: Run the full unit suite**

Run: `go test ./... -race -count=1`
Expected: PASS (integration tests skip without `TEST_DATABASE_URL`).

- [ ] **Step 6: Commit**

```bash
git add cmd/crawler/main.go cmd/crawler/inject.go internal/repository/migrate.go internal/config/config.go go.mod go.sum
git commit -m "feat: di wiring with samber/do and startup migrations"
```

### Task 5.4: Dockerfile, compose service, README

**Files:**
- Create: `deploy/Dockerfile`, `README.md`
- Modify: `deploy/docker-compose.yml`

**Interfaces:**
- Produces: a runnable container image and a one-command `docker compose up`.

- [ ] **Step 1: Create `deploy/Dockerfile` (multi-stage)**

```dockerfile
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/crawler ./cmd/crawler

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/crawler /crawler
COPY --from=build /src/db/migrations /db/migrations
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/crawler"]
```

> If the embed path in `migrate.go` was changed to `internal/repository/migrations`, drop the migrations COPY line here (they're embedded in the binary).

- [ ] **Step 2: Add the `crawler` service to `deploy/docker-compose.yml`**

Append under `services:`:
```yaml
  crawler:
    build:
      context: ..
      dockerfile: deploy/Dockerfile
    environment:
      PANCAKE_ACCESS_TOKEN: ${PANCAKE_ACCESS_TOKEN}
      PANCAKE_BASE_URL: ${PANCAKE_BASE_URL:-https://pages.fm/api/public_api/v1}
      DATABASE_URL: postgres://crawler:crawler@postgres:5432/crawler?sslmode=disable
      REDIS_URL: redis://redis:6379/0
      CRON_SCHEDULE: "0 21 * * *"
      CRON_TIMEZONE: Asia/Ho_Chi_Minh
      WORKER_POOL_SIZE: "8"
      HTTP_ADDR: ":8080"
    ports: ["8080:8080"]
    depends_on:
      postgres: { condition: service_healthy }
      redis: { condition: service_healthy }
```

- [ ] **Step 3: Create `README.md`**

```markdown
# Pancake Conversation Crawler

Crawls all Pancake (pages.fm) pages, conversations, messages, tags and customers into PostgreSQL. Runs daily at 21:00 (Asia/Ho_Chi_Minh); supports manual trigger.

## Run (docker compose)
1. `cp .env.example .env` and set `PANCAKE_ACCESS_TOKEN`.
2. `docker compose -f deploy/docker-compose.yml up --build`
3. Health: `curl localhost:8080/healthz` · Metrics: `curl localhost:8080/metrics`
4. Trigger now: `curl -XPOST localhost:8080/crawl`

## Develop
- `make test` — unit tests. Set `TEST_DATABASE_URL` to run repository integration tests.
- `make migrate-up` — apply migrations.
- Architecture & sprint status: `docs/2026-06-26-pancake-crawler-design.md`.

## Behavior
- First run per page = full backfill (pgx COPY). Later runs = incremental from checkpoint (overlap window).
- De-dup invariant: one `pancake_message_id` → one row, enforced by `UNIQUE(pancake_message_id, inserted_at)` + `ON CONFLICT DO NOTHING`.
```

- [ ] **Step 4: Verify full stack builds and runs**

Run:
```bash
echo "PANCAKE_ACCESS_TOKEN=<real>" > .env
docker compose -f deploy/docker-compose.yml up --build -d
sleep 5 && curl -s localhost:8080/healthz
curl -s -XPOST localhost:8080/crawl
docker compose -f deploy/docker-compose.yml logs crawler | tail -20
```
Expected: container healthy, crawl runs, DB populated.

- [ ] **Step 5: Commit**

```bash
git add deploy/Dockerfile deploy/docker-compose.yml README.md
git commit -m "feat: dockerfile, compose service and readme"
```

**Sprint 5 done note:** record under spec §12 Sprint 5 what was done / deferred.

---

## Final verification checklist
- [ ] `go build ./...` and `go vet ./...` clean.
- [ ] `go test ./... -race` green (unit); repository integration green with `TEST_DATABASE_URL`.
- [ ] `golangci-lint run` clean.
- [ ] De-dup invariant test passes (Tasks 3.2, 3.3).
- [ ] `docker compose up --build` brings up app + postgres + redis; `/healthz` ok; manual `/crawl` populates DB; second `/crawl` adds no duplicate messages.
- [ ] Spec §12 sprint notes updated with done/deferred items.

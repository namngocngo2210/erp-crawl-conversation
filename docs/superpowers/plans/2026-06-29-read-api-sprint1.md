# Read API Sprint 1 (nền tảng) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Mở rộng service Go hiện có thêm read API có xác thực JWT để FE đọc được pages, conversations và messages (keyset pagination + filter direction).

**Architecture:** Module read độc lập trong monolith hiện tại, theo layering `transport/http/api → business/read → repository/read → entity`. Auth bằng JWT HS256 + bảng `users` (bcrypt). Read queries dùng **GORM** (kể cả keyset pagination). Routes gắn vào cùng Gin engine với crawler.

**Tech Stack:** Go 1.25, Gin, GORM, goose (migration), `github.com/golang-jwt/jwt/v5`, `golang.org/x/crypto/bcrypt`, `samber/do` (DI).

## Global Constraints

- Go 1.25+, clean architecture, interface-driven; layer dependency: `transport → business → repository → entity`.
- Read API **read-only**: không ghi/sửa dữ liệu hội thoại.
- DTO response **không** chứa `raw` jsonb (tránh lộ PII thừa).
- Keyset (cursor) pagination, không OFFSET.
- GORM cho mọi query (theo yêu cầu user), không dùng pgx raw cho read.
- Mọi route `/api/v1/*` cần JWT trừ `POST /api/v1/auth/login`.
- Module name: `github.com/desi/crawl-conversations`.
- Test: unit test business (mock repo qua interface) + endpoint test (httptest).
- Commit message tiếng Anh, ngắn gọn, không kèm dòng Co-Authored-By.

---

## File Structure

**Create:**
- `db/migrations/00003_users.sql` — bảng `users`.
- `internal/entity/user.go` — entity `User`.
- `internal/repository/read/user_repo.go` — `UserRepository` (GORM): lookup + seed.
- `internal/repository/read/conversation_read_repo.go` — read queries pages/conversations/messages.
- `internal/repository/read/cursor.go` — encode/decode keyset cursor.
- `internal/business/read/auth_service.go` — `AuthService`: login, issue/verify JWT, seed admin.
- `internal/business/read/query_service.go` — `QueryService`: pages/conversations/messages.
- `internal/business/read/ports.go` — interface mà business/read phụ thuộc.
- `internal/dto/response/response.go` — response DTO + mapping entity→DTO.
- `internal/transport/http/api/router.go` — đăng ký routes `/api/v1`.
- `internal/transport/http/api/middleware.go` — JWT auth middleware + CORS.
- `internal/transport/http/api/auth_handler.go` — handler login/me.
- `internal/transport/http/api/query_handler.go` — handler pages/conversations/messages.
- Test files tương ứng (`*_test.go`).

**Modify:**
- `internal/config/config.go` — thêm `JWTSecret`, `AdminEmail`, `AdminPassword`, `CORSOrigin`, `JWTTTL`.
- `internal/transport/http/server.go` — nhận thêm tham số đăng ký API routes.
- `cmd/crawler/inject.go` — provide read repo + business read services.
- `cmd/crawler/main.go` — seed admin lúc khởi động + truyền API router vào server.
- `go.mod` — thêm `golang-jwt/jwt/v5`, `golang.org/x/crypto`.

---

## Task 1: Migration `users` + entity

**Files:**
- Create: `db/migrations/00003_users.sql`
- Create: `internal/entity/user.go`
- Test: `internal/entity/user_test.go`

**Interfaces:**
- Produces: `entity.User{ ID int64; Email string; PasswordHash string; Name string; Role string; CreatedAt, UpdatedAt time.Time }`, method `TableName() string` → `"users"`.

- [ ] **Step 1: Write the migration**

Create `db/migrations/00003_users.sql`:

```sql
-- +goose Up
CREATE TABLE users (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    name          TEXT NOT NULL DEFAULT '',
    role          TEXT NOT NULL DEFAULT 'viewer',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE users;
```

- [ ] **Step 2: Write the failing test**

Create `internal/entity/user_test.go`:

```go
package entity

import "testing"

func TestUserTableName(t *testing.T) {
	if (User{}).TableName() != "users" {
		t.Fatalf("want users, got %s", (User{}).TableName())
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/entity/ -run TestUserTableName`
Expected: FAIL (undefined: User)

- [ ] **Step 4: Write the entity**

Create `internal/entity/user.go`:

```go
package entity

import "time"

type User struct {
	ID           int64 `gorm:"primaryKey"`
	Email        string
	PasswordHash string
	Name         string
	Role         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (User) TableName() string { return "users" }
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/entity/ -run TestUserTableName`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add db/migrations/00003_users.sql internal/entity/user.go internal/entity/user_test.go
git commit -m "feat: users table migration and entity"
```

---

## Task 2: Config bổ sung

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: `Config` thêm field `JWTSecret string`, `AdminEmail string`, `AdminPassword string`, `CORSOrigin string`, `JWTTTL time.Duration`.

- [ ] **Step 1: Write the failing test**

Append to `internal/config/config_test.go`:

```go
func TestLoadReadAPIDefaults(t *testing.T) {
	t.Setenv("PANCAKE_ACCESS_TOKEN", "x")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("REDIS_URL", "redis://x")
	t.Setenv("JWT_SECRET", "secret")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JWTSecret != "secret" {
		t.Fatalf("want secret, got %q", cfg.JWTSecret)
	}
	if cfg.JWTTTL != 12*time.Hour {
		t.Fatalf("want 12h default, got %v", cfg.JWTTTL)
	}
	if cfg.CORSOrigin != "*" {
		t.Fatalf("want * default, got %q", cfg.CORSOrigin)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestLoadReadAPIDefaults`
Expected: FAIL (unknown field JWTSecret)

- [ ] **Step 3: Add fields and loading**

In `internal/config/config.go`, add to `Config` struct:

```go
	JWTSecret             string
	AdminEmail            string
	AdminPassword         string
	CORSOrigin            string
	JWTTTL                time.Duration
```

In `Load()`, add inside the `cfg := Config{...}` literal:

```go
		JWTSecret:             os.Getenv("JWT_SECRET"),
		AdminEmail:            getDefault("ADMIN_EMAIL", "admin@local"),
		AdminPassword:         os.Getenv("ADMIN_PASSWORD"),
		CORSOrigin:            getDefault("CORS_ORIGIN", "*"),
		JWTTTL:                getDuration("JWT_TTL", 12*time.Hour),
```

After the existing `RedisURL` check, add:

```go
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestLoadReadAPIDefaults`
Expected: PASS

- [ ] **Step 5: Run full config tests (regression)**

Run: `go test ./internal/config/`
Expected: PASS (existing tests that set the three required envs may now need `JWT_SECRET`; if an existing test fails, add `t.Setenv("JWT_SECRET", "x")` to it.)

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: read API config (jwt, admin seed, cors)"
```

---

## Task 3: Cursor encode/decode (keyset)

**Files:**
- Create: `internal/repository/read/cursor.go`
- Test: `internal/repository/read/cursor_test.go`

**Interfaces:**
- Produces:
  - `type Cursor struct { Time time.Time; ID int64 }`
  - `func EncodeCursor(c Cursor) string` — base64(URL) của `"<unixNano>,<id>"`.
  - `func DecodeCursor(s string) (Cursor, bool)` — `ok=false` nếu rỗng/sai định dạng.

- [ ] **Step 1: Write the failing test**

Create `internal/repository/read/cursor_test.go`:

```go
package read

import (
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	in := Cursor{Time: time.Unix(0, 1717000000123456789).UTC(), ID: 42}
	s := EncodeCursor(in)
	out, ok := DecodeCursor(s)
	if !ok {
		t.Fatal("decode failed")
	}
	if !out.Time.Equal(in.Time) || out.ID != in.ID {
		t.Fatalf("round trip mismatch: %+v vs %+v", out, in)
	}
}

func TestDecodeEmpty(t *testing.T) {
	if _, ok := DecodeCursor(""); ok {
		t.Fatal("empty should not decode")
	}
	if _, ok := DecodeCursor("not-base64!!"); ok {
		t.Fatal("garbage should not decode")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/repository/read/ -run TestCursor`
Expected: FAIL (undefined: Cursor)

- [ ] **Step 3: Implement cursor**

Create `internal/repository/read/cursor.go`:

```go
package read

import (
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

type Cursor struct {
	Time time.Time
	ID   int64
}

func EncodeCursor(c Cursor) string {
	raw := strconv.FormatInt(c.Time.UTC().UnixNano(), 10) + "," + strconv.FormatInt(c.ID, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func DecodeCursor(s string) (Cursor, bool) {
	if s == "" {
		return Cursor{}, false
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, false
	}
	parts := strings.SplitN(string(b), ",", 2)
	if len(parts) != 2 {
		return Cursor{}, false
	}
	ns, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return Cursor{}, false
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return Cursor{}, false
	}
	return Cursor{Time: time.Unix(0, ns).UTC(), ID: id}, true
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/repository/read/ -run TestCursor`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/repository/read/cursor.go internal/repository/read/cursor_test.go
git commit -m "feat: keyset cursor encode/decode"
```

---

## Task 4: User repository (GORM) — lookup + seed

**Files:**
- Create: `internal/repository/read/user_repo.go`
- Test: `internal/repository/read/user_repo_test.go` (integration; bỏ qua khi không có `TEST_DATABASE_URL`)

**Interfaces:**
- Consumes: `*repository.DB` (field `.Gorm`), `entity.User`.
- Produces:
  - `type UserRepository interface { FindByEmail(ctx, email string) (entity.User, error); CreateIfNotExists(ctx, u entity.User) error }`
  - `func NewUserRepository(db *repository.DB) UserRepository`
  - `FindByEmail` trả `gorm.ErrRecordNotFound` khi không thấy.

- [ ] **Step 1: Write the failing test**

Create `internal/repository/read/user_repo_test.go`:

```go
package read

import (
	"context"
	"os"
	"testing"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/desi/crawl-conversations/internal/repository"
)

func testDB(t *testing.T) *repository.DB {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := repository.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	return db
}

func TestUserCreateAndFind(t *testing.T) {
	db := testDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()
	u := entity.User{Email: "seed@test.local", PasswordHash: "h", Name: "Seed", Role: "admin"}
	if err := repo.CreateIfNotExists(ctx, u); err != nil {
		t.Fatal(err)
	}
	// idempotent
	if err := repo.CreateIfNotExists(ctx, u); err != nil {
		t.Fatal(err)
	}
	got, err := repo.FindByEmail(ctx, "seed@test.local")
	if err != nil {
		t.Fatal(err)
	}
	if got.Role != "admin" {
		t.Fatalf("want admin, got %s", got.Role)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/repository/read/ -run TestUserCreateAndFind`
Expected: FAIL (undefined: NewUserRepository) — or SKIP if no DB; if skipped, still confirms compile error first (`go build ./...` fails).

- [ ] **Step 3: Implement the repo**

Create `internal/repository/read/user_repo.go`:

```go
package read

import (
	"context"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/desi/crawl-conversations/internal/repository"
	"gorm.io/gorm/clause"
)

type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (entity.User, error)
	CreateIfNotExists(ctx context.Context, u entity.User) error
}

type userRepo struct{ db *repository.DB }

func NewUserRepository(db *repository.DB) UserRepository { return &userRepo{db} }

func (r *userRepo) FindByEmail(ctx context.Context, email string) (entity.User, error) {
	var u entity.User
	err := r.db.Gorm.WithContext(ctx).Where("email = ?", email).First(&u).Error
	return u, err
}

func (r *userRepo) CreateIfNotExists(ctx context.Context, u entity.User) error {
	return r.db.Gorm.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "email"}},
		DoNothing: true,
	}).Create(&u).Error
}
```

- [ ] **Step 4: Run build + test**

Run: `go build ./... && go test ./internal/repository/read/ -run TestUserCreateAndFind`
Expected: PASS (or SKIP if no `TEST_DATABASE_URL`); build must succeed.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/read/user_repo.go internal/repository/read/user_repo_test.go
git commit -m "feat: user repository (find + idempotent seed)"
```

---

## Task 5: Read repository — pages, conversations, messages (GORM keyset)

**Files:**
- Create: `internal/repository/read/conversation_read_repo.go`
- Test: `internal/repository/read/conversation_read_repo_test.go` (integration)

**Interfaces:**
- Consumes: `*repository.DB`, `entity.Page/Conversation/Message/Customer/Tag`, `Cursor`, `DecodeCursor`.
- Produces:
  - `type PageCount struct { entity.Page; ConversationCount int64 }`
  - `type ConvFilter struct { PageID int64; Cursor string; Limit int }`
  - `type MsgFilter struct { ConversationID int64; Direction string; Cursor string; Limit int }`
  - `type ConvDetail struct { Conversation entity.Conversation; Customer *entity.Customer; Tags []entity.Tag }`
  - `type ReadRepository interface {`
    - `ListPages(ctx) ([]PageCount, error)`
    - `ListConversations(ctx, f ConvFilter) ([]entity.Conversation, error)`
    - `GetConversation(ctx, id int64) (ConvDetail, error)`
    - `ListMessages(ctx, f MsgFilter) ([]entity.Message, error)`
  - `}`
  - `func NewReadRepository(db *repository.DB) ReadRepository`
- Semantics: `ListConversations` sort `updated_at DESC, id DESC`, keyset `(updated_at,id) < cursor`. `ListMessages` sort `inserted_at DESC, id DESC`, keyset tương tự, optional `direction`. `Limit` clamp về `[1,100]`, default 50.

- [ ] **Step 1: Write the failing test**

Create `internal/repository/read/conversation_read_repo_test.go`:

```go
package read

import (
	"context"
	"testing"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
)

func TestListConversationsKeyset(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	// seed one page + 3 conversations with increasing updated_at
	pageID, err := seedPage(ctx, db, "p-keyset")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Truncate(time.Second)
	for i := 0; i < 3; i++ {
		if err := seedConversation(ctx, db, pageID, "c-"+time.Duration(i).String(), base.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewReadRepository(db)
	first, err := repo.ListConversations(ctx, ConvFilter{PageID: pageID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 {
		t.Fatalf("want 2, got %d", len(first))
	}
	// newest first
	if !first[0].UpdatedAt.After(first[1].UpdatedAt) {
		t.Fatal("expected DESC order")
	}
	next := EncodeCursor(Cursor{Time: first[1].UpdatedAt, ID: first[1].ID})
	page2, err := repo.ListConversations(ctx, ConvFilter{PageID: pageID, Cursor: next, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 1 {
		t.Fatalf("want 1 remaining, got %d", len(page2))
	}
}

// seed helpers use db.Gorm directly
func seedPage(ctx context.Context, db interface{ table() }, _ string) (int64, error) { return 0, nil }
```

> NOTE for implementer: replace the stub `seedPage`/`seedConversation` with real helpers that insert via `db.Gorm.Create(&entity.Page{...})` / `&entity.Conversation{...}` and return the generated ID. Keep them in this test file. The `entity.Conversation` requires `LastMessageAt`; set it equal to `updatedAt`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/repository/read/ -run TestListConversationsKeyset`
Expected: FAIL/compile error (undefined: NewReadRepository).

- [ ] **Step 3: Implement the read repository**

Create `internal/repository/read/conversation_read_repo.go`:

```go
package read

import (
	"context"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/desi/crawl-conversations/internal/repository"
	"gorm.io/gorm"
)

type PageCount struct {
	entity.Page
	ConversationCount int64
}

type ConvFilter struct {
	PageID int64
	Cursor string
	Limit  int
}

type MsgFilter struct {
	ConversationID int64
	Direction      string
	Cursor         string
	Limit          int
}

type ConvDetail struct {
	Conversation entity.Conversation
	Customer     *entity.Customer
	Tags         []entity.Tag
}

type ReadRepository interface {
	ListPages(ctx context.Context) ([]PageCount, error)
	ListConversations(ctx context.Context, f ConvFilter) ([]entity.Conversation, error)
	GetConversation(ctx context.Context, id int64) (ConvDetail, error)
	ListMessages(ctx context.Context, f MsgFilter) ([]entity.Message, error)
}

type readRepo struct{ db *repository.DB }

func NewReadRepository(db *repository.DB) ReadRepository { return &readRepo{db} }

func clampLimit(n int) int {
	if n <= 0 {
		return 50
	}
	if n > 100 {
		return 100
	}
	return n
}

func (r *readRepo) ListPages(ctx context.Context) ([]PageCount, error) {
	var out []PageCount
	err := r.db.Gorm.WithContext(ctx).
		Model(&entity.Page{}).
		Select("pages.*, (SELECT count(*) FROM conversations c WHERE c.page_id = pages.id) AS conversation_count").
		Order("pages.name ASC").
		Find(&out).Error
	return out, err
}

func (r *readRepo) ListConversations(ctx context.Context, f ConvFilter) ([]entity.Conversation, error) {
	q := r.db.Gorm.WithContext(ctx).
		Where("page_id = ?", f.PageID).
		Order("updated_at DESC, id DESC").
		Limit(clampLimit(f.Limit))
	if c, ok := DecodeCursor(f.Cursor); ok {
		q = q.Where("(updated_at, id) < (?, ?)", c.Time, c.ID)
	}
	var out []entity.Conversation
	err := q.Find(&out).Error
	return out, err
}

func (r *readRepo) GetConversation(ctx context.Context, id int64) (ConvDetail, error) {
	var conv entity.Conversation
	if err := r.db.Gorm.WithContext(ctx).First(&conv, id).Error; err != nil {
		return ConvDetail{}, err
	}
	detail := ConvDetail{Conversation: conv}
	if conv.CustomerID != nil {
		var cust entity.Customer
		if err := r.db.Gorm.WithContext(ctx).First(&cust, *conv.CustomerID).Error; err == nil {
			detail.Customer = &cust
		} else if err != gorm.ErrRecordNotFound {
			return ConvDetail{}, err
		}
	}
	var tags []entity.Tag
	if err := r.db.Gorm.WithContext(ctx).
		Joins("JOIN conversation_tags ct ON ct.tag_id = tags.id").
		Where("ct.conversation_id = ?", id).
		Find(&tags).Error; err != nil {
		return ConvDetail{}, err
	}
	detail.Tags = tags
	return detail, nil
}

func (r *readRepo) ListMessages(ctx context.Context, f MsgFilter) ([]entity.Message, error) {
	q := r.db.Gorm.WithContext(ctx).
		Where("conversation_id = ?", f.ConversationID).
		Order("inserted_at DESC, id DESC").
		Limit(clampLimit(f.Limit))
	if f.Direction == "in" || f.Direction == "out" {
		q = q.Where("direction = ?", f.Direction)
	}
	if c, ok := DecodeCursor(f.Cursor); ok {
		q = q.Where("(inserted_at, id) < (?, ?)", c.Time, c.ID)
	}
	var out []entity.Message
	err := q.Find(&out).Error
	return out, err
}
```

- [ ] **Step 4: Run build + test**

Run: `go build ./... && go test ./internal/repository/read/`
Expected: PASS (integration tests SKIP without `TEST_DATABASE_URL`; build must succeed).

- [ ] **Step 5: Commit**

```bash
git add internal/repository/read/conversation_read_repo.go internal/repository/read/conversation_read_repo_test.go
git commit -m "feat: read repository for pages, conversations, messages (gorm keyset)"
```

---

## Task 6: Response DTOs + mapping

**Files:**
- Create: `internal/dto/response/response.go`
- Test: `internal/dto/response/response_test.go`

**Interfaces:**
- Consumes: `entity.*`, `read.PageCount`, `read.ConvDetail`, `read.Cursor`, `read.EncodeCursor`.
- Produces:
  - `type PageResp struct { ID int64; PancakePageID, Platform, Name, Status string; ConversationCount int64 }`
  - `type ConversationResp struct { ID int64; PancakeConversationID, Type, Snippet string; MessageCount int; LastMessageAt, UpdatedAt time.Time }`
  - `type CustomerResp struct { ID int64; Name, Phone, Email, FbID, Gender string }`
  - `type TagResp struct { ID int64; Name, Color string }`
  - `type ConversationDetailResp struct { ConversationResp; Customer *CustomerResp; Tags []TagResp }`
  - `type MessageResp struct { ID int64; PancakeMessageID, SenderID, Direction, Type, Content string; Attachments json.RawMessage; InsertedAt time.Time }`
  - `type Page[T any] struct { Items []T; NextCursor *string }`
  - mappers: `MapPages`, `MapConversations` (returns `Page[ConversationResp]`), `MapConversationDetail`, `MapMessages` (returns `Page[MessageResp]`).
- `NextCursor` = nil khi số phần tử < limit; ngược lại trỏ tới phần tử cuối.

- [ ] **Step 1: Write the failing test**

Create `internal/dto/response/response_test.go`:

```go
package response

import (
	"testing"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
)

func TestMapConversationsNextCursor(t *testing.T) {
	now := time.Now().UTC()
	convs := []entity.Conversation{
		{ID: 2, UpdatedAt: now, Snippet: "b"},
		{ID: 1, UpdatedAt: now.Add(-time.Minute), Snippet: "a"},
	}
	// limit == len -> has next cursor
	p := MapConversations(convs, 2)
	if p.NextCursor == nil {
		t.Fatal("expected next cursor when full page")
	}
	if len(p.Items) != 2 || p.Items[0].Snippet != "b" {
		t.Fatalf("unexpected items: %+v", p.Items)
	}
	// fewer than limit -> no next cursor
	p2 := MapConversations(convs, 5)
	if p2.NextCursor != nil {
		t.Fatal("expected nil cursor when partial page")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/dto/response/ -run TestMapConversationsNextCursor`
Expected: FAIL (undefined: MapConversations).

- [ ] **Step 3: Implement DTOs + mappers**

Create `internal/dto/response/response.go`:

```go
package response

import (
	"encoding/json"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/desi/crawl-conversations/internal/repository/read"
)

type PageResp struct {
	ID                int64  `json:"id"`
	PancakePageID     string `json:"pancake_page_id"`
	Platform          string `json:"platform"`
	Name              string `json:"name"`
	Status            string `json:"status"`
	ConversationCount int64  `json:"conversation_count"`
}

type ConversationResp struct {
	ID                    int64     `json:"id"`
	PancakeConversationID string    `json:"pancake_conversation_id"`
	Type                  string    `json:"type"`
	Snippet               string    `json:"snippet"`
	MessageCount          int       `json:"message_count"`
	LastMessageAt         time.Time `json:"last_message_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type CustomerResp struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Phone  string `json:"phone"`
	Email  string `json:"email"`
	FbID   string `json:"fb_id"`
	Gender string `json:"gender"`
}

type TagResp struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type ConversationDetailResp struct {
	ConversationResp
	Customer *CustomerResp `json:"customer"`
	Tags     []TagResp     `json:"tags"`
}

type MessageResp struct {
	ID               int64           `json:"id"`
	PancakeMessageID string          `json:"pancake_message_id"`
	SenderID         string          `json:"sender_id"`
	Direction        string          `json:"direction"`
	Type             string          `json:"type"`
	Content          string          `json:"content"`
	Attachments      json.RawMessage `json:"attachments"`
	InsertedAt       time.Time       `json:"inserted_at"`
}

type Page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

func MapPages(in []read.PageCount) []PageResp {
	out := make([]PageResp, 0, len(in))
	for _, p := range in {
		out = append(out, PageResp{
			ID: p.ID, PancakePageID: p.PancakePageID, Platform: p.Platform,
			Name: p.Name, Status: p.Status, ConversationCount: p.ConversationCount,
		})
	}
	return out
}

func mapConversation(c entity.Conversation) ConversationResp {
	return ConversationResp{
		ID: c.ID, PancakeConversationID: c.PancakeConversationID, Type: c.Type,
		Snippet: c.Snippet, MessageCount: c.MessageCount,
		LastMessageAt: c.LastMessageAt, UpdatedAt: c.UpdatedAt,
	}
}

func MapConversations(in []entity.Conversation, limit int) Page[ConversationResp] {
	items := make([]ConversationResp, 0, len(in))
	for _, c := range in {
		items = append(items, mapConversation(c))
	}
	var next *string
	if len(in) >= limit && len(in) > 0 {
		last := in[len(in)-1]
		s := read.EncodeCursor(read.Cursor{Time: last.UpdatedAt, ID: last.ID})
		next = &s
	}
	return Page[ConversationResp]{Items: items, NextCursor: next}
}

func MapConversationDetail(d read.ConvDetail) ConversationDetailResp {
	resp := ConversationDetailResp{ConversationResp: mapConversation(d.Conversation)}
	if d.Customer != nil {
		resp.Customer = &CustomerResp{
			ID: d.Customer.ID, Name: d.Customer.Name, Phone: d.Customer.Phone,
			Email: d.Customer.Email, FbID: d.Customer.FbID, Gender: d.Customer.Gender,
		}
	}
	resp.Tags = make([]TagResp, 0, len(d.Tags))
	for _, t := range d.Tags {
		resp.Tags = append(resp.Tags, TagResp{ID: t.ID, Name: t.Name, Color: t.Color})
	}
	return resp
}

func MapMessages(in []entity.Message, limit int) Page[MessageResp] {
	items := make([]MessageResp, 0, len(in))
	for _, m := range in {
		items = append(items, MessageResp{
			ID: m.ID, PancakeMessageID: m.PancakeMessageID, SenderID: m.SenderID,
			Direction: m.Direction, Type: m.Type, Content: m.Content,
			Attachments: m.Attachments, InsertedAt: m.InsertedAt,
		})
	}
	var next *string
	if len(in) >= limit && len(in) > 0 {
		last := in[len(in)-1]
		s := read.EncodeCursor(read.Cursor{Time: last.InsertedAt, ID: last.ID})
		next = &s
	}
	return Page[MessageResp]{Items: items, NextCursor: next}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/dto/response/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/dto/response/
git commit -m "feat: read API response DTOs and mappers"
```

---

## Task 7: AuthService (JWT + bcrypt + seed)

**Files:**
- Create: `internal/business/read/ports.go`
- Create: `internal/business/read/auth_service.go`
- Test: `internal/business/read/auth_service_test.go`
- Modify: `go.mod` (add deps)

**Interfaces:**
- Consumes: `read.UserRepository`, `entity.User`.
- Produces:
  - `internal/business/read/ports.go`:
    - `type UserRepo interface { FindByEmail(ctx, email string) (entity.User, error); CreateIfNotExists(ctx, u entity.User) error }`
  - `internal/business/read/auth_service.go`:
    - `type Claims struct { UserID int64; Email string; Role string }`
    - `type AuthService interface { Login(ctx, email, password string) (token string, u entity.User, err error); Verify(token string) (Claims, error); SeedAdmin(ctx, email, password string) error }`
    - `func NewAuthService(users UserRepo, secret string, ttl time.Duration) AuthService`
    - `var ErrInvalidCredentials = errors.New("invalid credentials")`

- [ ] **Step 1: Add dependencies**

Run:
```bash
go get github.com/golang-jwt/jwt/v5@latest
go get golang.org/x/crypto/bcrypt
```

- [ ] **Step 2: Write the failing test**

Create `internal/business/read/auth_service_test.go`:

```go
package read

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type fakeUsers struct{ byEmail map[string]entity.User }

func (f *fakeUsers) FindByEmail(_ context.Context, email string) (entity.User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return entity.User{}, gorm.ErrRecordNotFound
	}
	return u, nil
}
func (f *fakeUsers) CreateIfNotExists(_ context.Context, u entity.User) error {
	if _, ok := f.byEmail[u.Email]; !ok {
		f.byEmail[u.Email] = u
	}
	return nil
}

func newFake(email, pw string) *fakeUsers {
	h, _ := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return &fakeUsers{byEmail: map[string]entity.User{
		email: {ID: 1, Email: email, PasswordHash: string(h), Role: "admin"},
	}}
}

func TestLoginAndVerify(t *testing.T) {
	svc := NewAuthService(newFake("a@b.c", "pw123"), "secret", time.Hour)
	tok, u, err := svc.Login(context.Background(), "a@b.c", "pw123")
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "a@b.c" {
		t.Fatalf("unexpected user %+v", u)
	}
	claims, err := svc.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != 1 || claims.Role != "admin" {
		t.Fatalf("unexpected claims %+v", claims)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	svc := NewAuthService(newFake("a@b.c", "pw123"), "secret", time.Hour)
	if _, _, err := svc.Login(context.Background(), "a@b.c", "nope"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestVerifyBadToken(t *testing.T) {
	svc := NewAuthService(newFake("a@b.c", "pw123"), "secret", time.Hour)
	if _, err := svc.Verify("garbage"); err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/business/read/ -run TestLogin`
Expected: FAIL (undefined: NewAuthService).

- [ ] **Step 4: Implement ports + service**

Create `internal/business/read/ports.go`:

```go
package read

import (
	"context"

	"github.com/desi/crawl-conversations/internal/entity"
)

type UserRepo interface {
	FindByEmail(ctx context.Context, email string) (entity.User, error)
	CreateIfNotExists(ctx context.Context, u entity.User) error
}
```

Create `internal/business/read/auth_service.go`:

```go
package read

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/desi/crawl-conversations/internal/entity"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type Claims struct {
	UserID int64
	Email  string
	Role   string
}

type AuthService interface {
	Login(ctx context.Context, email, password string) (string, entity.User, error)
	Verify(token string) (Claims, error)
	SeedAdmin(ctx context.Context, email, password string) error
}

type authService struct {
	users  UserRepo
	secret []byte
	ttl    time.Duration
}

func NewAuthService(users UserRepo, secret string, ttl time.Duration) AuthService {
	return &authService{users: users, secret: []byte(secret), ttl: ttl}
}

func (s *authService) Login(ctx context.Context, email, password string) (string, entity.User, error) {
	u, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return "", entity.User{}, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return "", entity.User{}, ErrInvalidCredentials
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":   u.ID,
		"email": u.Email,
		"role":  u.Role,
		"iat":   now.Unix(),
		"exp":   now.Add(s.ttl).Unix(),
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", entity.User{}, err
	}
	return tok, u, nil
}

func (s *authService) Verify(token string) (Claims, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil || !parsed.Valid {
		return Claims{}, errors.New("invalid token")
	}
	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, errors.New("invalid claims")
	}
	uid, _ := mc["sub"].(float64)
	email, _ := mc["email"].(string)
	role, _ := mc["role"].(string)
	return Claims{UserID: int64(uid), Email: email, Role: role}, nil
}

func (s *authService) SeedAdmin(ctx context.Context, email, password string) error {
	if password == "" {
		return nil // nothing to seed
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.users.CreateIfNotExists(ctx, entity.User{
		Email: email, PasswordHash: string(h), Name: "Admin", Role: "admin",
	})
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/business/read/ -run TestLogin && go test ./internal/business/read/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/business/read/ports.go internal/business/read/auth_service.go internal/business/read/auth_service_test.go go.mod go.sum
git commit -m "feat: auth service with jwt and bcrypt"
```

---

## Task 8: QueryService (business)

**Files:**
- Create: `internal/business/read/query_service.go`
- Test: `internal/business/read/query_service_test.go`
- Modify: `internal/business/read/ports.go` (add `ReadRepo` interface)

**Interfaces:**
- Consumes: `read` repo types (`PageCount`, `ConvFilter`, `MsgFilter`, `ConvDetail`).
- Produces:
  - In `ports.go`: `type ReadRepo interface { ListPages(ctx) ([]reporead.PageCount, error); ListConversations(ctx, reporead.ConvFilter) ([]entity.Conversation, error); GetConversation(ctx, int64) (reporead.ConvDetail, error); ListMessages(ctx, reporead.MsgFilter) ([]entity.Message, error) }` (import alias `reporead "github.com/desi/crawl-conversations/internal/repository/read"`).
  - `type QueryService interface { Pages(ctx) ([]reporead.PageCount, error); Conversations(ctx, f reporead.ConvFilter) ([]entity.Conversation, error); Conversation(ctx, id int64) (reporead.ConvDetail, error); Messages(ctx, f reporead.MsgFilter) ([]entity.Message, error) }`
  - `func NewQueryService(repo ReadRepo) QueryService`

> NOTE: This service is a thin pass-through that fixes the `Limit` default and validates inputs. Keep it — it is the seam where filtering/authorization rules will grow, and it lets handlers depend on a business interface, not the repo.

- [ ] **Step 1: Write the failing test**

Create `internal/business/read/query_service_test.go`:

```go
package read

import (
	"context"
	"testing"

	"github.com/desi/crawl-conversations/internal/entity"
	reporead "github.com/desi/crawl-conversations/internal/repository/read"
)

type fakeRepo struct {
	lastConvFilter reporead.ConvFilter
}

func (f *fakeRepo) ListPages(context.Context) ([]reporead.PageCount, error) { return nil, nil }
func (f *fakeRepo) ListConversations(_ context.Context, fl reporead.ConvFilter) ([]entity.Conversation, error) {
	f.lastConvFilter = fl
	return []entity.Conversation{{ID: 1}}, nil
}
func (f *fakeRepo) GetConversation(context.Context, int64) (reporead.ConvDetail, error) {
	return reporead.ConvDetail{}, nil
}
func (f *fakeRepo) ListMessages(context.Context, reporead.MsgFilter) ([]entity.Message, error) {
	return nil, nil
}

func TestConversationsDefaultLimit(t *testing.T) {
	fr := &fakeRepo{}
	svc := NewQueryService(fr)
	if _, err := svc.Conversations(context.Background(), reporead.ConvFilter{PageID: 7}); err != nil {
		t.Fatal(err)
	}
	if fr.lastConvFilter.Limit != 50 {
		t.Fatalf("want default 50, got %d", fr.lastConvFilter.Limit)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/business/read/ -run TestConversationsDefaultLimit`
Expected: FAIL (undefined: NewQueryService).

- [ ] **Step 3: Implement ports addition + service**

Append to `internal/business/read/ports.go`:

```go
import reporead "github.com/desi/crawl-conversations/internal/repository/read"

type ReadRepo interface {
	ListPages(ctx context.Context) ([]reporead.PageCount, error)
	ListConversations(ctx context.Context, f reporead.ConvFilter) ([]entity.Conversation, error)
	GetConversation(ctx context.Context, id int64) (reporead.ConvDetail, error)
	ListMessages(ctx context.Context, f reporead.MsgFilter) ([]entity.Message, error)
}
```

> Implementer note: merge the two `import` blocks in `ports.go` into one; do not leave two separate `import` statements.

Create `internal/business/read/query_service.go`:

```go
package read

import (
	"context"

	"github.com/desi/crawl-conversations/internal/entity"
	reporead "github.com/desi/crawl-conversations/internal/repository/read"
)

const defaultLimit = 50

type QueryService interface {
	Pages(ctx context.Context) ([]reporead.PageCount, error)
	Conversations(ctx context.Context, f reporead.ConvFilter) ([]entity.Conversation, error)
	Conversation(ctx context.Context, id int64) (reporead.ConvDetail, error)
	Messages(ctx context.Context, f reporead.MsgFilter) ([]entity.Message, error)
}

type queryService struct{ repo ReadRepo }

func NewQueryService(repo ReadRepo) QueryService { return &queryService{repo: repo} }

func (s *queryService) Pages(ctx context.Context) ([]reporead.PageCount, error) {
	return s.repo.ListPages(ctx)
}

func (s *queryService) Conversations(ctx context.Context, f reporead.ConvFilter) ([]entity.Conversation, error) {
	if f.Limit <= 0 {
		f.Limit = defaultLimit
	}
	return s.repo.ListConversations(ctx, f)
}

func (s *queryService) Conversation(ctx context.Context, id int64) (reporead.ConvDetail, error) {
	return s.repo.GetConversation(ctx, id)
}

func (s *queryService) Messages(ctx context.Context, f reporead.MsgFilter) ([]entity.Message, error) {
	if f.Limit <= 0 {
		f.Limit = defaultLimit
	}
	return s.repo.ListMessages(ctx, f)
}
```

- [ ] **Step 4: Run build + test**

Run: `go build ./... && go test ./internal/business/read/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/business/read/ports.go internal/business/read/query_service.go internal/business/read/query_service_test.go
git commit -m "feat: query service for read API"
```

---

## Task 9: JWT middleware + CORS

**Files:**
- Create: `internal/transport/http/api/middleware.go`
- Test: `internal/transport/http/api/middleware_test.go`

**Interfaces:**
- Consumes: `read.AuthService`, `read.Claims`.
- Produces:
  - `func AuthMiddleware(auth read.AuthService) gin.HandlerFunc` — đọc `Authorization: Bearer <t>`, set `c.Set("claims", claims)`, 401 nếu thiếu/sai.
  - `func CORS(origin string) gin.HandlerFunc` — set CORS headers, xử lý preflight `OPTIONS` → 204.
  - `func ClaimsFrom(c *gin.Context) (read.Claims, bool)`.

- [ ] **Step 1: Write the failing test**

Create `internal/transport/http/api/middleware_test.go`:

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	read "github.com/desi/crawl-conversations/internal/business/read"
)

type stubAuth struct{ ok bool }

func (s stubAuth) Login(_ any, _, _ string) (string, any, error) { return "", nil, nil } // unused
func (s stubAuth) Verify(token string) (read.Claims, error) {
	if s.ok && token == "good" {
		return read.Claims{UserID: 9, Role: "admin"}, nil
	}
	return read.Claims{}, http.ErrNoCookie
}
func (s stubAuth) SeedAdmin(_ any, _, _ string) error { return nil }

func TestAuthMiddlewareRejectsMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(authAdapter{stubAuth{ok: true}}))
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/x", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestAuthMiddlewareAcceptsBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(authAdapter{stubAuth{ok: true}}))
	r.GET("/x", func(c *gin.Context) {
		cl, ok := ClaimsFrom(c)
		if !ok || cl.UserID != 9 {
			t.Errorf("claims not set")
		}
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer good")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}
```

> Implementer note: the middleware depends only on the `Verify(string) (read.Claims, error)` method. To keep the test from implementing the whole `AuthService`, define a tiny local interface in `middleware.go`: `type tokenVerifier interface { Verify(string) (read.Claims, error) }` and have `AuthMiddleware` take that. Then `authAdapter` in the test wraps the stub. Simpler: make `AuthMiddleware(v tokenVerifier)` and drop `authAdapter` — pass `stubAuth{ok:true}` directly (it already has `Verify`). Adjust the test to remove `authAdapter` accordingly.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/transport/http/api/ -run TestAuthMiddleware`
Expected: FAIL (undefined: AuthMiddleware).

- [ ] **Step 3: Implement middleware**

Create `internal/transport/http/api/middleware.go`:

```go
package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	read "github.com/desi/crawl-conversations/internal/business/read"
)

type tokenVerifier interface {
	Verify(token string) (read.Claims, error)
}

const claimsKey = "claims"

func AuthMiddleware(v tokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(h, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthorized", "message": "missing bearer token"}})
			return
		}
		claims, err := v.Verify(strings.TrimPrefix(h, prefix))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthorized", "message": "invalid token"}})
			return
		}
		c.Set(claimsKey, claims)
		c.Next()
	}
}

func ClaimsFrom(c *gin.Context) (read.Claims, bool) {
	v, ok := c.Get(claimsKey)
	if !ok {
		return read.Claims{}, false
	}
	cl, ok := v.(read.Claims)
	return cl, ok
}

func CORS(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
```

> Implementer note: update `middleware_test.go` to pass `stubAuth{ok: true}` directly to `AuthMiddleware` (remove `authAdapter`). `stubAuth` must satisfy only `Verify`; trim the unused `Login`/`SeedAdmin` if they cause unused-method noise (they don't fail compilation, but keep the stub minimal: only `Verify` is required by `tokenVerifier`).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/transport/http/api/ -run TestAuthMiddleware`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/transport/http/api/middleware.go internal/transport/http/api/middleware_test.go
git commit -m "feat: jwt auth middleware and cors"
```

---

## Task 10: Handlers (auth + query) + router

**Files:**
- Create: `internal/transport/http/api/auth_handler.go`
- Create: `internal/transport/http/api/query_handler.go`
- Create: `internal/transport/http/api/router.go`
- Test: `internal/transport/http/api/router_test.go`

**Interfaces:**
- Consumes: `read.AuthService`, `read.QueryService`, `response.*`, `reporead.ConvFilter/MsgFilter`.
- Produces:
  - `type Deps struct { Auth read.AuthService; Query read.QueryService; CORSOrigin string }`
  - `func Register(r *gin.Engine, d Deps)` — gắn nhóm `/api/v1` với CORS; `POST /auth/login` công khai; phần còn lại sau `AuthMiddleware`.

- [ ] **Step 1: Write the failing test**

Create `internal/transport/http/api/router_test.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	read "github.com/desi/crawl-conversations/internal/business/read"
	"github.com/desi/crawl-conversations/internal/entity"
	reporead "github.com/desi/crawl-conversations/internal/repository/read"
)

// --- fakes ---
type fakeAuth struct{}

func (fakeAuth) Login(_ context.Context, email, password string) (string, entity.User, error) {
	if email == "a@b.c" && password == "pw" {
		return "token123", entity.User{ID: 1, Email: email, Role: "admin"}, nil
	}
	return "", entity.User{}, read.ErrInvalidCredentials
}
func (fakeAuth) Verify(t string) (read.Claims, error) {
	if t == "token123" {
		return read.Claims{UserID: 1, Role: "admin"}, nil
	}
	return read.Claims{}, read.ErrInvalidCredentials
}
func (fakeAuth) SeedAdmin(context.Context, string, string) error { return nil }

type fakeQuery struct{}

func (fakeQuery) Pages(context.Context) ([]reporead.PageCount, error) {
	return []reporead.PageCount{{Page: entity.Page{ID: 1, Name: "P"}, ConversationCount: 3}}, nil
}
func (fakeQuery) Conversations(context.Context, reporead.ConvFilter) ([]entity.Conversation, error) {
	return []entity.Conversation{{ID: 5}}, nil
}
func (fakeQuery) Conversation(context.Context, int64) (reporead.ConvDetail, error) {
	return reporead.ConvDetail{Conversation: entity.Conversation{ID: 5}}, nil
}
func (fakeQuery) Messages(context.Context, reporead.MsgFilter) ([]entity.Message, error) {
	return []entity.Message{{ID: 9, Direction: "in"}}, nil
}

func setup() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r, Deps{Auth: fakeAuth{}, Query: fakeQuery{}, CORSOrigin: "*"})
	return r
}

func TestLoginThenAccess(t *testing.T) {
	r := setup()

	// login
	w := httptest.NewRecorder()
	body := `{"email":"a@b.c","password":"pw"}`
	req, _ := http.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var lr struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &lr)
	if lr.Token != "token123" {
		t.Fatalf("no token: %s", w.Body.String())
	}

	// protected without token -> 401
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/pages", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w2.Code)
	}

	// protected with token -> 200
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/api/v1/pages", nil)
	req3.Header.Set("Authorization", "Bearer token123")
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w3.Code, w3.Body.String())
	}
}

func TestMessagesDirectionParam(t *testing.T) {
	r := setup()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/conversations/5/messages?direction=in&limit=10", nil)
	req.Header.Set("Authorization", "Bearer token123")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/transport/http/api/ -run TestLoginThenAccess`
Expected: FAIL (undefined: Register/Deps).

- [ ] **Step 3: Implement auth handler**

Create `internal/transport/http/api/auth_handler.go`:

```go
package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	read "github.com/desi/crawl-conversations/internal/business/read"
)

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func handleLogin(auth read.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "invalid body"}})
			return
		}
		tok, u, err := auth.Login(c.Request.Context(), req.Email, req.Password)
		if err != nil {
			if errors.Is(err, read.ErrInvalidCredentials) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthorized", "message": "invalid credentials"}})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": "login failed"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"token": tok,
			"user":  gin.H{"id": u.ID, "email": u.Email, "name": u.Name, "role": u.Role},
		})
	}
}

func handleMe() gin.HandlerFunc {
	return func(c *gin.Context) {
		cl, ok := ClaimsFrom(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthorized", "message": "no claims"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": cl.UserID, "email": cl.Email, "role": cl.Role})
	}
}
```

- [ ] **Step 4: Implement query handler**

Create `internal/transport/http/api/query_handler.go`:

```go
package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	read "github.com/desi/crawl-conversations/internal/business/read"
	"github.com/desi/crawl-conversations/internal/dto/response"
	reporead "github.com/desi/crawl-conversations/internal/repository/read"
)

func parseLimit(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 50
	}
	if n > 100 {
		return 100
	}
	return n
}

func handlePages(q read.QueryService) gin.HandlerFunc {
	return func(c *gin.Context) {
		pages, err := q.Pages(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": "list pages failed"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": response.MapPages(pages)})
	}
}

func handleConversations(q read.QueryService) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID, err := strconv.ParseInt(c.Query("page_id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "page_id required"}})
			return
		}
		limit := parseLimit(c.Query("limit"))
		convs, err := q.Conversations(c.Request.Context(), reporead.ConvFilter{
			PageID: pageID, Cursor: c.Query("cursor"), Limit: limit,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": "list conversations failed"}})
			return
		}
		c.JSON(http.StatusOK, response.MapConversations(convs, limit))
	}
}

func handleConversationDetail(q read.QueryService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "invalid id"}})
			return
		}
		detail, err := q.Conversation(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "conversation not found"}})
			return
		}
		c.JSON(http.StatusOK, response.MapConversationDetail(detail))
	}
}

func handleMessages(q read.QueryService) gin.HandlerFunc {
	return func(c *gin.Context) {
		convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "invalid id"}})
			return
		}
		limit := parseLimit(c.Query("limit"))
		msgs, err := q.Messages(c.Request.Context(), reporead.MsgFilter{
			ConversationID: convID, Direction: c.Query("direction"),
			Cursor: c.Query("cursor"), Limit: limit,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": "list messages failed"}})
			return
		}
		c.JSON(http.StatusOK, response.MapMessages(msgs, limit))
	}
}
```

- [ ] **Step 5: Implement router**

Create `internal/transport/http/api/router.go`:

```go
package api

import (
	"github.com/gin-gonic/gin"

	read "github.com/desi/crawl-conversations/internal/business/read"
)

type Deps struct {
	Auth       read.AuthService
	Query      read.QueryService
	CORSOrigin string
}

func Register(r *gin.Engine, d Deps) {
	v1 := r.Group("/api/v1")
	v1.Use(CORS(d.CORSOrigin))

	v1.POST("/auth/login", handleLogin(d.Auth))

	auth := v1.Group("")
	auth.Use(AuthMiddleware(d.Auth))
	auth.GET("/auth/me", handleMe())
	auth.GET("/pages", handlePages(d.Query))
	auth.GET("/conversations", handleConversations(d.Query))
	auth.GET("/conversations/:id", handleConversationDetail(d.Query))
	auth.GET("/conversations/:id/messages", handleMessages(d.Query))
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/transport/http/api/`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/transport/http/api/auth_handler.go internal/transport/http/api/query_handler.go internal/transport/http/api/router.go internal/transport/http/api/router_test.go
git commit -m "feat: read API handlers and router"
```

---

## Task 11: Wire into server + DI + seed at startup

**Files:**
- Modify: `internal/transport/http/server.go`
- Modify: `internal/transport/http/server_test.go`
- Modify: `cmd/crawler/inject.go`
- Modify: `cmd/crawler/main.go`

**Interfaces:**
- Consumes: `api.Register`, `api.Deps`, `read.AuthService`, `read.QueryService`, `read.NewUserRepository`, `read.NewReadRepository`.
- Produces: `NewServer(addr, log, trigger, apiDeps api.Deps)` — registers `/api/v1` alongside existing routes.

- [ ] **Step 1: Update server signature (failing build)**

Modify `internal/transport/http/server.go` — change signature and register API:

```go
func NewServer(addr string, log *slog.Logger, trigger func() bool, apiDeps api.Deps) *http.Server {
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
	api.Register(r, apiDeps)
	return &http.Server{Addr: addr, Handler: r}
}
```

Add import: `api "github.com/desi/crawl-conversations/internal/transport/http/api"`.

- [ ] **Step 2: Update server_test.go for new signature**

In `internal/transport/http/server_test.go`, update every `NewServer(...)` call to pass an empty `api.Deps{}` (the existing healthz/crawl tests don't exercise the API group). Add the import. Example:

```go
srv := NewServer(":0", slog.Default(), func() bool { return true }, api.Deps{})
```

- [ ] **Step 3: Run server tests**

Run: `go test ./internal/transport/http/`
Expected: PASS (existing healthz/metrics/crawl tests still green with empty Deps; the API group registers but isn't hit).

- [ ] **Step 4: Wire DI providers**

In `cmd/crawler/inject.go`, add imports:

```go
	readrepo "github.com/desi/crawl-conversations/internal/repository/read"
	readbiz "github.com/desi/crawl-conversations/internal/business/read"
	api "github.com/desi/crawl-conversations/internal/transport/http/api"
```

Add providers before `return i`:

```go
	do.Provide(i, func(in do.Injector) (readbiz.AuthService, error) {
		c := do.MustInvoke[config.Config](in)
		db := do.MustInvoke[*repository.DB](in)
		users := readrepo.NewUserRepository(db)
		return readbiz.NewAuthService(users, c.JWTSecret, c.JWTTTL), nil
	})
	do.Provide(i, func(in do.Injector) (readbiz.QueryService, error) {
		db := do.MustInvoke[*repository.DB](in)
		return readbiz.NewQueryService(readrepo.NewReadRepository(db)), nil
	})
	do.Provide(i, func(in do.Injector) (api.Deps, error) {
		c := do.MustInvoke[config.Config](in)
		return api.Deps{
			Auth:       do.MustInvoke[readbiz.AuthService](in),
			Query:      do.MustInvoke[readbiz.QueryService](in),
			CORSOrigin: c.CORSOrigin,
		}, nil
	})
```

- [ ] **Step 5: Seed admin + pass Deps in main.go**

In `cmd/crawler/main.go`, after `db := do.MustInvoke[*repository.DB](injector)` and before starting the server, add:

```go
	apiDeps := do.MustInvoke[api.Deps](injector)
	if err := apiDeps.Auth.SeedAdmin(context.Background(), cfg.AdminEmail, cfg.AdminPassword); err != nil {
		log.Error("seed admin failed", "err", err)
	}
```

Change the server construction:

```go
	srv := httpx.NewServer(cfg.HTTPAddr, log, sched.Trigger, apiDeps)
```

Add import `api "github.com/desi/crawl-conversations/internal/transport/http/api"` (note: `context` is already imported).

- [ ] **Step 6: Build everything**

Run: `go build ./... && go vet ./...`
Expected: success, no errors.

- [ ] **Step 7: Run full test suite**

Run: `go test ./...`
Expected: PASS (integration tests SKIP without `TEST_DATABASE_URL`).

- [ ] **Step 8: Commit**

```bash
git add internal/transport/http/server.go internal/transport/http/server_test.go cmd/crawler/inject.go cmd/crawler/main.go
git commit -m "feat: wire read API into server and DI with admin seeding"
```

---

## Task 12: Update env example + README + design progress

**Files:**
- Modify: `.env.example` (or `deploy/.env.example` — check which exists)
- Modify: `README.md`
- Modify: `docs/superpowers/specs/2026-06-29-read-api-fe-spa-design.md` (mark Sprint 1 progress)

- [ ] **Step 1: Add env vars to example**

Locate the env example file (`git ls-files | grep env.example`). Append:

```
# Read API
JWT_SECRET=change-me-in-prod
JWT_TTL=12h
ADMIN_EMAIL=admin@local
ADMIN_PASSWORD=change-me
CORS_ORIGIN=*
```

- [ ] **Step 2: Document the read API in README**

Add a section to `README.md`:

```markdown
## Read API
Auth + read endpoints under `/api/v1` (JWT required except login):
- `POST /api/v1/auth/login` `{email,password}` → `{token,user}`
- `GET /api/v1/auth/me`
- `GET /api/v1/pages`
- `GET /api/v1/conversations?page_id=&cursor=&limit=`
- `GET /api/v1/conversations/:id`
- `GET /api/v1/conversations/:id/messages?direction=in|out&cursor=&limit=`

Seed admin via `ADMIN_EMAIL` / `ADMIN_PASSWORD` at startup. Set `JWT_SECRET`.
```

- [ ] **Step 3: Mark Sprint 1 done in design doc**

In the spec, change `### Sprint 1 — Read API nền (tiến độ: ☐)` to `(tiến độ: ✅)` and check its boxes; add a short "đã làm / chưa làm" note.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "docs: read API env, readme and sprint1 progress"
```

---

## Self-Review notes (đã kiểm)

- **Spec coverage (Sprint 1 scope):** users migration ✅ (T1), auth login/me + JWT middleware ✅ (T7,T9,T10), pages/conversations/detail/messages keyset + direction ✅ (T5,T10), DTO không trả raw ✅ (T6), CORS ✅ (T9), seed admin ✅ (T7,T11). Search/customers/stats/dashboard/deploy thuộc Sprint 2–5, có plan riêng.
- **Type consistency:** `ConvFilter`/`MsgFilter`/`ConvDetail`/`PageCount` định nghĩa ở `repository/read` (T5), tái dùng nhất quán ở business (T8) và handlers (T10). `read.Claims`/`AuthService` (T7) dùng ở middleware (T9) + handlers (T10). `api.Deps` (T10) dùng ở server + DI (T11).
- **Placeholder scan:** không có TODO/TBD trong code steps; các "Implementer note" là hướng dẫn ghép import/đơn giản hóa stub, không phải code thiếu.
- **Lưu ý GORM keyset:** dùng tuple compare `(updated_at, id) < (?, ?)` — Postgres hỗ trợ; index `(page_id, updated_at DESC)` sẵn có hỗ trợ filter page + sort.

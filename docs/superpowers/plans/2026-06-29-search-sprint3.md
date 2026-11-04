# Sprint 3 — Search (BE FTS + customers + FE) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tìm kiếm full-text nội dung tin nhắn (Postgres FTS) và tra cứu khách hàng, có filter page/thời gian/direction; FE màn Search hiển thị kết quả và nhảy vào hội thoại.

**Architecture:** Mở rộng module read Sprint 1: migration `tsvector` + GIN trên `messages`; thêm `SearchRepository`/`SearchService` và endpoints `/search/messages`, `/customers`. FE thêm feature `search`.

**Tech Stack:** Go 1.25, GORM, goose, Gin; FE React + TanStack Query.

## Global Constraints
- GORM cho read queries (FTS qua `Where` raw expression + GORM).
- Keyset pagination dạng `{items, next_cursor}` như Sprint 1.
- DTO không trả `raw`.
- Mọi endpoint cần JWT.
- Commit message tiếng Anh, ngắn gọn.

---

## File Structure
**Create:**
- `db/migrations/00004_messages_fts.sql` — cột `content_tsv` + GIN index + trigger.
- `internal/repository/read/search_repo.go` — `SearchMessages`, `SearchCustomers`.
- `internal/business/read/search_service.go` — `SearchService`.
- `internal/transport/http/api/search_handler.go` — handlers.
- `web/src/services/search.ts`, `web/src/features/search/use-search.ts`, `use-customers.ts`, `search-page.tsx`
- Tests tương ứng.

**Modify:**
- `internal/business/read/ports.go` — thêm `SearchRepo` interface.
- `internal/transport/http/api/router.go` — thêm routes + nhận `Search` trong `Deps`.
- `cmd/crawler/inject.go` — provide search service.
- `web/src/app/router.tsx` + `root-layout.tsx` — thêm route `/search` + nav link.

---

## Task 1: Migration FTS (tsvector + GIN + trigger)

**Files:**
- Create: `db/migrations/00004_messages_fts.sql`

> NOTE: `messages` partition theo tháng. Thêm cột generated trên bảng cha sẽ áp cho mọi partition. Dùng cột thường + trigger để tránh ràng buộc IMMUTABLE của generated column với cấu hình ngôn ngữ; index GIN tạo trên bảng cha.

- [ ] **Step 1: Write migration**

Create `db/migrations/00004_messages_fts.sql`:
```sql
-- +goose Up
-- +goose StatementBegin
ALTER TABLE messages ADD COLUMN IF NOT EXISTS content_tsv tsvector;

CREATE OR REPLACE FUNCTION messages_tsv_update() RETURNS trigger AS $$
BEGIN
  NEW.content_tsv := to_tsvector('simple', coalesce(NEW.content, ''));
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trg_messages_tsv ON messages;
CREATE TRIGGER trg_messages_tsv BEFORE INSERT OR UPDATE OF content ON messages
  FOR EACH ROW EXECUTE FUNCTION messages_tsv_update();

UPDATE messages SET content_tsv = to_tsvector('simple', coalesce(content, ''))
  WHERE content_tsv IS NULL;

CREATE INDEX IF NOT EXISTS idx_messages_content_tsv ON messages USING GIN (content_tsv);

-- +goose Down
DROP INDEX IF EXISTS idx_messages_content_tsv;
DROP TRIGGER IF EXISTS trg_messages_tsv ON messages;
DROP FUNCTION IF EXISTS messages_tsv_update();
ALTER TABLE messages DROP COLUMN IF EXISTS content_tsv;
```

> Implementer note: dùng `'simple'` config để không phụ thuộc ngôn ngữ (tiếng Việt không có stemmer mặc định); `plainto_tsquery('simple', ...)` ở query phía repo. Nếu `messages` rất lớn, cân nhắc `CREATE INDEX CONCURRENTLY` ngoài migration — ở đây giữ trong migration cho đơn giản.

- [ ] **Step 2: Verify migration applies**

Run (nếu có `TEST_DATABASE_URL` hoặc DB dev): `make migrate-up` (hoặc khởi động app, migrations chạy lúc boot).
Expected: không lỗi; index `idx_messages_content_tsv` tồn tại.

- [ ] **Step 3: Commit**
```bash
git add db/migrations/00004_messages_fts.sql
git commit -m "feat: full text search index on messages content"
```

---

## Task 2: SearchRepository (GORM + FTS)

**Files:**
- Create: `internal/repository/read/search_repo.go`
- Test: `internal/repository/read/search_repo_test.go` (integration)

**Interfaces:**
- Produces:
  - `type MsgSearchFilter struct { Query string; PageID int64; Direction string; From, To *time.Time; Cursor string; Limit int }`
  - `type MsgSearchHit struct { entity.Message; ConversationID int64 }` (Message đã có ConversationID → dùng thẳng `entity.Message`).
  - `type CustomerFilter struct { PageID int64; Query string; Cursor string; Limit int }`
  - `type SearchRepository interface { SearchMessages(ctx, f MsgSearchFilter) ([]entity.Message, error); SearchCustomers(ctx, f CustomerFilter) ([]entity.Customer, error) }`
  - `func NewSearchRepository(db *repository.DB) SearchRepository`
- Semantics: `SearchMessages` lọc `content_tsv @@ plainto_tsquery('simple', ?)`, optional page/direction/time-range; sort `inserted_at DESC, id DESC`; keyset như Sprint 1. `SearchCustomers` lọc `name/phone/email ILIKE`, keyset theo `id DESC`.

- [ ] **Step 1: Write the failing test**

Create `internal/repository/read/search_repo_test.go`:
```go
package read

import (
	"context"
	"testing"
	"time"
)

func TestSearchMessages(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	pageID, err := seedPage(ctx, db, "p-search")
	if err != nil {
		t.Fatal(err)
	}
	convID, err := seedConversationReturnID(ctx, db, pageID, "c-search", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := seedMessage(ctx, db, pageID, convID, "m1", "in", "hello pancake world", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	repo := NewSearchRepository(db)
	hits, err := repo.SearchMessages(ctx, MsgSearchFilter{Query: "pancake", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one hit")
	}
}
```

> Implementer note: tái dùng `testDB` (Sprint 1 Task 4). Thêm helper `seedConversationReturnID`, `seedMessage` trong file test này (insert qua `db.Gorm.Create`). `seedMessage` cần `InsertedAt` để partition routing hoạt động.

- [ ] **Step 2: Run test (fail)**
Run: `go test ./internal/repository/read/ -run TestSearchMessages`
Expected: FAIL (undefined NewSearchRepository) hoặc SKIP nếu không DB; build phải lỗi trước.

- [ ] **Step 3: Implement repo**

Create `internal/repository/read/search_repo.go`:
```go
package read

import (
	"context"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/desi/crawl-conversations/internal/repository"
)

type MsgSearchFilter struct {
	Query     string
	PageID    int64
	Direction string
	From, To  *time.Time
	Cursor    string
	Limit     int
}

type CustomerFilter struct {
	PageID int64
	Query  string
	Cursor string
	Limit  int
}

type SearchRepository interface {
	SearchMessages(ctx context.Context, f MsgSearchFilter) ([]entity.Message, error)
	SearchCustomers(ctx context.Context, f CustomerFilter) ([]entity.Customer, error)
}

type searchRepo struct{ db *repository.DB }

func NewSearchRepository(db *repository.DB) SearchRepository { return &searchRepo{db} }

func (r *searchRepo) SearchMessages(ctx context.Context, f MsgSearchFilter) ([]entity.Message, error) {
	q := r.db.Gorm.WithContext(ctx).
		Model(&entity.Message{}).
		Where("content_tsv @@ plainto_tsquery('simple', ?)", f.Query).
		Order("inserted_at DESC, id DESC").
		Limit(clampLimit(f.Limit))
	if f.PageID > 0 {
		q = q.Where("page_id = ?", f.PageID)
	}
	if f.Direction == "in" || f.Direction == "out" {
		q = q.Where("direction = ?", f.Direction)
	}
	if f.From != nil {
		q = q.Where("inserted_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("inserted_at <= ?", *f.To)
	}
	if c, ok := DecodeCursor(f.Cursor); ok {
		q = q.Where("(inserted_at, id) < (?, ?)", c.Time, c.ID)
	}
	var out []entity.Message
	err := q.Find(&out).Error
	return out, err
}

func (r *searchRepo) SearchCustomers(ctx context.Context, f CustomerFilter) ([]entity.Customer, error) {
	q := r.db.Gorm.WithContext(ctx).
		Model(&entity.Customer{}).
		Order("id DESC").
		Limit(clampLimit(f.Limit))
	if f.PageID > 0 {
		q = q.Where("page_id = ?", f.PageID)
	}
	if f.Query != "" {
		like := "%" + f.Query + "%"
		q = q.Where("name ILIKE ? OR phone ILIKE ? OR email ILIKE ?", like, like, like)
	}
	if c, ok := DecodeCursor(f.Cursor); ok {
		q = q.Where("id < ?", c.ID)
	}
	var out []entity.Customer
	err := q.Find(&out).Error
	return out, err
}
```

- [ ] **Step 4: Build + test**
Run: `go build ./... && go test ./internal/repository/read/`
Expected: PASS (integration SKIP nếu không DB); build OK.

- [ ] **Step 5: Commit**
```bash
git add internal/repository/read/search_repo.go internal/repository/read/search_repo_test.go
git commit -m "feat: search repository for messages and customers"
```

---

## Task 3: SearchService + ports + DTO mapping

**Files:**
- Modify: `internal/business/read/ports.go` (add `SearchRepo`)
- Create: `internal/business/read/search_service.go`
- Modify: `internal/dto/response/response.go` (add `MapCustomers`, reuse `MapMessages`)
- Test: `internal/business/read/search_service_test.go`, `internal/dto/response/response_test.go` (add case)

**Interfaces:**
- `ports.go`: `type SearchRepo interface { SearchMessages(ctx, reporead.MsgSearchFilter) ([]entity.Message, error); SearchCustomers(ctx, reporead.CustomerFilter) ([]entity.Customer, error) }`
- `search_service.go`: `type SearchService interface { Messages(ctx, reporead.MsgSearchFilter) ([]entity.Message, error); Customers(ctx, reporead.CustomerFilter) ([]entity.Customer, error) }`; `func NewSearchService(repo SearchRepo) SearchService`. Default limit 50.
- `response.go`: `func MapCustomers(in []entity.Customer, limit int) Page[CustomerResp]` (keyset theo `id`; NextCursor encode `Cursor{Time: zero, ID: last.ID}`).

- [ ] **Step 1: Write failing tests**

Create `internal/business/read/search_service_test.go`:
```go
package read

import (
	"context"
	"testing"

	"github.com/desi/crawl-conversations/internal/entity"
	reporead "github.com/desi/crawl-conversations/internal/repository/read"
)

type fakeSearch struct{ last reporead.MsgSearchFilter }

func (f *fakeSearch) SearchMessages(_ context.Context, fl reporead.MsgSearchFilter) ([]entity.Message, error) {
	f.last = fl
	return []entity.Message{{ID: 1}}, nil
}
func (f *fakeSearch) SearchCustomers(context.Context, reporead.CustomerFilter) ([]entity.Customer, error) {
	return nil, nil
}

func TestSearchMessagesDefaultLimit(t *testing.T) {
	fs := &fakeSearch{}
	svc := NewSearchService(fs)
	if _, err := svc.Messages(context.Background(), reporead.MsgSearchFilter{Query: "x"}); err != nil {
		t.Fatal(err)
	}
	if fs.last.Limit != 50 {
		t.Fatalf("want 50, got %d", fs.last.Limit)
	}
}
```

Add to `internal/dto/response/response_test.go`:
```go
func TestMapCustomersCursor(t *testing.T) {
	in := []entity.Customer{{ID: 3}, {ID: 2}}
	p := MapCustomers(in, 2)
	if p.NextCursor == nil {
		t.Fatal("want cursor")
	}
	if len(p.Items) != 2 {
		t.Fatalf("want 2 items, got %d", len(p.Items))
	}
}
```

- [ ] **Step 2: Run tests (fail)**
Run: `go test ./internal/business/read/ -run TestSearchMessagesDefaultLimit && go test ./internal/dto/response/ -run TestMapCustomersCursor`
Expected: FAIL.

- [ ] **Step 3: Implement ports + service + mapper**

Append to `internal/business/read/ports.go`:
```go
type SearchRepo interface {
	SearchMessages(ctx context.Context, f reporead.MsgSearchFilter) ([]entity.Message, error)
	SearchCustomers(ctx context.Context, f reporead.CustomerFilter) ([]entity.Customer, error)
}
```

Create `internal/business/read/search_service.go`:
```go
package read

import (
	"context"

	"github.com/desi/crawl-conversations/internal/entity"
	reporead "github.com/desi/crawl-conversations/internal/repository/read"
)

type SearchService interface {
	Messages(ctx context.Context, f reporead.MsgSearchFilter) ([]entity.Message, error)
	Customers(ctx context.Context, f reporead.CustomerFilter) ([]entity.Customer, error)
}

type searchService struct{ repo SearchRepo }

func NewSearchService(repo SearchRepo) SearchService { return &searchService{repo: repo} }

func (s *searchService) Messages(ctx context.Context, f reporead.MsgSearchFilter) ([]entity.Message, error) {
	if f.Limit <= 0 {
		f.Limit = defaultLimit
	}
	return s.repo.SearchMessages(ctx, f)
}

func (s *searchService) Customers(ctx context.Context, f reporead.CustomerFilter) ([]entity.Customer, error) {
	if f.Limit <= 0 {
		f.Limit = defaultLimit
	}
	return s.repo.SearchCustomers(ctx, f)
}
```

Append to `internal/dto/response/response.go`:
```go
func MapCustomers(in []entity.Customer, limit int) Page[CustomerResp] {
	items := make([]CustomerResp, 0, len(in))
	for _, c := range in {
		items = append(items, CustomerResp{
			ID: c.ID, Name: c.Name, Phone: c.Phone, Email: c.Email, FbID: c.FbID, Gender: c.Gender,
		})
	}
	var next *string
	if len(in) >= limit && len(in) > 0 {
		last := in[len(in)-1]
		s := read.EncodeCursor(read.Cursor{ID: last.ID})
		next = &s
	}
	return Page[CustomerResp]{Items: items, NextCursor: next}
}
```

- [ ] **Step 4: Run tests (pass)**
Run: `go test ./internal/business/read/ ./internal/dto/response/`
Expected: PASS

- [ ] **Step 5: Commit**
```bash
git add internal/business/read/ports.go internal/business/read/search_service.go internal/business/read/search_service_test.go internal/dto/response/
git commit -m "feat: search service and customer dto mapping"
```

---

## Task 4: Search handlers + router + DI wiring

**Files:**
- Create: `internal/transport/http/api/search_handler.go`
- Modify: `internal/transport/http/api/router.go` (add `Search` to `Deps` + routes)
- Modify: `cmd/crawler/inject.go` (provide `SearchService`, add to `api.Deps`)
- Test: `internal/transport/http/api/search_handler_test.go`

**Interfaces:**
- `Deps` thêm field `Search read.SearchService`.
- Routes (guarded): `GET /search/messages?q=&page_id=&direction=&from=&to=&cursor=&limit=`, `GET /customers?page_id=&q=&cursor=&limit=`.
- `from`/`to` parse RFC3339; bỏ qua nếu rỗng/sai.

- [ ] **Step 1: Write failing test**

Create `internal/transport/http/api/search_handler_test.go`:
```go
package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/desi/crawl-conversations/internal/entity"
	reporead "github.com/desi/crawl-conversations/internal/repository/read"
)

type fakeSearchSvc struct{}

func (fakeSearchSvc) Messages(context.Context, reporead.MsgSearchFilter) ([]entity.Message, error) {
	return []entity.Message{{ID: 1, Content: "hi", Direction: "in"}}, nil
}
func (fakeSearchSvc) Customers(context.Context, reporead.CustomerFilter) ([]entity.Customer, error) {
	return []entity.Customer{{ID: 2, Name: "Anna"}}, nil
}

func setupSearch() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r, Deps{Auth: fakeAuth{}, Query: fakeQuery{}, Search: fakeSearchSvc{}, CORSOrigin: "*"})
	return r
}

func TestSearchMessagesEndpoint(t *testing.T) {
	r := setupSearch()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/search/messages?q=hi&direction=in", nil)
	req.Header.Set("Authorization", "Bearer token123")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	r := setupSearch()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/search/messages", nil)
	req.Header.Set("Authorization", "Bearer token123")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}
```

> Implementer note: `fakeQuery`/`fakeAuth` đã có ở `router_test.go` (Sprint 1 Task 10) cùng package — tái dùng.

- [ ] **Step 2: Run test (fail)**
Run: `go test ./internal/transport/http/api/ -run TestSearch`
Expected: FAIL (Deps has no field Search).

- [ ] **Step 3: Implement handler**

Create `internal/transport/http/api/search_handler.go`:
```go
package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	read "github.com/desi/crawl-conversations/internal/business/read"
	"github.com/desi/crawl-conversations/internal/dto/response"
	reporead "github.com/desi/crawl-conversations/internal/repository/read"
)

func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

func parseInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func handleSearchMessages(s read.SearchService) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := c.Query("q")
		if q == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "q is required"}})
			return
		}
		limit := parseLimit(c.Query("limit"))
		msgs, err := s.Messages(c.Request.Context(), reporead.MsgSearchFilter{
			Query: q, PageID: parseInt64(c.Query("page_id")), Direction: c.Query("direction"),
			From: parseTime(c.Query("from")), To: parseTime(c.Query("to")),
			Cursor: c.Query("cursor"), Limit: limit,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": "search failed"}})
			return
		}
		c.JSON(http.StatusOK, response.MapMessages(msgs, limit))
	}
}

func handleCustomers(s read.SearchService) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := parseLimit(c.Query("limit"))
		custs, err := s.Customers(c.Request.Context(), reporead.CustomerFilter{
			PageID: parseInt64(c.Query("page_id")), Query: c.Query("q"),
			Cursor: c.Query("cursor"), Limit: limit,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": "list customers failed"}})
			return
		}
		c.JSON(http.StatusOK, response.MapCustomers(custs, limit))
	}
}
```

- [ ] **Step 4: Update router + Deps**

In `internal/transport/http/api/router.go`, add to `Deps`:
```go
	Search read.SearchService
```
Add inside the `auth` group in `Register`:
```go
	auth.GET("/search/messages", handleSearchMessages(d.Search))
	auth.GET("/customers", handleCustomers(d.Search))
```

- [ ] **Step 5: Wire DI**

In `cmd/crawler/inject.go`, add provider:
```go
	do.Provide(i, func(in do.Injector) (readbiz.SearchService, error) {
		db := do.MustInvoke[*repository.DB](in)
		return readbiz.NewSearchService(readrepo.NewSearchRepository(db)), nil
	})
```
Update the `api.Deps` provider to include:
```go
		Search: do.MustInvoke[readbiz.SearchService](in),
```

- [ ] **Step 6: Build + test**
Run: `go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 7: Commit**
```bash
git add internal/transport/http/api/ cmd/crawler/inject.go
git commit -m "feat: search and customers endpoints wired"
```

---

## Task 5: FE search service + hooks

**Files:**
- Create: `web/src/services/search.ts`, `web/src/features/search/use-search.ts`, `web/src/features/search/use-customers.ts`

**Interfaces:**
- `search.ts`: `searchMessages(params)`, `searchCustomers(params)` → `Paged<Message>` / `Paged<Customer>`.
- hooks: `useSearchMessages(params)` (`useInfiniteQuery`, enabled khi `q` không rỗng), `useCustomers(params)`.

- [ ] **Step 1: Implement service**

Create `web/src/services/search.ts`:
```ts
import api from "./client";
import type { Message, Customer, Paged } from "@/types/api";

export interface MsgSearchParams {
  q: string; page_id?: number; direction?: string; from?: string; to?: string; cursor?: string; limit?: number;
}
export async function searchMessages(p: MsgSearchParams): Promise<Paged<Message>> {
  const { data } = await api.get<Paged<Message>>("/search/messages", { params: p });
  return data;
}

export interface CustomerSearchParams { page_id?: number; q?: string; cursor?: string; limit?: number; }
export async function searchCustomers(p: CustomerSearchParams): Promise<Paged<Customer>> {
  const { data } = await api.get<Paged<Customer>>("/customers", { params: p });
  return data;
}
```

- [ ] **Step 2: Implement hooks**

Create `web/src/features/search/use-search.ts`:
```ts
import { useInfiniteQuery } from "@tanstack/react-query";
import { searchMessages, type MsgSearchParams } from "@/services/search";

export function useSearchMessages(params: MsgSearchParams) {
  return useInfiniteQuery({
    queryKey: ["search-messages", params],
    enabled: !!params.q,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) => searchMessages({ ...params, cursor: pageParam, limit: 30 }),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
}
```

Create `web/src/features/search/use-customers.ts`:
```ts
import { useInfiniteQuery } from "@tanstack/react-query";
import { searchCustomers, type CustomerSearchParams } from "@/services/search";

export function useCustomers(params: CustomerSearchParams) {
  return useInfiniteQuery({
    queryKey: ["customers", params],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) => searchCustomers({ ...params, cursor: pageParam, limit: 30 }),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
}
```

- [ ] **Step 3: Build**
Run: `cd web && npm run build`
Expected: success.

- [ ] **Step 4: Commit**
```bash
git add web/ -- ':!web/node_modules'
git commit -m "feat: frontend search service and hooks"
```

---

## Task 6: Search page + route + nav

**Files:**
- Create: `web/src/features/search/search-page.tsx`
- Modify: `web/src/app/router.tsx` (add `/search` route under appRoute), `web/src/app/root-layout.tsx` (nav links)
- Test: `web/src/features/search/search-page.test.tsx`

**Interfaces:**
- `SearchPage`: ô nhập `q`, filter direction + page (optional), nút tìm; list kết quả message (content + thời gian + direction badge); click 1 kết quả → `navigate({ to: "/", search... })` hoặc hiển thị conversation id (đơn giản: link tới inbox với pageId). Ở Sprint này chỉ cần hiển thị kết quả + nút "Tải thêm".

- [ ] **Step 1: Implement search page**

Create `web/src/features/search/search-page.tsx`:
```tsx
import { useState } from "react";
import { useSearchMessages } from "./use-search";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { formatDateTime } from "@/lib/format";
import { Search } from "lucide-react";

export function SearchPage() {
  const [term, setTerm] = useState("");
  const [direction, setDirection] = useState("");
  const [query, setQuery] = useState({ q: "", direction: "" });
  const res = useSearchMessages(query);
  const items = res.data?.pages.flatMap((p) => p.items) ?? [];

  return (
    <div className="mx-auto max-w-3xl p-4">
      <form
        className="mb-4 flex gap-2"
        onSubmit={(e) => { e.preventDefault(); setQuery({ q: term, direction }); }}
      >
        <Input placeholder="Tìm nội dung tin nhắn…" value={term} onChange={(e) => setTerm(e.target.value)} />
        <select className="rounded-md border px-2 text-sm" value={direction} onChange={(e) => setDirection(e.target.value)}>
          <option value="">Tất cả</option>
          <option value="in">Khách gửi</option>
          <option value="out">Trang gửi</option>
        </select>
        <Button type="submit"><Search className="mr-1 h-4 w-4" />Tìm</Button>
      </form>

      {res.isLoading && query.q && <p className="text-sm text-muted-foreground">Đang tìm…</p>}
      {query.q && !res.isLoading && items.length === 0 && <p className="text-sm text-muted-foreground">Không có kết quả.</p>}

      <ul className="space-y-2">
        {items.map((m) => (
          <li key={m.id} className="rounded-md border p-3">
            <div className="mb-1 flex items-center justify-between">
              <Badge variant={m.direction === "out" ? "default" : "secondary"}>
                {m.direction === "out" ? "Trang gửi" : "Khách gửi"}
              </Badge>
              <span className="text-xs text-muted-foreground">{formatDateTime(m.inserted_at)}</span>
            </div>
            <p className="whitespace-pre-wrap break-words text-sm">{m.content}</p>
          </li>
        ))}
      </ul>

      {res.hasNextPage && (
        <Button variant="ghost" className="mt-3" onClick={() => res.fetchNextPage()} disabled={res.isFetchingNextPage}>
          {res.isFetchingNextPage ? "Đang tải…" : "Tải thêm"}
        </Button>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Add route**

In `web/src/app/router.tsx`, add import + route:
```tsx
import { SearchPage } from "@/features/search/search-page";

const searchRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/search",
  component: SearchPage,
});
```
Update `appRoute.addChildren([inboxRoute, searchRoute])`.

- [ ] **Step 3: Nav links in header**

In `web/src/app/root-layout.tsx`, add nav between title and logout:
```tsx
import { Link } from "@tanstack/react-router";
// ... inside header, after the title div:
<nav className="flex gap-4 text-sm">
  <Link to="/" className="hover:text-primary [&.active]:text-primary [&.active]:font-medium">Inbox</Link>
  <Link to="/search" className="hover:text-primary [&.active]:text-primary [&.active]:font-medium">Tìm kiếm</Link>
</nav>
```

- [ ] **Step 4: Smoke test**

Create `web/src/features/search/search-page.test.tsx`:
```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { SearchPage } from "./search-page";

vi.mock("./use-search", () => ({
  useSearchMessages: () => ({
    data: { pages: [] }, isLoading: false, hasNextPage: false,
    isFetchingNextPage: false, fetchNextPage: vi.fn(),
  }),
}));

describe("SearchPage", () => {
  it("renders the search input", () => {
    render(<SearchPage />);
    expect(screen.getByPlaceholderText("Tìm nội dung tin nhắn…")).toBeInTheDocument();
  });
});
```

- [ ] **Step 5: Run test + build**
Run: `cd web && npm run test && npm run build`
Expected: PASS + success.

- [ ] **Step 6: Commit**
```bash
git add web/ -- ':!web/node_modules'
git commit -m "feat: search page with route and nav"
```

---

## Task 7: Progress note
- [ ] Đổi `### Sprint 3 — Search (tiến độ: ☐)` → `(tiến độ: ✅)` trong spec, tick box, note "đã làm / chưa làm".
```bash
git add docs/
git commit -m "docs: sprint3 progress"
```

---

## Self-Review notes (đã kiểm)
- **Coverage Sprint 3:** FTS migration ✅(T1), search repo messages+customers ✅(T2), service+DTO ✅(T3), endpoints+wiring ✅(T4), FE service/hooks ✅(T5), search page+route+nav ✅(T6).
- **Type consistency:** `MsgSearchFilter`/`CustomerFilter` định nghĩa ở repo (T2), dùng ở ports/service (T3) và handler (T4). FE `Paged<Message>`/`Paged<Customer>` (Sprint 2 T3) tái dùng.
- **Reuse:** `fakeAuth`/`fakeQuery` từ Sprint 1 Task 10 router_test (cùng package). `MapMessages` tái dùng cho search; thêm `MapCustomers`.
- **Placeholder scan:** không có TODO/TBD.

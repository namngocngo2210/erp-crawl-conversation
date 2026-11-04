# Sprint 4 — Dashboard (stats + crawl-runs) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cung cấp thống kê tổng quan (tổng page/conversation/message, message theo ngày, theo page) và lịch sử crawl-runs; FE Dashboard hiển thị card + biểu đồ + bảng giám sát crawl.

**Architecture:** Mở rộng module read: `StatsRepository`/`StatsService` (aggregation bằng GORM) + endpoints `/stats/overview`, `/stats/crawl-runs`. FE feature `dashboard` dùng recharts (qua shadcn chart).

**Tech Stack:** Go 1.25, GORM, Gin; FE React + TanStack Query + recharts.

## Global Constraints
- GORM cho aggregation (`Select` + `Group` + `Scan` vào struct).
- DTO không trả `raw`.
- Mọi endpoint cần JWT.
- Commit message tiếng Anh, ngắn gọn.

---

## File Structure
**Create:**
- `internal/repository/read/stats_repo.go` — counts, daily message series, per-page counts, recent crawl runs.
- `internal/business/read/stats_service.go` — `StatsService`.
- `internal/transport/http/api/stats_handler.go` — handlers.
- `internal/dto/response/stats.go` — stats DTO.
- `web/src/services/stats.ts`, `web/src/features/dashboard/use-stats.ts`, `use-crawl-runs.ts`, `dashboard-page.tsx`
- Tests tương ứng.

**Modify:**
- `internal/business/read/ports.go` — `StatsRepo` interface.
- `internal/transport/http/api/router.go` — routes + `Stats` trong `Deps`.
- `cmd/crawler/inject.go` — provide stats service.
- `web/src/app/router.tsx` + `root-layout.tsx` — route `/dashboard` + nav.

---

## Task 1: StatsRepository (GORM aggregation)

**Files:**
- Create: `internal/repository/read/stats_repo.go`
- Test: `internal/repository/read/stats_repo_test.go` (integration)

**Interfaces:**
- Produces:
  - `type Totals struct { Pages int64; Conversations int64; Messages int64 }`
  - `type DailyCount struct { Day time.Time; Count int64 }`
  - `type PageCountRow struct { PageID int64; PageName string; Conversations int64; Messages int64 }`
  - `type StatsRepository interface { Totals(ctx) (Totals, error); MessagesByDay(ctx, days int) ([]DailyCount, error); CountsByPage(ctx) ([]PageCountRow, error); RecentCrawlRuns(ctx, limit int) ([]entity.CrawlRun, error) }`
  - `func NewStatsRepository(db *repository.DB) StatsRepository`
- Semantics: `MessagesByDay` group theo `date_trunc('day', inserted_at)` trong `days` gần nhất, asc. `CountsByPage` join pages → conversations/messages counts. `RecentCrawlRuns` order `started_at DESC` limit N.

- [ ] **Step 1: Write failing test**

Create `internal/repository/read/stats_repo_test.go`:
```go
package read

import (
	"context"
	"testing"
)

func TestTotals(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	repo := NewStatsRepository(db)
	got, err := repo.Totals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Pages < 0 || got.Conversations < 0 || got.Messages < 0 {
		t.Fatalf("unexpected negative totals: %+v", got)
	}
}
```

- [ ] **Step 2: Run test (fail)**
Run: `go test ./internal/repository/read/ -run TestTotals`
Expected: FAIL (undefined NewStatsRepository) or SKIP without DB; build must fail first.

- [ ] **Step 3: Implement repo**

Create `internal/repository/read/stats_repo.go`:
```go
package read

import (
	"context"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/desi/crawl-conversations/internal/repository"
)

type Totals struct {
	Pages         int64
	Conversations int64
	Messages      int64
}

type DailyCount struct {
	Day   time.Time
	Count int64
}

type PageCountRow struct {
	PageID        int64
	PageName      string
	Conversations int64
	Messages      int64
}

type StatsRepository interface {
	Totals(ctx context.Context) (Totals, error)
	MessagesByDay(ctx context.Context, days int) ([]DailyCount, error)
	CountsByPage(ctx context.Context) ([]PageCountRow, error)
	RecentCrawlRuns(ctx context.Context, limit int) ([]entity.CrawlRun, error)
}

type statsRepo struct{ db *repository.DB }

func NewStatsRepository(db *repository.DB) StatsRepository { return &statsRepo{db} }

func (r *statsRepo) Totals(ctx context.Context) (Totals, error) {
	var t Totals
	g := r.db.Gorm.WithContext(ctx)
	if err := g.Model(&entity.Page{}).Count(&t.Pages).Error; err != nil {
		return t, err
	}
	if err := g.Model(&entity.Conversation{}).Count(&t.Conversations).Error; err != nil {
		return t, err
	}
	if err := g.Model(&entity.Message{}).Count(&t.Messages).Error; err != nil {
		return t, err
	}
	return t, nil
}

func (r *statsRepo) MessagesByDay(ctx context.Context, days int) ([]DailyCount, error) {
	if days <= 0 {
		days = 30
	}
	since := time.Now().UTC().AddDate(0, 0, -days)
	var out []DailyCount
	err := r.db.Gorm.WithContext(ctx).
		Model(&entity.Message{}).
		Select("date_trunc('day', inserted_at) AS day, count(*) AS count").
		Where("inserted_at >= ?", since).
		Group("day").
		Order("day ASC").
		Scan(&out).Error
	return out, err
}

func (r *statsRepo) CountsByPage(ctx context.Context) ([]PageCountRow, error) {
	var out []PageCountRow
	err := r.db.Gorm.WithContext(ctx).
		Table("pages p").
		Select(`p.id AS page_id, p.name AS page_name,
			(SELECT count(*) FROM conversations c WHERE c.page_id = p.id) AS conversations,
			(SELECT count(*) FROM messages m WHERE m.page_id = p.id) AS messages`).
		Order("messages DESC").
		Scan(&out).Error
	return out, err
}

func (r *statsRepo) RecentCrawlRuns(ctx context.Context, limit int) ([]entity.CrawlRun, error) {
	if limit <= 0 {
		limit = 20
	}
	var out []entity.CrawlRun
	err := r.db.Gorm.WithContext(ctx).
		Order("started_at DESC").
		Limit(limit).
		Find(&out).Error
	return out, err
}
```

- [ ] **Step 4: Build + test**
Run: `go build ./... && go test ./internal/repository/read/`
Expected: PASS (integration SKIP without DB).

- [ ] **Step 5: Commit**
```bash
git add internal/repository/read/stats_repo.go internal/repository/read/stats_repo_test.go
git commit -m "feat: stats repository with aggregations"
```

---

## Task 2: Stats DTO + StatsService + ports

**Files:**
- Create: `internal/dto/response/stats.go`, `internal/business/read/stats_service.go`
- Modify: `internal/business/read/ports.go`
- Test: `internal/business/read/stats_service_test.go`

**Interfaces:**
- `ports.go`: `type StatsRepo interface { Totals(ctx) (reporead.Totals, error); MessagesByDay(ctx, days int) ([]reporead.DailyCount, error); CountsByPage(ctx) ([]reporead.PageCountRow, error); RecentCrawlRuns(ctx, limit int) ([]entity.CrawlRun, error) }`
- `stats_service.go`: `type Overview struct { Totals reporead.Totals; ByDay []reporead.DailyCount; ByPage []reporead.PageCountRow }`; `type StatsService interface { Overview(ctx, days int) (Overview, error); CrawlRuns(ctx, limit int) ([]entity.CrawlRun, error) }`; `func NewStatsService(repo StatsRepo) StatsService`.
- `stats.go` (DTO): `OverviewResp`, `CrawlRunResp` + mappers `MapOverview`, `MapCrawlRuns`.

- [ ] **Step 1: Write failing test**

Create `internal/business/read/stats_service_test.go`:
```go
package read

import (
	"context"
	"testing"

	"github.com/desi/crawl-conversations/internal/entity"
	reporead "github.com/desi/crawl-conversations/internal/repository/read"
)

type fakeStats struct{ days int }

func (f *fakeStats) Totals(context.Context) (reporead.Totals, error) {
	return reporead.Totals{Pages: 2, Conversations: 5, Messages: 99}, nil
}
func (f *fakeStats) MessagesByDay(_ context.Context, days int) ([]reporead.DailyCount, error) {
	f.days = days
	return nil, nil
}
func (f *fakeStats) CountsByPage(context.Context) ([]reporead.PageCountRow, error) { return nil, nil }
func (f *fakeStats) RecentCrawlRuns(context.Context, int) ([]entity.CrawlRun, error) { return nil, nil }

func TestOverviewDefaultDays(t *testing.T) {
	fs := &fakeStats{}
	svc := NewStatsService(fs)
	ov, err := svc.Overview(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if ov.Totals.Messages != 99 {
		t.Fatalf("want 99, got %d", ov.Totals.Messages)
	}
	if fs.days != 30 {
		t.Fatalf("want default 30 days, got %d", fs.days)
	}
}
```

- [ ] **Step 2: Run test (fail)**
Run: `go test ./internal/business/read/ -run TestOverviewDefaultDays`
Expected: FAIL.

- [ ] **Step 3: Implement ports + service + DTO**

Append to `internal/business/read/ports.go`:
```go
type StatsRepo interface {
	Totals(ctx context.Context) (reporead.Totals, error)
	MessagesByDay(ctx context.Context, days int) ([]reporead.DailyCount, error)
	CountsByPage(ctx context.Context) ([]reporead.PageCountRow, error)
	RecentCrawlRuns(ctx context.Context, limit int) ([]entity.CrawlRun, error)
}
```

Create `internal/business/read/stats_service.go`:
```go
package read

import (
	"context"

	"github.com/desi/crawl-conversations/internal/entity"
	reporead "github.com/desi/crawl-conversations/internal/repository/read"
)

type Overview struct {
	Totals reporead.Totals
	ByDay  []reporead.DailyCount
	ByPage []reporead.PageCountRow
}

type StatsService interface {
	Overview(ctx context.Context, days int) (Overview, error)
	CrawlRuns(ctx context.Context, limit int) ([]entity.CrawlRun, error)
}

type statsService struct{ repo StatsRepo }

func NewStatsService(repo StatsRepo) StatsService { return &statsService{repo: repo} }

func (s *statsService) Overview(ctx context.Context, days int) (Overview, error) {
	if days <= 0 {
		days = 30
	}
	totals, err := s.repo.Totals(ctx)
	if err != nil {
		return Overview{}, err
	}
	byDay, err := s.repo.MessagesByDay(ctx, days)
	if err != nil {
		return Overview{}, err
	}
	byPage, err := s.repo.CountsByPage(ctx)
	if err != nil {
		return Overview{}, err
	}
	return Overview{Totals: totals, ByDay: byDay, ByPage: byPage}, nil
}

func (s *statsService) CrawlRuns(ctx context.Context, limit int) ([]entity.CrawlRun, error) {
	return s.repo.RecentCrawlRuns(ctx, limit)
}
```

Create `internal/dto/response/stats.go`:
```go
package response

import (
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
	read "github.com/desi/crawl-conversations/internal/business/read"
)

type DailyCountResp struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

type PageCountResp struct {
	PageID        int64  `json:"page_id"`
	PageName      string `json:"page_name"`
	Conversations int64  `json:"conversations"`
	Messages      int64  `json:"messages"`
}

type OverviewResp struct {
	Pages         int64           `json:"pages"`
	Conversations int64           `json:"conversations"`
	Messages      int64           `json:"messages"`
	ByDay         []DailyCountResp `json:"by_day"`
	ByPage        []PageCountResp  `json:"by_page"`
}

type CrawlRunResp struct {
	ID         int64      `json:"id"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Trigger    string     `json:"trigger"`
	Status     string     `json:"status"`
	Error      string     `json:"error"`
}

func MapOverview(o read.Overview) OverviewResp {
	resp := OverviewResp{
		Pages: o.Totals.Pages, Conversations: o.Totals.Conversations, Messages: o.Totals.Messages,
	}
	for _, d := range o.ByDay {
		resp.ByDay = append(resp.ByDay, DailyCountResp{Day: d.Day.Format("2006-01-02"), Count: d.Count})
	}
	for _, p := range o.ByPage {
		resp.ByPage = append(resp.ByPage, PageCountResp{
			PageID: p.PageID, PageName: p.PageName, Conversations: p.Conversations, Messages: p.Messages,
		})
	}
	return resp
}

func MapCrawlRuns(runs []entity.CrawlRun) []CrawlRunResp {
	out := make([]CrawlRunResp, 0, len(runs))
	for _, r := range runs {
		out = append(out, CrawlRunResp{
			ID: r.ID, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt,
			Trigger: r.Trigger, Status: r.Status, Error: r.Error,
		})
	}
	return out
}
```

> Implementer note: `dto/response` giờ import `business/read` (cho type `Overview`). Kiểm tra không tạo import cycle: `business/read` KHÔNG được import `dto/response`. Handlers (transport) là nơi gọi cả hai — không cycle. Nếu lo ngại, có thể để `MapOverview` nhận các slice rời (`Totals`, `[]DailyCount`, `[]PageCountRow` từ `repository/read`) thay vì `read.Overview`. Ưu tiên cách nhận `repository/read` types để tránh phụ thuộc `business/read`:

Thay chữ ký `MapOverview` thành nhận `reporead` types (an toàn hơn về layering):
```go
import reporead "github.com/desi/crawl-conversations/internal/repository/read"

func MapOverview(t reporead.Totals, byDay []reporead.DailyCount, byPage []reporead.PageCountRow) OverviewResp {
	// ... dùng t, byDay, byPage ...
}
```
Và bỏ import `business/read` khỏi `stats.go`. Handler sẽ gọi `response.MapOverview(ov.Totals, ov.ByDay, ov.ByPage)`.

- [ ] **Step 4: Run test (pass)**
Run: `go test ./internal/business/read/ ./internal/dto/response/`
Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/business/read/ports.go internal/business/read/stats_service.go internal/business/read/stats_service_test.go internal/dto/response/stats.go
git commit -m "feat: stats service and response dto"
```

---

## Task 3: Stats handlers + router + DI

**Files:**
- Create: `internal/transport/http/api/stats_handler.go`
- Modify: `internal/transport/http/api/router.go`, `cmd/crawler/inject.go`
- Test: `internal/transport/http/api/stats_handler_test.go`

**Interfaces:**
- `Deps` thêm `Stats read.StatsService`.
- Routes (guarded): `GET /stats/overview?days=`, `GET /stats/crawl-runs?limit=`.

- [ ] **Step 1: Write failing test**

Create `internal/transport/http/api/stats_handler_test.go`:
```go
package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	read "github.com/desi/crawl-conversations/internal/business/read"
	"github.com/desi/crawl-conversations/internal/entity"
)

type fakeStatsSvc struct{}

func (fakeStatsSvc) Overview(context.Context, int) (read.Overview, error) {
	return read.Overview{}, nil
}
func (fakeStatsSvc) CrawlRuns(context.Context, int) ([]entity.CrawlRun, error) {
	return []entity.CrawlRun{{ID: 1, Status: "success"}}, nil
}

func setupStats() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r, Deps{Auth: fakeAuth{}, Query: fakeQuery{}, Search: fakeSearchSvc{}, Stats: fakeStatsSvc{}, CORSOrigin: "*"})
	return r
}

func TestOverviewEndpoint(t *testing.T) {
	r := setupStats()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stats/overview", nil)
	req.Header.Set("Authorization", "Bearer token123")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
}
```

> Implementer note: `fakeSearchSvc` từ Sprint 3 `search_handler_test.go` cùng package — tái dùng. Cập nhật `setup()`/`setupSearch()` cũ nếu cần thêm field `Stats` (Go cho phép bỏ field zero-value, nhưng nếu handler nil-deref thì test khác sẽ panic — các test cũ không gọi `/stats`, nên để `Stats` nil ở đó vẫn an toàn).

- [ ] **Step 2: Run test (fail)**
Run: `go test ./internal/transport/http/api/ -run TestOverviewEndpoint`
Expected: FAIL (Deps has no field Stats).

- [ ] **Step 3: Implement handler**

Create `internal/transport/http/api/stats_handler.go`:
```go
package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	read "github.com/desi/crawl-conversations/internal/business/read"
	"github.com/desi/crawl-conversations/internal/dto/response"
)

func handleOverview(s read.StatsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		days, _ := strconv.Atoi(c.Query("days"))
		ov, err := s.Overview(c.Request.Context(), days)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": "overview failed"}})
			return
		}
		c.JSON(http.StatusOK, response.MapOverview(ov.Totals, ov.ByDay, ov.ByPage))
	}
}

func handleCrawlRuns(s read.StatsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.Query("limit"))
		runs, err := s.CrawlRuns(c.Request.Context(), limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": "crawl runs failed"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": response.MapCrawlRuns(runs)})
	}
}
```

- [ ] **Step 4: Router + Deps + DI**

In `router.go`, add to `Deps`: `Stats read.StatsService`; add routes in auth group:
```go
	auth.GET("/stats/overview", handleOverview(d.Stats))
	auth.GET("/stats/crawl-runs", handleCrawlRuns(d.Stats))
```
In `cmd/crawler/inject.go`, add provider + Deps field:
```go
	do.Provide(i, func(in do.Injector) (readbiz.StatsService, error) {
		db := do.MustInvoke[*repository.DB](in)
		return readbiz.NewStatsService(readrepo.NewStatsRepository(db)), nil
	})
```
Add to `api.Deps`: `Stats: do.MustInvoke[readbiz.StatsService](in),`.

- [ ] **Step 5: Build + test**
Run: `go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**
```bash
git add internal/transport/http/api/ cmd/crawler/inject.go
git commit -m "feat: stats endpoints wired"
```

---

## Task 4: FE stats service + hooks

**Files:**
- Create: `web/src/services/stats.ts`, `web/src/features/dashboard/use-stats.ts`, `use-crawl-runs.ts`
- Modify: `web/src/types/api.ts` (add stats types)

**Interfaces:**
- types: `Overview { pages; conversations; messages; by_day: {day; count}[]; by_page: {...}[] }`, `CrawlRun { id; started_at; finished_at; trigger; status; error }`.
- `stats.ts`: `getOverview(days?)`, `getCrawlRuns(limit?)`.
- hooks: `useOverview(days)`, `useCrawlRuns(limit)`.

- [ ] **Step 1: Add types**

Append to `web/src/types/api.ts`:
```ts
export interface DailyCount { day: string; count: number; }
export interface PageCount { page_id: number; page_name: string; conversations: number; messages: number; }
export interface Overview {
  pages: number; conversations: number; messages: number;
  by_day: DailyCount[]; by_page: PageCount[];
}
export interface CrawlRun {
  id: number; started_at: string; finished_at: string | null;
  trigger: string; status: string; error: string;
}
```

- [ ] **Step 2: Service**

Create `web/src/services/stats.ts`:
```ts
import api from "./client";
import type { Overview, CrawlRun } from "@/types/api";

export async function getOverview(days?: number): Promise<Overview> {
  const { data } = await api.get<Overview>("/stats/overview", { params: { days } });
  return data;
}
export async function getCrawlRuns(limit?: number): Promise<{ items: CrawlRun[] }> {
  const { data } = await api.get<{ items: CrawlRun[] }>("/stats/crawl-runs", { params: { limit } });
  return data;
}
```

- [ ] **Step 3: Hooks**

Create `web/src/features/dashboard/use-stats.ts`:
```ts
import { useQuery } from "@tanstack/react-query";
import { getOverview } from "@/services/stats";

export function useOverview(days = 30) {
  return useQuery({ queryKey: ["overview", days], queryFn: () => getOverview(days) });
}
```

Create `web/src/features/dashboard/use-crawl-runs.ts`:
```ts
import { useQuery } from "@tanstack/react-query";
import { getCrawlRuns } from "@/services/stats";

export function useCrawlRuns(limit = 20) {
  return useQuery({ queryKey: ["crawl-runs", limit], queryFn: () => getCrawlRuns(limit) });
}
```

- [ ] **Step 4: Build**
Run: `cd web && npm run build`
Expected: success.

- [ ] **Step 5: Commit**
```bash
git add web/ -- ':!web/node_modules'
git commit -m "feat: frontend stats service and hooks"
```

---

## Task 5: Dashboard page (cards + chart + crawl-runs table)

**Files:**
- Create: `web/src/features/dashboard/dashboard-page.tsx`
- Modify: `web/src/app/router.tsx`, `web/src/app/root-layout.tsx`
- Test: `web/src/features/dashboard/dashboard-page.test.tsx`
- Install: `recharts`

**Interfaces:**
- `DashboardPage`: 3 stat card (pages/conversations/messages), 1 line chart message theo ngày (recharts), bảng top page theo messages, bảng crawl-runs gần đây (status màu).

- [ ] **Step 1: Install recharts**
Run: `cd web && npm install recharts`

- [ ] **Step 2: Implement dashboard page**

Create `web/src/features/dashboard/dashboard-page.tsx`:
```tsx
import { useOverview } from "./use-stats";
import { useCrawlRuns } from "./use-crawl-runs";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { formatDateTime } from "@/lib/format";
import {
  LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid,
} from "recharts";

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <Card className="p-4">
      <p className="text-sm text-muted-foreground">{label}</p>
      <p className="text-2xl font-semibold text-primary">{value.toLocaleString("vi-VN")}</p>
    </Card>
  );
}

const statusColor: Record<string, string> = {
  success: "bg-green-100 text-green-700",
  failed: "bg-red-100 text-red-700",
  running: "bg-blue-100 text-blue-700",
};

export function DashboardPage() {
  const ov = useOverview(30);
  const runs = useCrawlRuns(20);

  return (
    <div className="space-y-6 p-6">
      <div className="grid grid-cols-3 gap-4">
        <Stat label="Pages" value={ov.data?.pages ?? 0} />
        <Stat label="Conversations" value={ov.data?.conversations ?? 0} />
        <Stat label="Messages" value={ov.data?.messages ?? 0} />
      </div>

      <Card className="p-4">
        <h2 className="mb-3 text-sm font-medium">Tin nhắn theo ngày (30 ngày)</h2>
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={ov.data?.by_day ?? []}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="day" fontSize={11} />
              <YAxis fontSize={11} />
              <Tooltip />
              <Line type="monotone" dataKey="count" stroke="hsl(205 90% 48%)" dot={false} />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </Card>

      <div className="grid grid-cols-2 gap-4">
        <Card className="p-4">
          <h2 className="mb-3 text-sm font-medium">Top page theo tin nhắn</h2>
          <table className="w-full text-sm">
            <thead><tr className="text-left text-muted-foreground"><th>Page</th><th>Conv</th><th>Msg</th></tr></thead>
            <tbody>
              {(ov.data?.by_page ?? []).map((p) => (
                <tr key={p.page_id} className="border-t">
                  <td className="py-1">{p.page_name}</td><td>{p.conversations}</td><td>{p.messages}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>

        <Card className="p-4">
          <h2 className="mb-3 text-sm font-medium">Lịch sử crawl gần đây</h2>
          <table className="w-full text-sm">
            <thead><tr className="text-left text-muted-foreground"><th>Bắt đầu</th><th>Trigger</th><th>Trạng thái</th></tr></thead>
            <tbody>
              {(runs.data?.items ?? []).map((r) => (
                <tr key={r.id} className="border-t">
                  <td className="py-1">{formatDateTime(r.started_at)}</td>
                  <td>{r.trigger}</td>
                  <td><Badge className={statusColor[r.status] ?? ""}>{r.status}</Badge></td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Route + nav**

In `web/src/app/router.tsx`:
```tsx
import { DashboardPage } from "@/features/dashboard/dashboard-page";

const dashboardRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/dashboard",
  component: DashboardPage,
});
```
Update `appRoute.addChildren([inboxRoute, searchRoute, dashboardRoute])`.

In `web/src/app/root-layout.tsx`, add nav link:
```tsx
<Link to="/dashboard" className="hover:text-primary [&.active]:text-primary [&.active]:font-medium">Dashboard</Link>
```

- [ ] **Step 4: Smoke test**

Create `web/src/features/dashboard/dashboard-page.test.tsx`:
```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { DashboardPage } from "./dashboard-page";

vi.mock("./use-stats", () => ({
  useOverview: () => ({ data: { pages: 2, conversations: 5, messages: 99, by_day: [], by_page: [] } }),
}));
vi.mock("./use-crawl-runs", () => ({
  useCrawlRuns: () => ({ data: { items: [] } }),
}));

describe("DashboardPage", () => {
  it("renders totals", () => {
    render(<DashboardPage />);
    expect(screen.getByText("99")).toBeInTheDocument();
  });
});
```

> Implementer note: recharts `ResponsiveContainer` cần kích thước; trong jsdom có thể warn nhưng không fail test vì `by_day` rỗng. Nếu test lỗi do recharts, bọc chart trong điều kiện `ov.data?.by_day?.length` hoặc mock recharts.

- [ ] **Step 5: Run test + build**
Run: `cd web && npm run test && npm run build`
Expected: PASS + success.

- [ ] **Step 6: Commit**
```bash
git add web/ -- ':!web/node_modules'
git commit -m "feat: dashboard page with charts and crawl runs"
```

---

## Task 6: Progress note
- [ ] Đổi `### Sprint 4 — Dashboard (tiến độ: ☐)` → `(tiến độ: ✅)` trong spec; tick box; note "đã làm / chưa làm".
```bash
git add docs/
git commit -m "docs: sprint4 progress"
```

---

## Self-Review notes (đã kiểm)
- **Coverage Sprint 4:** stats repo aggregation ✅(T1), service+DTO ✅(T2), endpoints+wiring ✅(T3), FE service/hooks ✅(T4), dashboard page chart+tables ✅(T5).
- **Layering:** `MapOverview` nhận `repository/read` types để `dto/response` không phụ thuộc `business/read` (tránh import cycle) — ghi rõ trong T2.
- **Type consistency:** `Totals/DailyCount/PageCountRow` (repo T1) dùng ở ports/service (T2) và mapper (T2). FE `Overview/CrawlRun` (T4) khớp DTO BE.
- **Reuse test fakes:** `fakeAuth/fakeQuery/fakeSearchSvc` từ sprint trước, cùng package `api`.
- **Placeholder scan:** không có TODO/TBD.

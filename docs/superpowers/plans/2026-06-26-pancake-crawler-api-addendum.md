# API Adaptation Addendum — Pancake Crawler

> Created 2026-06-26 after Task 0.5 verified the live API (`docs/pancake-api-verified.md`).
> This addendum **supersedes the code** in plan tasks 2.1, 2.3, 4.1, 4.2. Implement those tasks from the exact code here. Interfaces/return types keep the same NAMES as the plan so Sprint 3 repos and the worker pool still compose; only field names, pagination, base URLs, and time parsing change.

## Summary of changes vs original plan
1. **Two base URLs:** account API `https://pages.fm/api/v1` (pages + generate token) and page API `https://pages.fm/api/public_api/v1` (conversations/messages/tags). New config field `PancakeAccountBaseURL`.
2. **GeneratePageToken:** `POST` with `Content-Type: application/json` body `{}`.
3. **List pages:** results under `categorized.activated[]` (not `data[]`).
4. **Tags:** id is integer, label field is `text` (not `name`); conversation tags are embedded objects in `tags[]`.
5. **Conversations:** `since`/`until` (unix seconds) and `page_number` are REQUIRED; no `has_next` — compute from `total` (page size 200).
6. **Messages:** paginate with `current_count` offset (step by returned count, stop on empty array). `page_number` is ignored. Sender is `from.id`; content is `original_message` (fallback `message`); direction inferred (`from.id == page_id` → outbound). Messages response also carries a richer `customers[]` (with phones) used to enrich customers.
7. **Timestamps:** format `2006-01-02T15:04:05.999999` with NO timezone — needs a custom `PancakeTime` unmarshaler (plain `time.Time` JSON unmarshal would fail).

---

## Task 2.1 (SUPERSEDED) — DTO structs + PancakeTime

**Files:** Create `internal/dto/time.go`, `internal/dto/time_test.go`, `internal/dto/pancake.go`.

### Step 1: failing test `internal/dto/time_test.go`
```go
package dto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPancakeTime_NoTZMicros(t *testing.T) {
	var pt PancakeTime
	require.NoError(t, pt.UnmarshalJSON([]byte(`"2026-06-14T02:00:49.000000"`)))
	require.Equal(t, 2026, pt.Year())
	require.Equal(t, time.June, pt.Month())
	require.Equal(t, time.UTC, pt.Location())
}

func TestPancakeTime_Seconds(t *testing.T) {
	var pt PancakeTime
	require.NoError(t, pt.UnmarshalJSON([]byte(`"2026-06-14T02:00:56"`)))
	require.Equal(t, 56, pt.Second())
}

func TestPancakeTime_Null(t *testing.T) {
	var pt PancakeTime
	require.NoError(t, pt.UnmarshalJSON([]byte(`null`)))
	require.True(t, pt.IsZero())
}

func TestPancakeTime_Invalid(t *testing.T) {
	var pt PancakeTime
	require.Error(t, pt.UnmarshalJSON([]byte(`"not-a-time"`)))
}
```

### Step 2: run `go test ./internal/dto/ -run TestPancakeTime -v` → FAIL.

### Step 3: `internal/dto/time.go`
```go
package dto

import (
	"fmt"
	"strings"
	"time"
)

// PancakeTime parses Pancake timestamps that have no timezone suffix,
// e.g. "2026-06-14T02:00:49.000000" or "2026-06-14T02:00:56". Treated as UTC.
type PancakeTime struct{ time.Time }

var pancakeLayouts = []string{
	"2006-01-02T15:04:05.999999",
	"2006-01-02T15:04:05",
	time.RFC3339Nano,
	time.RFC3339,
}

func (t *PancakeTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	for _, l := range pancakeLayouts {
		if tm, err := time.Parse(l, s); err == nil {
			t.Time = tm.UTC()
			return nil
		}
	}
	return fmt.Errorf("dto: cannot parse pancake time %q", s)
}
```

### Step 4: `internal/dto/pancake.go`
```go
package dto

import (
	"encoding/json"
	"time"
)

type ListParams struct {
	PageNumber   int        // conversations: 1-based page
	Since        *time.Time // conversations: filter by inserted_at (unix seconds)
	Until        *time.Time // conversations
	CurrentCount int        // messages: offset = number of messages already fetched
}

type PageDTO struct {
	ID         string
	Name       string
	Platform   string
	InsertedAt time.Time
	Raw        json.RawMessage
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
	ID    int
	Name  string // from API field "text"
	Color string
}

type ConversationDTO struct {
	ID           string
	Type         string
	Snippet      string
	MessageCount int
	UpdatedAt    time.Time
	InsertedAt   time.Time
	Customer     CustomerDTO
	Tags         []TagDTO
	Raw          json.RawMessage
}

type MessageDTO struct {
	ID          string
	SenderID    string // from.id
	Direction   string // "in" | "out" (computed: from.id == page_id => out)
	Type        string
	Content     string // original_message (fallback: message)
	Attachments json.RawMessage
	InsertedAt  time.Time
	Raw         json.RawMessage
}

type ConversationsResponse struct {
	Conversations []ConversationDTO
	Total         int
	HasNext       bool
}

type MessagesResponse struct {
	Messages  []MessageDTO
	Customers []CustomerDTO // richer customer data (incl. phones) from the messages envelope
}
```

### Step 5: `go test ./internal/dto/ -v` → PASS. Commit `feat: pancake dto structs and time parsing`.

---

## Config change (do as part of Task 2.3) — add account base URL

In `internal/config/config.go`:
- Add field `PancakeAccountBaseURL string` to `Config`.
- In `Load`, add: `PancakeAccountBaseURL: getDefault("PANCAKE_ACCOUNT_BASE_URL", "https://pages.fm/api/v1"),`

In `.env.example`, add after `PANCAKE_BASE_URL`:
```dotenv
PANCAKE_ACCOUNT_BASE_URL=https://pages.fm/api/v1
```
No new validation needed (it has a default). Existing config tests still pass.

---

## Task 2.3 (SUPERSEDED) — Pancake HTTP client

**Files:** Create `internal/pancake/client.go`, `internal/pancake/client_test.go`. (Task 2.2 rate limiter is unchanged — implement it from the original plan.)

**Interface (names unchanged from plan; constructor gains `accountBaseURL`):**
```go
type Client interface {
	ListPages(ctx context.Context) ([]dto.PageDTO, error)
	GeneratePageToken(ctx context.Context, pageID string) (string, error)
	ListConversations(ctx context.Context, pageID, pageToken string, p dto.ListParams) (dto.ConversationsResponse, error)
	ListMessages(ctx context.Context, pageID, conversationID, pageToken string, p dto.ListParams) (dto.MessagesResponse, error)
	ListTags(ctx context.Context, pageID, pageToken string) ([]dto.TagDTO, error)
}
func NewClient(pageBaseURL, accountBaseURL, accessToken string, rl *RateLimiter, hc *http.Client) Client
```

### Step 1: failing test `internal/pancake/client_test.go`
```go
package pancake

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"context"

	"github.com/desi/crawl-conversations/internal/dto"
	"github.com/stretchr/testify/require"
)

func TestListPages_CategorizedActivated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/pages", r.URL.Path)
		require.Equal(t, "acc", r.URL.Query().Get("access_token"))
		_, _ = w.Write([]byte(`{"success":true,"categorized":{"activated":[{"id":"945","name":"Shop","platform":"facebook","inserted_at":"2026-06-06T01:40:05"}]}}`))
	}))
	defer srv.Close()
	c := NewClient("http://unused", srv.URL, "acc", NewRateLimiter(100, 10), srv.Client())
	pages, err := c.ListPages(context.Background())
	require.NoError(t, err)
	require.Len(t, pages, 1)
	require.Equal(t, "945", pages[0].ID)
	require.Equal(t, "Shop", pages[0].Name)
}

func TestGeneratePageToken_PostJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Contains(t, r.URL.Path, "/generate_page_access_token")
		_, _ = w.Write([]byte(`{"success":true,"page_access_token":"PT123"}`))
	}))
	defer srv.Close()
	c := NewClient("http://unused", srv.URL, "acc", NewRateLimiter(100, 10), srv.Client())
	tok, err := c.GeneratePageToken(context.Background(), "945")
	require.NoError(t, err)
	require.Equal(t, "PT123", tok)
}

func TestListConversations_TotalDrivesHasNext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "PT", r.URL.Query().Get("page_access_token"))
		require.NotEmpty(t, r.URL.Query().Get("since"))
		require.NotEmpty(t, r.URL.Query().Get("until"))
		require.Equal(t, "1", r.URL.Query().Get("page_number"))
		_, _ = w.Write([]byte(`{"success":true,"total":554,"conversations":[{"id":"c1","type":"INBOX","updated_at":"2026-06-14T02:03:04.381000","tags":[{"id":46,"text":"VIP","color":"#bd2727"}],"customers":[{"id":"u1","name":"A","fb_id":"fb1"}],"page_customer":{"gender":"male"}}]}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "http://unused", "acc", NewRateLimiter(100, 10), srv.Client())
	resp, err := c.ListConversations(context.Background(), "945", "PT", dto.ListParams{PageNumber: 1})
	require.NoError(t, err)
	require.Equal(t, 554, resp.Total)
	require.True(t, resp.HasNext) // 1*200 < 554
	require.Len(t, resp.Conversations, 1)
	require.Equal(t, "VIP", resp.Conversations[0].Tags[0].Name)
	require.Equal(t, 46, resp.Conversations[0].Tags[0].ID)
	require.Equal(t, "u1", resp.Conversations[0].Customer.ID)
	require.Equal(t, "male", resp.Conversations[0].Customer.Gender)
}

func TestListMessages_FromIDDirectionAndContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "0", r.URL.Query().Get("current_count"))
		_, _ = w.Write([]byte(`{"success":true,"messages":[` +
			`{"id":"m1","from":{"id":"945"},"type":"INBOX","message":"<div>hi</div>","original_message":"hi","inserted_at":"2026-06-14T02:00:49.000000","attachments":[]},` +
			`{"id":"m2","from":{"id":"cust"},"type":"INBOX","original_message":"hello","inserted_at":"2026-06-14T02:01:00.000000","attachments":[]}` +
			`],"customers":[{"id":"u1","name":"A","recent_phone_numbers":["0900"],"personal_info":{"gender":"male"}}]}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "http://unused", "acc", NewRateLimiter(100, 10), srv.Client())
	resp, err := c.ListMessages(context.Background(), "945", "945_c1", "PT", dto.ListParams{CurrentCount: 0})
	require.NoError(t, err)
	require.Len(t, resp.Messages, 2)
	require.Equal(t, "945", resp.Messages[0].SenderID)
	require.Equal(t, "out", resp.Messages[0].Direction) // from.id == page_id
	require.Equal(t, "in", resp.Messages[1].Direction)
	require.Equal(t, "hi", resp.Messages[0].Content) // original_message preferred
	require.Len(t, resp.Customers, 1)
	require.Equal(t, "0900", resp.Customers[0].Phone)
	require.Equal(t, "male", resp.Customers[0].Gender)
}

func TestListTags_TextField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tags":[{"id":22,"text":"BOT","color":"#323da9"}]}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "http://unused", "acc", NewRateLimiter(100, 10), srv.Client())
	tags, err := c.ListTags(context.Background(), "945", "PT")
	require.NoError(t, err)
	require.Len(t, tags, 1)
	require.Equal(t, 22, tags[0].ID)
	require.Equal(t, "BOT", tags[0].Name)
}

func TestRetryOn500(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"categorized":{"activated":[]}}`))
	}))
	defer srv.Close()
	c := NewClient("http://unused", srv.URL, "acc", NewRateLimiter(100, 10), srv.Client())
	_, err := c.ListPages(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	_ = strings.TrimSpace("")
}
```

### Step 2: run `go test ./internal/pancake/ -v` → FAIL (NewClient signature).

### Step 3: `internal/pancake/client.go`
```go
package pancake

import (
	"bytes"
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

const conversationsPageSize = 200

type client struct {
	pageBaseURL    string
	accountBaseURL string
	accessToken    string
	rl             *RateLimiter
	hc             *http.Client
}

func NewClient(pageBaseURL, accountBaseURL, accessToken string, rl *RateLimiter, hc *http.Client) Client {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &client{pageBaseURL: pageBaseURL, accountBaseURL: accountBaseURL, accessToken: accessToken, rl: rl, hc: hc}
}

// doJSON issues an HTTP request with retry on 429/5xx and decodes JSON into out.
// pageID (when non-empty) drives the per-page rate limiter. body/contentType are optional.
func (c *client) doJSON(ctx context.Context, pageID, method, fullURL string, body []byte, contentType string, out any) error {
	const maxAttempts = 4
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if pageID != "" {
			if err := c.rl.Wait(ctx, pageID); err != nil {
				return err
			}
		}
		var reader io.Reader
		if body != nil {
			reader = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, fullURL, reader)
		if err != nil {
			return err
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		resp, err := c.hc.Do(req)
		if err != nil {
			lastErr = err
			backoff(ctx, attempt)
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("pancake status %d: %s", resp.StatusCode, string(respBody))
			backoff(ctx, attempt)
			continue
		}
		if resp.StatusCode >= 400 {
			return fmt.Errorf("pancake status %d: %s", resp.StatusCode, string(respBody))
		}
		return json.Unmarshal(respBody, out)
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
	u := fmt.Sprintf("%s/pages?access_token=%s", c.accountBaseURL, url.QueryEscape(c.accessToken))
	var raw struct {
		Categorized struct {
			Activated []struct {
				ID         string          `json:"id"`
				Name       string          `json:"name"`
				Platform   string          `json:"platform"`
				InsertedAt dto.PancakeTime `json:"inserted_at"`
			} `json:"activated"`
		} `json:"categorized"`
	}
	if err := c.doJSON(ctx, "", http.MethodGet, u, nil, "", &raw); err != nil {
		return nil, err
	}
	out := make([]dto.PageDTO, 0, len(raw.Categorized.Activated))
	for _, p := range raw.Categorized.Activated {
		out = append(out, dto.PageDTO{ID: p.ID, Name: p.Name, Platform: p.Platform, InsertedAt: p.InsertedAt.Time})
	}
	return out, nil
}

func (c *client) GeneratePageToken(ctx context.Context, pageID string) (string, error) {
	u := fmt.Sprintf("%s/pages/%s/generate_page_access_token?access_token=%s",
		c.accountBaseURL, url.PathEscape(pageID), url.QueryEscape(c.accessToken))
	var raw struct {
		PageAccessToken string `json:"page_access_token"`
		Success         bool   `json:"success"`
		Message         string `json:"message"`
	}
	if err := c.doJSON(ctx, "", http.MethodPost, u, []byte(`{}`), "application/json", &raw); err != nil {
		return "", err
	}
	if raw.PageAccessToken == "" {
		return "", fmt.Errorf("generate page token failed for %s: %s", pageID, raw.Message)
	}
	return raw.PageAccessToken, nil
}

type rawTag struct {
	ID    int    `json:"id"`
	Text  string `json:"text"`
	Color string `json:"color"`
}

func (c *client) ListConversations(ctx context.Context, pageID, pageToken string, p dto.ListParams) (dto.ConversationsResponse, error) {
	since := int64(0)
	if p.Since != nil {
		since = p.Since.Unix()
	}
	until := time.Now().Unix()
	if p.Until != nil {
		until = p.Until.Unix()
	}
	pageNum := p.PageNumber
	if pageNum < 1 {
		pageNum = 1
	}
	q := url.Values{}
	q.Set("page_access_token", pageToken)
	q.Set("page_number", strconv.Itoa(pageNum))
	q.Set("since", strconv.FormatInt(since, 10))
	q.Set("until", strconv.FormatInt(until, 10))
	q.Set("sort_by", "updated_at")
	u := fmt.Sprintf("%s/pages/%s/conversations?%s", c.pageBaseURL, url.PathEscape(pageID), q.Encode())

	var raw struct {
		Success       *bool  `json:"success"`
		Message       string `json:"message"`
		Total         int    `json:"total"`
		Conversations []struct {
			ID           string          `json:"id"`
			Type         string          `json:"type"`
			Snippet      string          `json:"snippet"`
			MessageCount int             `json:"message_count"`
			UpdatedAt    dto.PancakeTime `json:"updated_at"`
			InsertedAt   dto.PancakeTime `json:"inserted_at"`
			Tags         []rawTag        `json:"tags"`
			Customers    []struct {
				ID   string `json:"id"`
				FbID string `json:"fb_id"`
				Name string `json:"name"`
			} `json:"customers"`
			PageCustomer struct {
				Gender string `json:"gender"`
			} `json:"page_customer"`
		} `json:"conversations"`
	}
	if err := c.doJSON(ctx, pageID, http.MethodGet, u, nil, "", &raw); err != nil {
		return dto.ConversationsResponse{}, err
	}
	// Pancake returns HTTP 200 with success:false for logical errors
	// (e.g. "date range must be less than 1 month!"). Surface it as an error
	// instead of silently returning an empty list.
	if raw.Success != nil && !*raw.Success {
		return dto.ConversationsResponse{}, fmt.Errorf("pancake conversations error (page %s): %s", pageID, raw.Message)
	}
	out := dto.ConversationsResponse{Total: raw.Total, HasNext: pageNum*conversationsPageSize < raw.Total}
	for _, cv := range raw.Conversations {
		cust := dto.CustomerDTO{Gender: cv.PageCustomer.Gender}
		if len(cv.Customers) > 0 {
			cust.ID = cv.Customers[0].ID
			cust.FbID = cv.Customers[0].FbID
			cust.Name = cv.Customers[0].Name
		}
		tags := make([]dto.TagDTO, 0, len(cv.Tags))
		for _, t := range cv.Tags {
			tags = append(tags, dto.TagDTO{ID: t.ID, Name: t.Text, Color: t.Color})
		}
		out.Conversations = append(out.Conversations, dto.ConversationDTO{
			ID: cv.ID, Type: cv.Type, Snippet: cv.Snippet, MessageCount: cv.MessageCount,
			UpdatedAt: cv.UpdatedAt.Time, InsertedAt: cv.InsertedAt.Time, Customer: cust, Tags: tags,
		})
	}
	return out, nil
}

func (c *client) ListMessages(ctx context.Context, pageID, conversationID, pageToken string, p dto.ListParams) (dto.MessagesResponse, error) {
	q := url.Values{}
	q.Set("page_access_token", pageToken)
	q.Set("current_count", strconv.Itoa(p.CurrentCount))
	u := fmt.Sprintf("%s/pages/%s/conversations/%s/messages?%s",
		c.pageBaseURL, url.PathEscape(pageID), url.PathEscape(conversationID), q.Encode())

	var raw struct {
		Messages []struct {
			ID         string          `json:"id"`
			Type       string          `json:"type"`
			Message    string          `json:"message"`
			Original   string          `json:"original_message"`
			InsertedAt dto.PancakeTime `json:"inserted_at"`
			From       struct {
				ID string `json:"id"`
			} `json:"from"`
			Attachments json.RawMessage `json:"attachments"`
		} `json:"messages"`
		Customers []struct {
			ID           string   `json:"id"`
			Name         string   `json:"name"`
			FbID         string   `json:"fb_id"`
			Phones       []string `json:"recent_phone_numbers"`
			PersonalInfo struct {
				Gender string `json:"gender"`
			} `json:"personal_info"`
		} `json:"customers"`
	}
	if err := c.doJSON(ctx, pageID, http.MethodGet, u, nil, "", &raw); err != nil {
		return dto.MessagesResponse{}, err
	}
	out := dto.MessagesResponse{}
	for _, m := range raw.Messages {
		content := m.Original
		if content == "" {
			content = m.Message
		}
		direction := "in"
		if m.From.ID == pageID {
			direction = "out"
		}
		out.Messages = append(out.Messages, dto.MessageDTO{
			ID: m.ID, SenderID: m.From.ID, Direction: direction, Type: m.Type,
			Content: content, Attachments: m.Attachments, InsertedAt: m.InsertedAt.Time,
		})
	}
	for _, cu := range raw.Customers {
		phone := ""
		if len(cu.Phones) > 0 {
			phone = cu.Phones[0]
		}
		out.Customers = append(out.Customers, dto.CustomerDTO{
			ID: cu.ID, Name: cu.Name, FbID: cu.FbID, Phone: phone, Gender: cu.PersonalInfo.Gender,
		})
	}
	return out, nil
}

func (c *client) ListTags(ctx context.Context, pageID, pageToken string) ([]dto.TagDTO, error) {
	u := fmt.Sprintf("%s/pages/%s/tags?page_access_token=%s",
		c.pageBaseURL, url.PathEscape(pageID), url.QueryEscape(pageToken))
	var raw struct {
		Tags []rawTag `json:"tags"`
	}
	if err := c.doJSON(ctx, pageID, http.MethodGet, u, nil, "", &raw); err != nil {
		return nil, err
	}
	out := make([]dto.TagDTO, 0, len(raw.Tags))
	for _, t := range raw.Tags {
		out = append(out, dto.TagDTO{ID: t.ID, Name: t.Text, Color: t.Color})
	}
	return out, nil
}
```

### Step 4: `go test ./internal/pancake/ -v` → PASS. Commit `feat: pancake http client matching verified api`.

---

## Task 2.3b — fix: ListConversations must error on `success:false` (live-validation finding)

Task 2.3 is already committed. Apply this small fix (the canonical `client.go` code above already includes it):

**Files:** Modify `internal/pancake/client.go`, `internal/pancake/client_test.go`.

1. In `ListConversations`, add `Success *bool json:"success"` and `Message string json:"message"` to the `raw` struct, and after `doJSON` succeeds, add:
   ```go
   if raw.Success != nil && !*raw.Success {
       return dto.ConversationsResponse{}, fmt.Errorf("pancake conversations error (page %s): %s", pageID, raw.Message)
   }
   ```
   (`*bool` so the existing success-less test fixtures, which omit `success`, are treated as success.)

2. Add a test:
   ```go
   func TestListConversations_SuccessFalseIsError(t *testing.T) {
   	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
   		_, _ = w.Write([]byte(`{"success":false,"message":"date range must be less than 1 month!"}`))
   	}))
   	defer srv.Close()
   	c := NewClient(srv.URL, "http://unused", "acc", NewRateLimiter(100, 10), srv.Client())
   	_, err := c.ListConversations(context.Background(), "945", "PT", dto.ListParams{PageNumber: 1})
   	require.Error(t, err)
   	require.Contains(t, err.Error(), "less than 1 month")
   }
   ```

3. Run `go test ./internal/pancake/ -v` → all PASS (existing client tests have `success:true` or omit it, so they still pass). Commit `fix: error on pancake success:false in conversations`.

---

## Task 4.1 (SUPERSEDED) — DTO → entity mappers

**Files:** Create `internal/dto/mapper.go`, `internal/dto/mapper_test.go`.

### Step 1: failing test `internal/dto/mapper_test.go`
```go
package dto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMapTag_IntIDToString(t *testing.T) {
	tag := MapTag(7, TagDTO{ID: 46, Name: "VIP", Color: "#fff"})
	require.Equal(t, int64(7), tag.PageID)
	require.Equal(t, "46", tag.PancakeTagID)
	require.Equal(t, "VIP", tag.Name)
}

func TestMapMessage(t *testing.T) {
	ts := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	m := MapMessage(7, 9, MessageDTO{ID: "m1", SenderID: "s1", Direction: "out", Type: "INBOX", Content: "hi", InsertedAt: ts})
	require.Equal(t, int64(7), m.PageID)
	require.Equal(t, int64(9), m.ConversationID)
	require.Equal(t, "m1", m.PancakeMessageID)
	require.Equal(t, "out", m.Direction)
	require.Equal(t, ts, m.InsertedAt)
}

func TestMapConversation_UsesUpdatedAtForLastMessage(t *testing.T) {
	cid := int64(3)
	ts := time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC)
	c := MapConversation(7, &cid, ConversationDTO{ID: "c1", Type: "INBOX", MessageCount: 5, UpdatedAt: ts})
	require.Equal(t, "c1", c.PancakeConversationID)
	require.Equal(t, &cid, c.CustomerID)
	require.Equal(t, ts, c.UpdatedAt)
	require.Equal(t, ts, c.LastMessageAt) // proxy: updated_at
}
```

### Step 2: run `go test ./internal/dto/ -run TestMap -v` → FAIL.

### Step 3: `internal/dto/mapper.go`
```go
package dto

import (
	"strconv"

	"github.com/desi/crawl-conversations/internal/entity"
)

func MapCustomer(pageID int64, c CustomerDTO) entity.Customer {
	return entity.Customer{
		PageID: pageID, PancakeCustomerID: c.ID, Name: c.Name, Phone: c.Phone,
		Email: c.Email, FbID: c.FbID, Gender: c.Gender, Raw: c.Raw,
	}
}

func MapTag(pageID int64, t TagDTO) entity.Tag {
	return entity.Tag{PageID: pageID, PancakeTagID: strconv.Itoa(t.ID), Name: t.Name, Color: t.Color}
}

func MapConversation(pageID int64, customerID *int64, c ConversationDTO) entity.Conversation {
	return entity.Conversation{
		PageID: pageID, PancakeConversationID: c.ID, CustomerID: customerID,
		Type: c.Type, Snippet: c.Snippet, MessageCount: c.MessageCount,
		LastMessageAt: c.UpdatedAt, UpdatedAt: c.UpdatedAt, Raw: c.Raw,
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

### Step 4: `go test ./internal/dto/ -run TestMap -v` → PASS. Commit `feat: dto to entity mappers`.

---

## Task 4.2 (SUPERSEDED) — PageProcessor

**Files:** Create `internal/business/ports.go` (unchanged from plan — same `PancakeClient`, `TokenManager`, `Stats`, `Repos`), `internal/business/page_processor.go`, `internal/business/page_processor_test.go`.

Key behavior changes vs plan (driven by the live-validation findings):
- **Month-windowed conversation scan:** the API requires each `since..until` span to be **< 1 month** (filtered by `inserted_at`). So BOTH modes walk the full history `[start, now]` in ≤29-day windows; within each window paginate `page_number` until `HasNext` is false. `start = page.InsertedAt` (minus a 1-day margin), falling back to `now - defaultBackfillLookback` when the page has no `InsertedAt`.
- **Incremental gates on `updated_at`, not `since`:** because `since` filters `inserted_at`, an old conversation with a new message would be missed if we narrowed `since` to the cursor. Instead, incremental scans the **same full history** but only does the expensive message fetch when `backfill || conv.UpdatedAt.After(cursor - overlap)`. Conversation/customer/tag upserts are always done (cheap, idempotent).
- Tags: upsert each tag from `conv.Tags` (embedded) and link — no page-tag map keyed by string.
- Messages: paginate with `current_count` offset, stop when an empty page returns; also capture the richer `customers[]` from the first non-empty messages response and re-upsert the customer (adds phone/gender).
- Customer ordering: upsert customer from the conversation first (for the FK), then re-upsert with the richer message-customer data after fetching messages.
- Checkpoint cursor = max `updated_at` seen across all conversations this run.

### Step 1: failing test `internal/business/page_processor_test.go`
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

type fakeClient struct{}

func (fakeClient) ListPages(ctx context.Context) ([]dto.PageDTO, error) { return nil, nil }
func (fakeClient) ListTags(ctx context.Context, pageID, tok string) ([]dto.TagDTO, error) {
	return []dto.TagDTO{{ID: 1, Name: "VIP"}}, nil
}
// convTime is the conversation's inserted_at; the fake returns the conversation
// only in the ≤29-day window whose [Since,Until) brackets it, mimicking the real
// API's inserted_at filter. This makes the windowed scan return it exactly once.
var convTime = time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)

func (fakeClient) ListConversations(ctx context.Context, pageID, tok string, p dto.ListParams) (dto.ConversationsResponse, error) {
	if p.PageNumber > 1 || p.Since == nil || p.Until == nil {
		return dto.ConversationsResponse{}, nil
	}
	if convTime.Before(*p.Since) || !convTime.Before(*p.Until) {
		return dto.ConversationsResponse{}, nil // not in this window
	}
	return dto.ConversationsResponse{Total: 1, HasNext: false, Conversations: []dto.ConversationDTO{
		{ID: "c1", Type: "INBOX", InsertedAt: convTime, UpdatedAt: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
			Tags:     []dto.TagDTO{{ID: 1, Name: "VIP"}},
			Customer: dto.CustomerDTO{ID: "cust1", Name: "Anh"}},
	}}, nil
}
func (fakeClient) ListMessages(ctx context.Context, pageID, convID, tok string, p dto.ListParams) (dto.MessagesResponse, error) {
	if p.CurrentCount > 0 {
		return dto.MessagesResponse{}, nil // end of history
	}
	return dto.MessagesResponse{
		Messages:  []dto.MessageDTO{{ID: "m1", Type: "INBOX", Content: "hi", Direction: "in", InsertedAt: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)}},
		Customers: []dto.CustomerDTO{{ID: "cust1", Name: "Anh", Phone: "0900"}},
	}, nil
}

type fakeTokens struct{}

func (fakeTokens) Get(ctx context.Context, pageID string) (string, error) { return "ptok", nil }

type spyMsg struct{ bulk, upsert int }

func (s *spyMsg) UpsertBatch(ctx context.Context, m []entity.Message) error { s.upsert++; return nil }
func (s *spyMsg) BulkLoad(ctx context.Context, m []entity.Message) (int64, error) {
	s.bulk++
	return int64(len(m)), nil
}

type spyCustomer struct{ upserts int }

func (s *spyCustomer) Upsert(ctx context.Context, c entity.Customer) (int64, error) {
	s.upserts++
	return 2, nil
}

type stubPage struct{}

func (stubPage) Upsert(ctx context.Context, p entity.Page) (int64, error) { return 1, nil }

type stubTag struct{}

func (stubTag) Upsert(ctx context.Context, t entity.Tag) (int64, error) { return 3, nil }

type stubConv struct{ tagsSet int }

func (stubConv) Upsert(ctx context.Context, c entity.Conversation) (int64, error) { return 4, nil }
func (s *stubConv) SetTags(ctx context.Context, id int64, ids []int64) error      { s.tagsSet = len(ids); return nil }

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

func newRepos(msg *spyMsg, cp *stubCheckpoint, cust *spyCustomer, conv *stubConv) Repos {
	return Repos{Page: stubPage{}, Customer: cust, Tag: stubTag{}, Conversation: conv, Message: msg, Checkpoint: cp}
}

func TestProcess_BackfillUsesBulkLoad(t *testing.T) {
	msg := &spyMsg{}
	cp := &stubCheckpoint{found: false}
	cust := &spyCustomer{}
	conv := &stubConv{}
	p := NewPageProcessor(fakeClient{}, fakeTokens{}, newRepos(msg, cp, cust, conv), 5*time.Second)
	st, err := p.Process(context.Background(), entity.Page{ID: 1, PancakePageID: "pg1"})
	require.NoError(t, err)
	require.Equal(t, 1, msg.bulk)
	require.Equal(t, 0, msg.upsert)
	require.Equal(t, 1, st.Messages)
	require.GreaterOrEqual(t, cust.upserts, 2) // once from conv, once enriched from messages
	require.Equal(t, 1, conv.tagsSet)
	require.NotNil(t, cp.saved)
	require.WithinDuration(t, time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), *cp.saved.Cursor, time.Second)
}

func TestProcess_IncrementalUsesUpsert(t *testing.T) {
	msg := &spyMsg{}
	cur := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	cp := &stubCheckpoint{found: true, cp: entity.CrawlCheckpoint{PageID: 1, Cursor: &cur}}
	p := NewPageProcessor(fakeClient{}, fakeTokens{}, newRepos(msg, cp, &spyCustomer{}, &stubConv{}), 5*time.Second)
	_, err := p.Process(context.Background(), entity.Page{ID: 1, PancakePageID: "pg1"})
	require.NoError(t, err)
	require.Equal(t, 0, msg.bulk)
	require.Equal(t, 1, msg.upsert)
}

func TestProcess_IncrementalSkipsUnchanged(t *testing.T) {
	msg := &spyMsg{}
	// cursor newer than the conversation's updated_at (2026-06-10) → unchanged → skip message fetch
	cur := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	cp := &stubCheckpoint{found: true, cp: entity.CrawlCheckpoint{PageID: 1, Cursor: &cur}}
	conv := &stubConv{}
	p := NewPageProcessor(fakeClient{}, fakeTokens{}, newRepos(msg, cp, &spyCustomer{}, conv), 5*time.Second)
	st, err := p.Process(context.Background(), entity.Page{ID: 1, PancakePageID: "pg1"})
	require.NoError(t, err)
	require.Equal(t, 0, msg.upsert) // unchanged → no message fetch/write
	require.Equal(t, 0, msg.bulk)
	require.Equal(t, 1, st.Conversations) // conversation itself still upserted
}
```

### Step 2: run `go test ./internal/business/ -run TestProcess -v` → FAIL.

### Step 3: `internal/business/ports.go` — identical to the original plan Task 4.2 (copy it verbatim from the plan: `PancakeClient`, `TokenManager`, `Stats`, `Repos`).

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

const (
	conversationWindow      = 29 * 24 * time.Hour      // API requires since..until span < 1 month
	defaultBackfillLookback = 3 * 365 * 24 * time.Hour // used when page.InsertedAt is unknown
)

func (p *pageProcessor) Process(ctx context.Context, page entity.Page) (Stats, error) {
	var st Stats
	tok, err := p.tokens.Get(ctx, page.PancakePageID)
	if err != nil {
		return st, err
	}

	// Capture the page's tag catalog (idempotent).
	tags, err := p.client.ListTags(ctx, page.PancakePageID, tok)
	if err != nil {
		return st, err
	}
	for _, t := range tags {
		if _, err := p.repos.Tag.Upsert(ctx, dto.MapTag(page.ID, t)); err != nil {
			return st, err
		}
	}

	cp, found, err := p.repos.Checkpoint.Get(ctx, page.ID)
	if err != nil {
		return st, err
	}
	backfill := !found

	var cursor time.Time
	if found && cp.Cursor != nil {
		cursor = *cp.Cursor
	}
	msgThreshold := cursor.Add(-p.overlap) // only (re)fetch messages for convs updated after this
	maxUpdated := cursor

	// Conversations are filtered by inserted_at and each since..until window must
	// span < 1 month, so walk [start, now] in <=29-day windows for BOTH modes.
	// Incremental does NOT narrow the window by cursor (the cursor is updated_at,
	// the filter is inserted_at) — it relies on the per-conversation updated_at
	// gate below to skip the expensive message fetch for unchanged conversations.
	end := time.Now()
	start := page.InsertedAt
	if start.IsZero() {
		start = end.Add(-defaultBackfillLookback)
	}
	start = start.Add(-24 * time.Hour) // safety margin

	processConv := func(c dto.ConversationDTO) error {
		var custID *int64
		if c.Customer.ID != "" {
			id, err := p.repos.Customer.Upsert(ctx, dto.MapCustomer(page.ID, c.Customer))
			if err != nil {
				return err
			}
			custID = &id
			st.Customers++
		}
		convID, err := p.repos.Conversation.Upsert(ctx, dto.MapConversation(page.ID, custID, c))
		if err != nil {
			return err
		}
		st.Conversations++

		tagIDs := make([]int64, 0, len(c.Tags))
		for _, t := range c.Tags {
			id, err := p.repos.Tag.Upsert(ctx, dto.MapTag(page.ID, t))
			if err != nil {
				return err
			}
			tagIDs = append(tagIDs, id)
		}
		if err := p.repos.Conversation.SetTags(ctx, convID, tagIDs); err != nil {
			return err
		}

		// Expensive step: fetch messages only on backfill, or when the conversation
		// changed since the last run.
		if backfill || c.UpdatedAt.After(msgThreshold) {
			msgs, richCustomers, err := p.fetchMessages(ctx, page, convID, c.ID, tok)
			if err != nil {
				return err
			}
			for _, rc := range richCustomers {
				if c.Customer.ID != "" && rc.ID == c.Customer.ID {
					if _, err := p.repos.Customer.Upsert(ctx, dto.MapCustomer(page.ID, rc)); err != nil {
						return err
					}
				}
			}
			if len(msgs) > 0 {
				if backfill {
					if _, err := p.repos.Message.BulkLoad(ctx, msgs); err != nil {
						return err
					}
				} else {
					if err := p.repos.Message.UpsertBatch(ctx, msgs); err != nil {
						return err
					}
				}
				st.Messages += len(msgs)
			}
		}
		if c.UpdatedAt.After(maxUpdated) {
			maxUpdated = c.UpdatedAt
		}
		return nil
	}

	for winStart := start; winStart.Before(end); winStart = winStart.Add(conversationWindow) {
		winEnd := winStart.Add(conversationWindow)
		if winEnd.After(end) {
			winEnd = end
		}
		ws, we := winStart, winEnd
		pageNum := 1
		for {
			resp, err := p.client.ListConversations(ctx, page.PancakePageID, tok, dto.ListParams{PageNumber: pageNum, Since: &ws, Until: &we})
			if err != nil {
				return st, err
			}
			if len(resp.Conversations) == 0 {
				break
			}
			for _, c := range resp.Conversations {
				if err := processConv(c); err != nil {
					return st, err
				}
			}
			if !resp.HasNext {
				break
			}
			pageNum++
		}
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

// fetchMessages walks the full message history via the current_count offset,
// stopping when an empty page is returned. Returns the mapped messages and the
// richer customer list from the first non-empty messages response.
func (p *pageProcessor) fetchMessages(ctx context.Context, page entity.Page, convID int64, pancakeConvID, tok string) ([]entity.Message, []dto.CustomerDTO, error) {
	var out []entity.Message
	var customers []dto.CustomerDTO
	collected := 0
	for {
		resp, err := p.client.ListMessages(ctx, page.PancakePageID, pancakeConvID, tok, dto.ListParams{CurrentCount: collected})
		if err != nil {
			return nil, nil, err
		}
		if len(resp.Messages) == 0 {
			break
		}
		if customers == nil {
			customers = resp.Customers
		}
		for _, m := range resp.Messages {
			out = append(out, dto.MapMessage(page.ID, convID, m))
		}
		collected += len(resp.Messages)
	}
	return out, customers, nil
}
```

### Step 4: `go test ./internal/business/ -run TestProcess -v` → PASS. Commit `feat: page processor matching verified api`.

> Note for Task 4.4 (CrawlService) and Task 5.3 (DI): `NewClient` now takes `(pageBaseURL, accountBaseURL, ...)`. The DI provider must pass both `cfg.PancakeBaseURL` and `cfg.PancakeAccountBaseURL`. The original plan's `business.PancakeClient` interface and `CrawlService` are otherwise unchanged.

## Known limitation to record (README + Sprint 4 notes)
The messages endpoint has no server-side time filter (`since`/`until` ignored). Incremental runs still fetch each changed conversation's messages from `current_count=0`; the DB `ON CONFLICT DO NOTHING` dedup makes re-fetched messages free of duplicates. For very long conversations this re-reads pages until reaching the bottom; acceptable for a daily run, but a future optimization is to stop paging once a page's oldest message is older than the checkpoint.

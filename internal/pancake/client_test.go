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
		_, _ = w.Write([]byte(`{"success":true,"recent_emails":["x@y.com"],"messages":[` +
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
	require.Equal(t, "x@y.com", resp.Customers[0].Email)
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

func TestListMessages_SuccessFalseIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"message":"Invalid access_token"}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "http://unused", "acc", NewRateLimiter(100, 10), srv.Client())
	_, err := c.ListMessages(context.Background(), "945", "conv1", "PT", dto.ListParams{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid access_token")
}

func TestListTags_SuccessFalseIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"message":"Invalid access_token"}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "http://unused", "acc", NewRateLimiter(100, 10), srv.Client())
	_, err := c.ListTags(context.Background(), "945", "PT")
	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid access_token")
}

func TestListMessages_PhonesAsObjects(t *testing.T) {
	// Some pages return recent_phone_numbers / recent_emails as arrays of OBJECTS
	// instead of strings; the client must tolerate both shapes.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"recent_emails":[{"email":"a@b.com"}],"messages":[` +
			`{"id":"m1","from":{"id":"945"},"type":"INBOX","original_message":"hi","inserted_at":"2026-06-14T02:00:49.000000","attachments":[]}` +
			`],"customers":[{"id":"u1","name":"A","recent_phone_numbers":[{"phone_number":"0900","type":"mobile"}],"personal_info":{"gender":"male"}}]}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "http://unused", "acc", NewRateLimiter(100, 10), srv.Client())
	resp, err := c.ListMessages(context.Background(), "945", "945_c1", "PT", dto.ListParams{CurrentCount: 0})
	require.NoError(t, err) // must NOT fail to unmarshal
	require.Len(t, resp.Messages, 1)
	require.Len(t, resp.Customers, 1)
	require.Equal(t, "0900", resp.Customers[0].Phone)
	require.Equal(t, "a@b.com", resp.Customers[0].Email)
}

func TestListMessages_PhonesAsStringsStillWork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"recent_emails":["a@b.com"],"messages":[],"customers":[{"id":"u1","recent_phone_numbers":["0911"]}]}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "http://unused", "acc", NewRateLimiter(100, 10), srv.Client())
	resp, err := c.ListMessages(context.Background(), "945", "945_c1", "PT", dto.ListParams{CurrentCount: 0})
	require.NoError(t, err)
	require.Len(t, resp.Customers, 1)
	require.Equal(t, "0911", resp.Customers[0].Phone)
	require.Equal(t, "a@b.com", resp.Customers[0].Email)
}

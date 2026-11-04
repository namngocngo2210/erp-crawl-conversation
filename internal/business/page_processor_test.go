package business

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
func (fakeTokens) Invalidate(ctx context.Context, pageID string) error    { return nil }

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

type errorOnFirstConvClient struct{}

func (errorOnFirstConvClient) ListPages(ctx context.Context) ([]dto.PageDTO, error) { return nil, nil }
func (errorOnFirstConvClient) ListTags(ctx context.Context, pageID, tok string) ([]dto.TagDTO, error) {
	return []dto.TagDTO{}, nil
}
func (errorOnFirstConvClient) ListConversations(ctx context.Context, pageID, tok string, p dto.ListParams) (dto.ConversationsResponse, error) {
	return dto.ConversationsResponse{}, errors.New("network error")
}
func (errorOnFirstConvClient) ListMessages(ctx context.Context, pageID, convID, tok string, p dto.ListParams) (dto.MessagesResponse, error) {
	return dto.MessagesResponse{}, nil
}

func TestProcess_DoesNotSaveCheckpointOnError(t *testing.T) {
	cp := &stubCheckpoint{found: false}
	p := NewPageProcessor(errorOnFirstConvClient{}, fakeTokens{}, newRepos(&spyMsg{}, cp, &spyCustomer{}, &stubConv{}), 5*time.Second)
	_, err := p.Process(context.Background(), entity.Page{ID: 1, PancakePageID: "pg1"})
	require.Error(t, err)
	require.Nil(t, cp.saved, "checkpoint must not be saved when error occurs")
}

func TestProcess_CheckpointIsMaxUpdatedAt(t *testing.T) {
	msg := &spyMsg{}
	cp := &stubCheckpoint{found: false}
	p := NewPageProcessor(fakeClient{}, fakeTokens{}, newRepos(msg, cp, &spyCustomer{}, &stubConv{}), 5*time.Second)
	_, err := p.Process(context.Background(), entity.Page{ID: 1, PancakePageID: "pg1"})
	require.NoError(t, err)
	require.NotNil(t, cp.saved)
	// cursor must equal the conversation's UpdatedAt (2026-06-10T00:00:00Z)
	want := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	require.Equal(t, want.UTC(), cp.saved.Cursor.UTC())
}

type retryFakeClient struct {
	calls int
}

func (r *retryFakeClient) ListPages(ctx context.Context) ([]dto.PageDTO, error) { return nil, nil }
func (r *retryFakeClient) ListConversations(ctx context.Context, pageID, tok string, p dto.ListParams) (dto.ConversationsResponse, error) {
	return dto.ConversationsResponse{}, nil
}
func (r *retryFakeClient) ListMessages(ctx context.Context, pageID, convID, tok string, p dto.ListParams) (dto.MessagesResponse, error) {
	return dto.MessagesResponse{}, nil
}
func (r *retryFakeClient) ListTags(ctx context.Context, pageID, tok string) ([]dto.TagDTO, error) {
	r.calls++
	if r.calls == 1 {
		return nil, fmt.Errorf("pancake tags error: Invalid access_token")
	}
	return []dto.TagDTO{}, nil
}

type spyTokens struct {
	invalidated int
	calls       int
}

func (s *spyTokens) Get(ctx context.Context, pageID string) (string, error) {
	s.calls++
	return "tok" + strconv.Itoa(s.calls), nil
}
func (s *spyTokens) Invalidate(ctx context.Context, pageID string) error {
	s.invalidated++
	return nil
}

func TestProcess_RetriesOnInvalidToken(t *testing.T) {
	msg := &spyMsg{}
	cp := &stubCheckpoint{found: false}
	cust := &spyCustomer{}
	conv := &stubConv{}
	client := &retryFakeClient{}
	tokens := &spyTokens{}
	p := NewPageProcessor(client, tokens, newRepos(msg, cp, cust, conv), 5*time.Second)
	_, err := p.Process(context.Background(), entity.Page{ID: 1, PancakePageID: "pg1"})
	require.NoError(t, err)
	require.Equal(t, 1, tokens.invalidated)
	require.Equal(t, 2, client.calls) // called twice: first fails, second succeeds
}

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

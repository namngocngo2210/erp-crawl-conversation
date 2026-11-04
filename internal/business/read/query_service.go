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

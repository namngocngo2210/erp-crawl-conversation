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

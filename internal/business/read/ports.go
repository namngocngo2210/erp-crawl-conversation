package read

import (
	"context"

	"github.com/desi/crawl-conversations/internal/entity"
	reporead "github.com/desi/crawl-conversations/internal/repository/read"
)

type UserRepo interface {
	FindByEmail(ctx context.Context, email string) (entity.User, error)
	CreateIfNotExists(ctx context.Context, u entity.User) error
}

type ReadRepo interface {
	ListPages(ctx context.Context) ([]reporead.PageCount, error)
	ListConversations(ctx context.Context, f reporead.ConvFilter) ([]entity.Conversation, error)
	GetConversation(ctx context.Context, id int64) (reporead.ConvDetail, error)
	ListMessages(ctx context.Context, f reporead.MsgFilter) ([]entity.Message, error)
}

type SearchRepo interface {
	SearchMessages(ctx context.Context, f reporead.MsgSearchFilter) ([]entity.Message, error)
	SearchCustomers(ctx context.Context, f reporead.CustomerFilter) ([]entity.Customer, error)
}

type StatsRepo interface {
	Totals(ctx context.Context) (reporead.Totals, error)
	MessagesByDay(ctx context.Context, days int) ([]reporead.DailyCount, error)
	CountsByPage(ctx context.Context) ([]reporead.PageCountRow, error)
	RecentCrawlRuns(ctx context.Context, limit int) ([]entity.CrawlRun, error)
}

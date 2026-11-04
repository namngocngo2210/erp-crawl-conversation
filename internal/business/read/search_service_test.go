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

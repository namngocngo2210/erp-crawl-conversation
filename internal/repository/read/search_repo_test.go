package read

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/desi/crawl-conversations/internal/repository"
)

// seedConversationReturnID inserts a Conversation and returns its generated ID.
func seedConversationReturnID(ctx context.Context, db *repository.DB, pageID int64, pancakeConvID string, updatedAt time.Time) (int64, error) {
	c := entity.Conversation{
		PageID:                pageID,
		PancakeConversationID: pancakeConvID,
		LastMessageAt:         updatedAt,
	}
	if err := db.Gorm.WithContext(ctx).Create(&c).Error; err != nil {
		return 0, fmt.Errorf("seedConversationReturnID create: %w", err)
	}
	return c.ID, nil
}

// seedMessage inserts a Message into the partitioned messages table.
// It calls EnsurePartition first so the correct child partition exists.
func seedMessage(ctx context.Context, db *repository.DB, pageID, convID int64, pancakeMsgID, direction, content string, insertedAt time.Time) error {
	if err := db.EnsurePartition(ctx, insertedAt); err != nil {
		return fmt.Errorf("seedMessage partition: %w", err)
	}
	m := entity.Message{
		PageID:           pageID,
		ConversationID:   convID,
		PancakeMessageID: pancakeMsgID,
		Direction:        direction,
		Content:          content,
		InsertedAt:       insertedAt,
	}
	if err := db.Gorm.WithContext(ctx).Create(&m).Error; err != nil {
		return fmt.Errorf("seedMessage create: %w", err)
	}
	return nil
}

func TestSearchMessages(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	pageID, err := seedPage(ctx, db, "p-search-"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	convID, err := seedConversationReturnID(ctx, db, pageID, "c-search-"+suffix, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := seedMessage(ctx, db, pageID, convID, "m1-"+suffix, "in", "hello pancake world", time.Now().UTC()); err != nil {
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

func TestSearchCustomers(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	pageID, err := seedPage(ctx, db, "p-cust-"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	cust := entity.Customer{
		PageID:            pageID,
		PancakeCustomerID: "cust-" + suffix,
		Name:              "Nguyen Van Alpha",
		Phone:             "0909" + suffix[:6],
		Email:             "alpha" + suffix + "@test.local",
	}
	if err := db.Gorm.WithContext(ctx).Create(&cust).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewSearchRepository(db)
	results, err := repo.SearchCustomers(ctx, CustomerFilter{PageID: pageID, Query: "Alpha", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one customer hit")
	}
}

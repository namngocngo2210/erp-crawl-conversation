package read

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/desi/crawl-conversations/internal/repository"
)

// seedPage inserts a Page and returns its generated ID.
// pancakePageID must be unique across all test runs; callers should embed a
// timestamp or UUID suffix to avoid UNIQUE-constraint collisions.
func seedPage(ctx context.Context, db *repository.DB, pancakePageID string) (int64, error) {
	p := entity.Page{
		PancakePageID: pancakePageID,
		Name:          "Test Page " + pancakePageID,
		Platform:      "facebook",
		Status:        "active",
	}
	if err := db.Gorm.WithContext(ctx).Create(&p).Error; err != nil {
		return 0, fmt.Errorf("seedPage: %w", err)
	}
	return p.ID, nil
}

// seedConversation inserts a Conversation for pageID with the given updatedAt.
// Because GORM auto-manages updated_at, we insert first then UPDATE to pin the
// timestamp.  LastMessageAt is also set to updatedAt (NOT NULL column).
func seedConversation(ctx context.Context, db *repository.DB, pageID int64, pancakeConvID string, updatedAt time.Time) error {
	c := entity.Conversation{
		PageID:                pageID,
		PancakeConversationID: pancakeConvID,
		LastMessageAt:         updatedAt,
	}
	if err := db.Gorm.WithContext(ctx).Create(&c).Error; err != nil {
		return fmt.Errorf("seedConversation create: %w", err)
	}
	// Pin updated_at to the requested value (GORM overwrites it on Create).
	if err := db.Gorm.WithContext(ctx).
		Exec("UPDATE conversations SET updated_at = ? WHERE id = ?", updatedAt, c.ID).Error; err != nil {
		return fmt.Errorf("seedConversation pin updated_at: %w", err)
	}
	return nil
}

func TestListConversationsKeyset(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// Use a time-based suffix so pancake_page_id stays unique across runs.
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	pageID, err := seedPage(ctx, db, "p-keyset-"+suffix)
	if err != nil {
		t.Fatal(err)
	}

	base := time.Now().UTC().Truncate(time.Second)
	for i := 0; i < 3; i++ {
		convID := fmt.Sprintf("c-%d-%s", i, suffix)
		if err := seedConversation(ctx, db, pageID, convID, base.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}

	repo := NewReadRepository(db)

	// Page 1: limit 2, expect newest two (i=2 then i=1)
	first, err := repo.ListConversations(ctx, ConvFilter{PageID: pageID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 {
		t.Fatalf("page 1: want 2 rows, got %d", len(first))
	}
	// Newest first — DESC order
	if !first[0].UpdatedAt.After(first[1].UpdatedAt) {
		t.Fatalf("expected DESC order: first[0].UpdatedAt=%v first[1].UpdatedAt=%v",
			first[0].UpdatedAt, first[1].UpdatedAt)
	}

	// Page 2: cursor from last item of page 1
	next := EncodeCursor(Cursor{Time: first[1].UpdatedAt, ID: first[1].ID})
	page2, err := repo.ListConversations(ctx, ConvFilter{PageID: pageID, Cursor: next, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 1 {
		t.Fatalf("page 2: want 1 remaining row, got %d", len(page2))
	}
}

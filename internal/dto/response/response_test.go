package response

import (
	"testing"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
)

func TestMapConversationsNextCursor(t *testing.T) {
	now := time.Now().UTC()
	convs := []entity.Conversation{
		{ID: 2, UpdatedAt: now, Snippet: "b"},
		{ID: 1, UpdatedAt: now.Add(-time.Minute), Snippet: "a"},
	}
	// limit == len -> has next cursor
	p := MapConversations(convs, 2)
	if p.NextCursor == nil {
		t.Fatal("expected next cursor when full page")
	}
	if len(p.Items) != 2 || p.Items[0].Snippet != "b" {
		t.Fatalf("unexpected items: %+v", p.Items)
	}
	// fewer than limit -> no next cursor
	p2 := MapConversations(convs, 5)
	if p2.NextCursor != nil {
		t.Fatal("expected nil cursor when partial page")
	}
}

func TestMapCustomersCursor(t *testing.T) {
	in := []entity.Customer{{ID: 3}, {ID: 2}}
	p := MapCustomers(in, 2)
	if p.NextCursor == nil {
		t.Fatal("want cursor")
	}
	if len(p.Items) != 2 {
		t.Fatalf("want 2 items, got %d", len(p.Items))
	}
}

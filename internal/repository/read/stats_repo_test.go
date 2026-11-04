package read

import (
	"context"
	"testing"
)

func TestTotals(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	repo := NewStatsRepository(db)
	got, err := repo.Totals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Pages < 0 || got.Conversations < 0 || got.Messages < 0 {
		t.Fatalf("unexpected negative totals: %+v", got)
	}
}

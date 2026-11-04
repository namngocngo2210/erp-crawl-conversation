package read

import (
	"context"
	"os"
	"testing"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/desi/crawl-conversations/internal/repository"
)

func testDB(t *testing.T) *repository.DB {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := repository.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	return db
}

func TestUserCreateAndFind(t *testing.T) {
	db := testDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()
	u := entity.User{Email: "seed@test.local", PasswordHash: "h", Name: "Seed", Role: "admin"}
	if err := repo.CreateIfNotExists(ctx, u); err != nil {
		t.Fatal(err)
	}
	// idempotent
	if err := repo.CreateIfNotExists(ctx, u); err != nil {
		t.Fatal(err)
	}
	got, err := repo.FindByEmail(ctx, "seed@test.local")
	if err != nil {
		t.Fatal(err)
	}
	if got.Role != "admin" {
		t.Fatalf("want admin, got %s", got.Role)
	}
}

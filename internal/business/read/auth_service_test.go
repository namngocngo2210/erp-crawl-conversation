package read

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type fakeUsers struct{ byEmail map[string]entity.User }

func (f *fakeUsers) FindByEmail(_ context.Context, email string) (entity.User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return entity.User{}, gorm.ErrRecordNotFound
	}
	return u, nil
}
func (f *fakeUsers) CreateIfNotExists(_ context.Context, u entity.User) error {
	if _, ok := f.byEmail[u.Email]; !ok {
		f.byEmail[u.Email] = u
	}
	return nil
}

func newFake(email, pw string) *fakeUsers {
	h, _ := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return &fakeUsers{byEmail: map[string]entity.User{
		email: {ID: 1, Email: email, PasswordHash: string(h), Role: "admin"},
	}}
}

func TestLoginAndVerify(t *testing.T) {
	svc := NewAuthService(newFake("a@b.c", "pw123"), "secret", time.Hour)
	tok, u, err := svc.Login(context.Background(), "a@b.c", "pw123")
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "a@b.c" {
		t.Fatalf("unexpected user %+v", u)
	}
	claims, err := svc.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != 1 || claims.Role != "admin" {
		t.Fatalf("unexpected claims %+v", claims)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	svc := NewAuthService(newFake("a@b.c", "pw123"), "secret", time.Hour)
	if _, _, err := svc.Login(context.Background(), "a@b.c", "nope"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestVerifyBadToken(t *testing.T) {
	svc := NewAuthService(newFake("a@b.c", "pw123"), "secret", time.Hour)
	if _, err := svc.Verify("garbage"); err == nil {
		t.Fatal("expected error")
	}
}

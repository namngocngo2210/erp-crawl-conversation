package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	read "github.com/desi/crawl-conversations/internal/business/read"
	"github.com/desi/crawl-conversations/internal/entity"
	reporead "github.com/desi/crawl-conversations/internal/repository/read"
)

// --- fakes ---
type fakeAuth struct{}

func (fakeAuth) Login(_ context.Context, email, password string) (string, entity.User, error) {
	if email == "a@b.c" && password == "pw" {
		return "token123", entity.User{ID: 1, Email: email, Role: "admin"}, nil
	}
	return "", entity.User{}, read.ErrInvalidCredentials
}
func (fakeAuth) Verify(t string) (read.Claims, error) {
	if t == "token123" {
		return read.Claims{UserID: 1, Role: "admin"}, nil
	}
	return read.Claims{}, read.ErrInvalidCredentials
}
func (fakeAuth) SeedAdmin(context.Context, string, string) error { return nil }

type fakeQuery struct{}

func (fakeQuery) Pages(context.Context) ([]reporead.PageCount, error) {
	return []reporead.PageCount{{Page: entity.Page{ID: 1, Name: "P"}, ConversationCount: 3}}, nil
}
func (fakeQuery) Conversations(context.Context, reporead.ConvFilter) ([]entity.Conversation, error) {
	return []entity.Conversation{{ID: 5}}, nil
}
func (fakeQuery) Conversation(context.Context, int64) (reporead.ConvDetail, error) {
	return reporead.ConvDetail{Conversation: entity.Conversation{ID: 5}}, nil
}
func (fakeQuery) Messages(context.Context, reporead.MsgFilter) ([]entity.Message, error) {
	return []entity.Message{{ID: 9, Direction: "in"}}, nil
}

func setup() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r, Deps{Auth: fakeAuth{}, Query: fakeQuery{}, CORSOrigin: "*"})
	return r
}

func TestLoginThenAccess(t *testing.T) {
	r := setup()

	// login
	w := httptest.NewRecorder()
	body := `{"email":"a@b.c","password":"pw"}`
	req, _ := http.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var lr struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &lr)
	if lr.Token != "token123" {
		t.Fatalf("no token: %s", w.Body.String())
	}

	// protected without token -> 401
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/pages", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w2.Code)
	}

	// protected with token -> 200
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/api/v1/pages", nil)
	req3.Header.Set("Authorization", "Bearer token123")
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w3.Code, w3.Body.String())
	}
}

func TestMessagesDirectionParam(t *testing.T) {
	r := setup()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/conversations/5/messages?direction=in&limit=10", nil)
	req.Header.Set("Authorization", "Bearer token123")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
}

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/desi/crawl-conversations/internal/entity"
	reporead "github.com/desi/crawl-conversations/internal/repository/read"
)

type fakeSearchSvc struct{}

func (fakeSearchSvc) Messages(context.Context, reporead.MsgSearchFilter) ([]entity.Message, error) {
	return []entity.Message{{ID: 1, Content: "hi", Direction: "in"}}, nil
}
func (fakeSearchSvc) Customers(context.Context, reporead.CustomerFilter) ([]entity.Customer, error) {
	return []entity.Customer{{ID: 2, Name: "Anna"}}, nil
}

func setupSearch() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r, Deps{Auth: fakeAuth{}, Query: fakeQuery{}, Search: fakeSearchSvc{}, CORSOrigin: "*"})
	return r
}

func TestSearchMessagesEndpoint(t *testing.T) {
	r := setupSearch()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/search/messages?q=hi&direction=in", nil)
	req.Header.Set("Authorization", "Bearer token123")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	r := setupSearch()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/search/messages", nil)
	req.Header.Set("Authorization", "Bearer token123")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	read "github.com/desi/crawl-conversations/internal/business/read"
	"github.com/desi/crawl-conversations/internal/entity"
)

type fakeStatsSvc struct{}

func (fakeStatsSvc) Overview(context.Context, int) (read.Overview, error) {
	return read.Overview{}, nil
}
func (fakeStatsSvc) CrawlRuns(context.Context, int) ([]entity.CrawlRun, error) {
	return []entity.CrawlRun{{ID: 1, Status: "success"}}, nil
}

func setupStats() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r, Deps{Auth: fakeAuth{}, Query: fakeQuery{}, Search: fakeSearchSvc{}, Stats: fakeStatsSvc{}, CORSOrigin: "*"})
	return r
}

func TestOverviewEndpoint(t *testing.T) {
	r := setupStats()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stats/overview", nil)
	req.Header.Set("Authorization", "Bearer token123")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
}

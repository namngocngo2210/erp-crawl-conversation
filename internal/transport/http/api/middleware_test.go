package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	read "github.com/desi/crawl-conversations/internal/business/read"
)

type stubAuth struct{ ok bool }

func (s stubAuth) Verify(token string) (read.Claims, error) {
	if s.ok && token == "good" {
		return read.Claims{UserID: 9, Role: "admin"}, nil
	}
	return read.Claims{}, http.ErrNoCookie
}

func TestAuthMiddlewareRejectsMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(stubAuth{ok: true}))
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/x", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestAuthMiddlewareAcceptsBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(stubAuth{ok: true}))
	r.GET("/x", func(c *gin.Context) {
		cl, ok := ClaimsFrom(c)
		if !ok || cl.UserID != 9 {
			t.Errorf("claims not set")
		}
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer good")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

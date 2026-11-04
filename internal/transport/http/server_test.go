package http

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	api "github.com/desi/crawl-conversations/internal/transport/http/api"
)

func TestHealthz(t *testing.T) {
	srv := NewServer(":0", slog.Default(), func() bool { return true }, api.Deps{})
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestMetricsExposed(t *testing.T) {
	srv := NewServer(":0", slog.Default(), func() bool { return true }, api.Deps{})
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestManualTrigger(t *testing.T) {
	srv := NewServer(":0", slog.Default(), func() bool { return true }, api.Deps{})
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/crawl", nil))
	require.Equal(t, http.StatusAccepted, rec.Code)

	busy := NewServer(":0", slog.Default(), func() bool { return false }, api.Deps{})
	rec2 := httptest.NewRecorder()
	busy.Handler.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/crawl", nil))
	require.Equal(t, http.StatusConflict, rec2.Code)
}

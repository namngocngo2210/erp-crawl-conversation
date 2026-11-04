package http

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	api "github.com/desi/crawl-conversations/internal/transport/http/api"
)

func NewServer(addr string, log *slog.Logger, trigger func() bool, apiDeps api.Deps) *http.Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.POST("/crawl", func(c *gin.Context) {
		if trigger() {
			c.JSON(http.StatusAccepted, gin.H{"status": "started"})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"status": "already running"})
	})
	api.Register(r, apiDeps)
	return &http.Server{Addr: addr, Handler: r}
}

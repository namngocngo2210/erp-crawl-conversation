package api

import (
	"github.com/gin-gonic/gin"

	read "github.com/desi/crawl-conversations/internal/business/read"
)

type Deps struct {
	Auth       read.AuthService
	Query      read.QueryService
	Search     read.SearchService
	Stats      read.StatsService
	CORSOrigin string
}

func Register(r *gin.Engine, d Deps) {
	v1 := r.Group("/api/v1")
	v1.Use(CORS(d.CORSOrigin))

	v1.POST("/auth/login", handleLogin(d.Auth))

	auth := v1.Group("")
	auth.Use(AuthMiddleware(d.Auth))
	auth.GET("/auth/me", handleMe())
	auth.GET("/pages", handlePages(d.Query))
	auth.GET("/conversations", handleConversations(d.Query))
	auth.GET("/conversations/:id", handleConversationDetail(d.Query))
	auth.GET("/conversations/:id/messages", handleMessages(d.Query))
	auth.GET("/search/messages", handleSearchMessages(d.Search))
	auth.GET("/customers", handleCustomers(d.Search))
	auth.GET("/stats/overview", handleOverview(d.Stats))
	auth.GET("/stats/crawl-runs", handleCrawlRuns(d.Stats))
}

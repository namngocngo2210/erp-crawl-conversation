package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	read "github.com/desi/crawl-conversations/internal/business/read"
	"github.com/desi/crawl-conversations/internal/dto/response"
)

func handleOverview(s read.StatsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		days, _ := strconv.Atoi(c.Query("days"))
		ov, err := s.Overview(c.Request.Context(), days)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": "overview failed"}})
			return
		}
		c.JSON(http.StatusOK, response.MapOverview(ov.Totals, ov.ByDay, ov.ByPage))
	}
}

func handleCrawlRuns(s read.StatsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.Query("limit"))
		runs, err := s.CrawlRuns(c.Request.Context(), limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": "crawl runs failed"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": response.MapCrawlRuns(runs)})
	}
}

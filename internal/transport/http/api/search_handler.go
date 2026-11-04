package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	read "github.com/desi/crawl-conversations/internal/business/read"
	"github.com/desi/crawl-conversations/internal/dto/response"
	reporead "github.com/desi/crawl-conversations/internal/repository/read"
)

func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

func parseInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func handleSearchMessages(s read.SearchService) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := c.Query("q")
		if q == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "q is required"}})
			return
		}
		limit := parseLimit(c.Query("limit"))
		msgs, err := s.Messages(c.Request.Context(), reporead.MsgSearchFilter{
			Query: q, PageID: parseInt64(c.Query("page_id")), Direction: c.Query("direction"),
			From: parseTime(c.Query("from")), To: parseTime(c.Query("to")),
			Cursor: c.Query("cursor"), Limit: limit,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": "search failed"}})
			return
		}
		c.JSON(http.StatusOK, response.MapMessages(msgs, limit))
	}
}

func handleCustomers(s read.SearchService) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := parseLimit(c.Query("limit"))
		custs, err := s.Customers(c.Request.Context(), reporead.CustomerFilter{
			PageID: parseInt64(c.Query("page_id")), Query: c.Query("q"),
			Cursor: c.Query("cursor"), Limit: limit,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": "list customers failed"}})
			return
		}
		c.JSON(http.StatusOK, response.MapCustomers(custs, limit))
	}
}

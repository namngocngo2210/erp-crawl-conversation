package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	read "github.com/desi/crawl-conversations/internal/business/read"
	"github.com/desi/crawl-conversations/internal/dto/response"
	reporead "github.com/desi/crawl-conversations/internal/repository/read"
)

func parseLimit(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 50
	}
	if n > 100 {
		return 100
	}
	return n
}

func handlePages(q read.QueryService) gin.HandlerFunc {
	return func(c *gin.Context) {
		pages, err := q.Pages(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": "list pages failed"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": response.MapPages(pages)})
	}
}

func handleConversations(q read.QueryService) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID, err := strconv.ParseInt(c.Query("page_id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "page_id required"}})
			return
		}
		limit := parseLimit(c.Query("limit"))
		convs, err := q.Conversations(c.Request.Context(), reporead.ConvFilter{
			PageID: pageID, Cursor: c.Query("cursor"), Limit: limit,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": "list conversations failed"}})
			return
		}
		c.JSON(http.StatusOK, response.MapConversations(convs, limit))
	}
}

func handleConversationDetail(q read.QueryService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "invalid id"}})
			return
		}
		detail, err := q.Conversation(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "conversation not found"}})
			return
		}
		c.JSON(http.StatusOK, response.MapConversationDetail(detail))
	}
}

func handleMessages(q read.QueryService) gin.HandlerFunc {
	return func(c *gin.Context) {
		convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "invalid id"}})
			return
		}
		limit := parseLimit(c.Query("limit"))
		msgs, err := q.Messages(c.Request.Context(), reporead.MsgFilter{
			ConversationID: convID, Direction: c.Query("direction"),
			Cursor: c.Query("cursor"), Limit: limit,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": "list messages failed"}})
			return
		}
		c.JSON(http.StatusOK, response.MapMessages(msgs, limit))
	}
}

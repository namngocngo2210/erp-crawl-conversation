package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	read "github.com/desi/crawl-conversations/internal/business/read"
)

type tokenVerifier interface {
	Verify(token string) (read.Claims, error)
}

const claimsKey = "claims"

func AuthMiddleware(v tokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(h, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthorized", "message": "missing bearer token"}})
			return
		}
		claims, err := v.Verify(strings.TrimPrefix(h, prefix))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthorized", "message": "invalid token"}})
			return
		}
		c.Set(claimsKey, claims)
		c.Next()
	}
}

func ClaimsFrom(c *gin.Context) (read.Claims, bool) {
	v, ok := c.Get(claimsKey)
	if !ok {
		return read.Claims{}, false
	}
	cl, ok := v.(read.Claims)
	return cl, ok
}

func CORS(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	read "github.com/desi/crawl-conversations/internal/business/read"
)

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func handleLogin(auth read.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "invalid body"}})
			return
		}
		tok, u, err := auth.Login(c.Request.Context(), req.Email, req.Password)
		if err != nil {
			if errors.Is(err, read.ErrInvalidCredentials) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthorized", "message": "invalid credentials"}})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal", "message": "login failed"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"token": tok,
			"user":  gin.H{"id": u.ID, "email": u.Email, "name": u.Name, "role": u.Role},
		})
	}
}

func handleMe() gin.HandlerFunc {
	return func(c *gin.Context) {
		cl, ok := ClaimsFrom(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthorized", "message": "no claims"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": cl.UserID, "email": cl.Email, "role": cl.Role})
	}
}

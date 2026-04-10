package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yubrajnag/taskflow/backend/internal/auth"
)

const claimsKey = "claims"

func AuthMiddleware(tokens *auth.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			return
		}

		claims, err := tokens.Verify(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set(claimsKey, claims)
		c.Next()
	}
}

func GetUserID(c *gin.Context) uuid.UUID {
	return getClaims(c).UserID
}

func getClaims(c *gin.Context) *auth.Claims {
	v, exists := c.Get(claimsKey)
	if !exists {
		panic("getClaims called without AuthMiddleware")
	}
	return v.(*auth.Claims)
}

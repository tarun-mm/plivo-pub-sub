package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware creates a Gin middleware for X-API-Key authentication
func AuthMiddleware(validator *APIKeyValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		// If auth is disabled, allow all requests
		if !validator.IsEnabled() {
			c.Next()
			return
		}

		// Extract API key from X-API-Key header
		apiKey := c.GetHeader("X-API-Key")

		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    ErrCodeMissingAPIKey,
					"message": ErrMsgMissingAPIKey,
				},
			})
			c.Abort()
			return
		}

		// Validate the API key
		if !validator.ValidateKey(apiKey) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    ErrCodeInvalidAPIKey,
					"message": ErrMsgInvalidAPIKey,
				},
			})
			c.Abort()
			return
		}

		// Key is valid, proceed
		c.Next()
	}
}

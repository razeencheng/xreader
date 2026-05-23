package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireCSRF rejects state-mutating requests that lack the X-Requested-With
// header. The frontend already sends "X-Requested-With: XMLHttpRequest" on
// every fetch call, so legitimate requests pass through transparently. Because
// browsers block cross-origin custom headers by default, this prevents simple
// CSRF attacks without requiring a token.
func RequireCSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}

		if c.GetHeader("X-Requested-With") == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "missing X-Requested-With header"})
			return
		}
		c.Next()
	}
}

package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds common security headers to every response.
//
// When gaEnabled is true the Content-Security-Policy is widened to permit the
// Google Analytics gtag.js script and its measurement beacons; otherwise the
// policy stays locked to same-origin so self-hosted instances phone home to
// no one.
func SecurityHeaders(gaEnabled bool) gin.HandlerFunc {
	scriptSrc := "'self' 'unsafe-inline'"
	connectSrc := "'self'"
	if gaEnabled {
		scriptSrc += " https://www.googletagmanager.com"
		connectSrc += " https://www.googletagmanager.com https://www.google-analytics.com https://*.google-analytics.com https://*.analytics.google.com"
	}

	csp := strings.Join([]string{
		"default-src 'self'",
		"script-src " + scriptSrc,
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' https: data:",
		"connect-src " + connectSrc,
		"font-src 'self' https://fonts.gstatic.com",
		"frame-ancestors 'none'",
	}, "; ")

	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Content-Security-Policy", csp)
		c.Next()
	}
}

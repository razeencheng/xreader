package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetMe(c *gin.Context) {
	u, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *Handler) Logout(c *gin.Context) {
	cookie, err := c.Cookie("xreader_session")
	if err == nil && cookie != "" {
		_ = h.SessionStore.Delete(c.Request.Context(), cookie)
	}
	c.SetCookie("xreader_session", "", -1, "/", "", h.isSecureCookie(c), true)
	c.JSON(http.StatusOK, gin.H{"status": "logged out"})
}

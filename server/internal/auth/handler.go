package auth

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	Service      *Service
	SessionStore SessionStore
	secureCookie bool
}

func NewHandler(svc *Service, sessions SessionStore) *Handler {
	secure := os.Getenv("COOKIE_SECURE") == "true"
	return &Handler{Service: svc, SessionStore: sessions, secureCookie: secure}
}

func (h *Handler) isSecureCookie(c *gin.Context) bool {
	if h.secureCookie {
		return true
	}
	return c.Request.TLS != nil
}

func (h *Handler) BeginLogin(c *gin.Context) {
	redirectURL, state, err := h.Service.BeginLogin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start login"})
		return
	}
	// Set the CSRF state as an HttpOnly cookie so HandleCallback can verify it.
	// Must be SameSite=Lax (not Strict): GitHub's OAuth redirect back to this
	// app is a cross-site top-level navigation, and Strict would drop the
	// cookie, breaking state verification. Lax still blocks CSRF here.
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("xreader_oauth_state", state, 600, "/", "", h.isSecureCookie(c), true)
	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

func (h *Handler) HandleCallback(c *gin.Context) {
	stateParam := c.Query("state")
	code := c.Query("code")
	if stateParam == "" || code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing state or code"})
		return
	}

	cookieValue, err := c.Cookie("xreader_oauth_state")
	if err != nil || cookieValue == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing state cookie"})
		return
	}

	result, err := h.Service.Callback(c.Request.Context(), stateParam, cookieValue, code, c.GetHeader("User-Agent"))

	// Clear the one-time state cookie regardless of outcome.
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("xreader_oauth_state", "", -1, "/", "", h.isSecureCookie(c), true)

	if err != nil {
		switch err {
		case ErrInvalidState:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid state"})
		case ErrNotAllowlisted:
			c.JSON(http.StatusForbidden, gin.H{"error": "not on allowlist"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		}
		return
	}

	c.SetCookie("xreader_session", result.SessionID, 30*24*3600, "/", "", h.isSecureCookie(c), true)
	c.Redirect(http.StatusTemporaryRedirect, "/")
}

package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/internal/auth"
	"github.com/razeencheng/xreader/internal/guest"
)

type User struct {
	ID             int64      `json:"id"`
	GitHubID       *int64     `json:"github_id,omitempty"`
	GitHubUsername string     `json:"github_username"`
	AvatarURL      string     `json:"avatar_url,omitempty"`
	NativeLanguage string     `json:"native_language"`
	Role           string     `json:"role"`
	DensityPref    string     `json:"density_pref"`
	ThemePref      string     `json:"theme_pref"`
	ExpiresAt      *time.Time `json:"-"`
}

func loadUser(c *gin.Context, pool *pgxpool.Pool, userID int64) (*User, error) {
	var u User
	var avatarURL *string
	var githubID *int64
	var expiresAt *time.Time
	err := pool.QueryRow(c.Request.Context(),
		`SELECT id, github_id, github_username, avatar_url, native_language, role, density_pref, theme_pref, expires_at
		 FROM users WHERE id = $1`, userID,
	).Scan(&u.ID, &githubID, &u.GitHubUsername, &avatarURL, &u.NativeLanguage, &u.Role, &u.DensityPref, &u.ThemePref, &expiresAt)
	if err != nil {
		return nil, err
	}
	u.GitHubID = githubID
	u.ExpiresAt = expiresAt
	if avatarURL != nil {
		u.AvatarURL = *avatarURL
	}
	return &u, nil
}

func RequireAuth(sessions auth.SessionStore, pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie("xreader_session")
		if err != nil || cookie == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		userID, err := sessions.Get(c.Request.Context(), cookie)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
			return
		}

		user, err := loadUser(c, pool, userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}

		_ = sessions.Touch(c.Request.Context(), cookie)
		c.Set("user", user)
		c.Next()
	}
}

func OptionalAuth(sessions auth.SessionStore, pool *pgxpool.Pool, guestSvc *guest.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie("xreader_session")
		if err == nil && cookie != "" {
			userID, err := sessions.Get(c.Request.Context(), cookie)
			if err == nil {
				user, err := loadUser(c, pool, userID)
				if err == nil {
					// Check guest expiry
					if user.Role == "guest" && user.ExpiresAt != nil && time.Now().After(*user.ExpiresAt) {
						_ = sessions.Delete(c.Request.Context(), cookie)
						c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "guest session expired"})
						return
					}
					// Check guest mode still enabled for guest users
					if user.Role == "guest" {
						enabled, _ := guestSvc.IsEnabled(c.Request.Context())
						if !enabled {
							c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "guest mode disabled"})
							return
						}
					}
					_ = sessions.Touch(c.Request.Context(), cookie)
					c.Set("user", user)
					c.Next()
					return
				}
			}
		}

		// No valid session — try creating guest
		enabled, _ := guestSvc.IsEnabled(c.Request.Context())
		if !enabled {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		guestUser, err := guestSvc.CreateGuest(c.Request.Context())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to create guest"})
			return
		}

		sessionID, err := sessions.Create(c.Request.Context(), guestUser.ID, c.Request.UserAgent())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
			return
		}

		secure := c.Request.TLS != nil
		c.SetCookie("xreader_session", sessionID, 86400, "/", "", secure, true)

		user, err := loadUser(c, pool, guestUser.ID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to load guest"})
			return
		}
		c.Set("user", user)
		c.Next()
	}
}

func GuestReadOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetUser(c)
		if user != nil && user.Role == "guest" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "guests cannot modify this resource"})
			return
		}
		c.Next()
	}
}

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}
		user := u.(*User)
		if user.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin required"})
			return
		}
		c.Next()
	}
}

func GetUser(c *gin.Context) *User {
	u, exists := c.Get("user")
	if !exists {
		return nil
	}
	return u.(*User)
}

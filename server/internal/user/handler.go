package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/db/gen"
	"github.com/razeencheng/xreader/internal/middleware"
)

type Handler struct {
	queries *gen.Queries
}

func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{queries: gen.New(pool)}
}

type updateMeRequest struct {
	NativeLanguage *string `json:"native_language"`
	DensityPref    *string `json:"density_pref"`
	ThemePref      *string `json:"theme_pref"`
}

var (
	allowedNativeLanguages = map[string]struct{}{
		"zh-CN": {},
		"zh-TW": {},
		"en-US": {},
		"ja-JP": {},
		"ko-KR": {},
		"es-ES": {},
		"fr-FR": {},
		"de-DE": {},
		"pt-PT": {},
	}
	allowedDensityPrefs = map[string]struct{}{
		"comfortable": {},
		"compact":     {},
	}
	allowedThemePrefs = map[string]struct{}{
		"light":  {},
		"dark":   {},
		"system": {},
	}
)

func (h *Handler) GetMe(c *gin.Context) {
	u := middleware.GetUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	me, err := h.queries.GetUserByID(c.Request.Context(), u.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, me)
}

func (h *Handler) UpdateMe(c *gin.Context) {
	u := middleware.GetUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	var req updateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	lang := ""
	if req.NativeLanguage != nil {
		if !isAllowed(*req.NativeLanguage, allowedNativeLanguages) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid native_language"})
			return
		}
		lang = *req.NativeLanguage
	}

	density := ""
	if req.DensityPref != nil {
		if !isAllowed(*req.DensityPref, allowedDensityPrefs) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid density_pref"})
			return
		}
		density = *req.DensityPref
	}

	theme := ""
	if req.ThemePref != nil {
		if !isAllowed(*req.ThemePref, allowedThemePrefs) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid theme_pref"})
			return
		}
		theme = *req.ThemePref
	}

	updated, err := h.queries.UpdateUserSettings(c.Request.Context(), gen.UpdateUserSettingsParams{
		ID:      u.ID,
		Column2: lang,
		Column3: density,
		Column4: theme,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update settings"})
		return
	}

	c.JSON(http.StatusOK, updated)
}

func isAllowed(value string, allowed map[string]struct{}) bool {
	_, ok := allowed[value]
	return ok
}

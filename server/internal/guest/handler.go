package guest

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Status(c *gin.Context) {
	enabled, _ := h.svc.IsEnabled(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"enabled": enabled})
}

func (h *Handler) GetSettings(c *gin.Context) {
	enabled, _ := h.svc.IsEnabled(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"enabled": enabled})
}

func (h *Handler) UpdateSettings(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.svc.SetEnabled(c.Request.Context(), req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update setting"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"enabled": req.Enabled})
}

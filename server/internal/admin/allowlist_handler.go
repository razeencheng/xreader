package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/razeencheng/xreader/internal/middleware"
)

type AllowlistHandler struct {
	Service *AllowlistService
}

func NewAllowlistHandler(svc *AllowlistService) *AllowlistHandler {
	return &AllowlistHandler{Service: svc}
}

func (h *AllowlistHandler) List(c *gin.Context) {
	entries, err := h.Service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list allowlist"})
		return
	}
	if entries == nil {
		entries = []AllowlistEntry{}
	}
	c.JSON(http.StatusOK, entries)
}

type addAllowlistRequest struct {
	GithubUsername string `json:"github_username" binding:"required"`
	Note           string `json:"note"`
}

func (h *AllowlistHandler) Add(c *gin.Context) {
	var req addAllowlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "github_username required"})
		return
	}

	user := middleware.GetUser(c)
	var addedBy *int64
	if user != nil {
		addedBy = &user.ID
	}

	if err := h.Service.Add(c.Request.Context(), req.GithubUsername, addedBy, req.Note); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "added"})
}

func (h *AllowlistHandler) Remove(c *gin.Context) {
	username := c.Param("username")
	if err := h.Service.Remove(c.Request.Context(), username); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "removed"})
}

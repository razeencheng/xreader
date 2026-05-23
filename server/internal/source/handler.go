package source

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/razeencheng/xreader/internal/middleware"
)

type SourceHandler struct {
	Service        *SourceService
	JobStore       JobStore
	ContentOwnerID func(ctx context.Context) (int64, error)
}

func NewSourceHandler(svc *SourceService, jobStore JobStore) *SourceHandler {
	if jobStore == nil {
		jobStore = NewMemoryJobStore()
	}
	return &SourceHandler{Service: svc, JobStore: jobStore}
}

func (h *SourceHandler) List(c *gin.Context) {
	user := middleware.GetUser(c)
	if user.Role == "guest" && h.ContentOwnerID != nil {
		if adminID, err := h.ContentOwnerID(c.Request.Context()); err == nil {
			// Fetch sources owned by the admin but compute unread counts against
			// the guest's own article_states so guests see their read progress.
			sources, err := h.Service.GuestList(c.Request.Context(), adminID, user.ID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list sources"})
				return
			}
			c.JSON(http.StatusOK, sources)
			return
		}
	}
	sources, err := h.Service.List(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list sources"})
		return
	}
	c.JSON(http.StatusOK, sources)
}

type createSourceRequest struct {
	URL      string `json:"url" binding:"required"`
	Category string `json:"category"`
}

func (h *SourceHandler) Create(c *gin.Context) {
	var req createSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url required"})
		return
	}

	user := middleware.GetUser(c)
	src, err := h.Service.Create(c.Request.Context(), user.ID, req.URL, req.Category)
	if err != nil {
		if isUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "source already exists"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, src)
}

type renameSourceRequest struct {
	Title string `json:"title" binding:"required"`
}

func (h *SourceHandler) Rename(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req renameSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title required"})
		return
	}

	user := middleware.GetUser(c)
	if err := h.Service.Rename(c.Request.Context(), user.ID, id, req.Title); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "renamed"})
}

type updateCategoryRequest struct {
	Category string `json:"category" binding:"required"`
}

func (h *SourceHandler) UpdateCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req updateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category required"})
		return
	}

	user := middleware.GetUser(c)
	if err := h.Service.UpdateCategory(c.Request.Context(), user.ID, id, req.Category); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "category updated"})
}

func (h *SourceHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	user := middleware.GetUser(c)
	if err := h.Service.Delete(c.Request.Context(), user.ID, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *SourceHandler) Refresh(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	user := middleware.GetUser(c)
	inserted, err := h.Service.Refresh(c.Request.Context(), user.ID, id)
	if err != nil {
		if errors.Is(err, ErrSourceNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "refresh completed", "inserted": inserted})
}

func (h *SourceHandler) ImportOPML(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	opml, err := ParseOPML(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid OPML"})
		return
	}
	feeds := FlattenOPML(opml)
	if len(feeds) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no feeds found in OPML"})
		return
	}

	jobID := fmt.Sprintf("import-%d-%d", user.ID, time.Now().UnixNano())
	h.Service.ImportOPML(c.Request.Context(), user.ID, feeds, jobID, h.JobStore)

	c.JSON(http.StatusAccepted, gin.H{"job_id": jobID})
}

func (h *SourceHandler) GetJob(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	jobID := c.Param("jobID")
	if !jobBelongsToUser(jobID, user.ID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	status, ok := h.JobStore.Get(jobID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	c.JSON(http.StatusOK, status)
}

func jobBelongsToUser(jobID string, userID int64) bool {
	parts := strings.Split(jobID, "-")
	if len(parts) != 3 || parts[0] != "import" {
		return false
	}

	ownerID, err := strconv.ParseInt(parts[1], 10, 64)
	return err == nil && ownerID == userID
}

func (h *SourceHandler) ExportOPML(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	ownerID := user.ID
	if user.Role == "guest" && h.ContentOwnerID != nil {
		if id, err := h.ContentOwnerID(c.Request.Context()); err == nil {
			ownerID = id
		}
	}
	data, err := h.Service.ExportOPML(c.Request.Context(), ownerID, "xReader Export")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate OPML"})
		return
	}

	c.Data(http.StatusOK, "text/x-opml", data)
}

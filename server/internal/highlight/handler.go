package highlight

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/razeencheng/xreader/db/gen"
	"github.com/razeencheng/xreader/internal/middleware"
)

type HighlightHandler struct {
	Service        *HighlightService
	ContentOwnerID func(ctx context.Context) (int64, error)
}

func NewHighlightHandler(svc *HighlightService) *HighlightHandler {
	return &HighlightHandler{Service: svc}
}

type createHighlightRequest struct {
	ArticleID       int64   `json:"article_id"`
	Layer           string  `json:"layer"`
	ParagraphIndex  int32   `json:"paragraph_index"`
	TextStartOffset int32   `json:"text_start_offset"`
	TextEndOffset   int32   `json:"text_end_offset"`
	QuotedText      string  `json:"quoted_text"`
	Note            *string `json:"note,omitempty"`
	TargetLanguage  string  `json:"target_language,omitempty"`
}

type updateHighlightNoteRequest struct {
	Note string `json:"note"`
}

type highlightResponse struct {
	ID              int64   `json:"id"`
	UserID          int64   `json:"user_id"`
	ArticleID       int64   `json:"article_id"`
	Layer           string  `json:"layer"`
	ParagraphIndex  int32   `json:"paragraph_index"`
	TextStartOffset int32   `json:"text_start_offset"`
	TextEndOffset   int32   `json:"text_end_offset"`
	QuotedText      string  `json:"quoted_text"`
	Note            *string `json:"note,omitempty"`
	Color           string  `json:"color"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type highlightWithArticleResponse struct {
	highlightResponse
	ArticleTitle string `json:"article_title"`
	ArticleLink  string `json:"article_link"`
}

func (h *HighlightHandler) Create(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	var req createHighlightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	contentOwnerID := user.ID
	if user.Role == "guest" && h.ContentOwnerID != nil {
		if id, err := h.ContentOwnerID(c.Request.Context()); err == nil {
			contentOwnerID = id
		}
	}
	created, err := h.Service.Create(c.Request.Context(), user.ID, contentOwnerID, CreateParams{
		ArticleID:       req.ArticleID,
		Layer:           strings.TrimSpace(req.Layer),
		ParagraphIndex:  req.ParagraphIndex,
		TextStartOffset: req.TextStartOffset,
		TextEndOffset:   req.TextEndOffset,
		QuotedText:      req.QuotedText,
		Note:            req.Note,
		TargetLanguage:  strings.TrimSpace(req.TargetLanguage),
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, toHighlightResponse(*created))
}

func (h *HighlightHandler) ListByArticle(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	articleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	items, err := h.Service.ListByArticle(c.Request.Context(), user.ID, articleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list highlights"})
		return
	}

	resp := make([]highlightResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, toHighlightResponse(item))
	}
	c.JSON(http.StatusOK, gin.H{"items": resp})
}

func (h *HighlightHandler) ListByUser(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	limit := parseLimit(c.DefaultQuery("limit", "50"))
	offset := parseOffset(c.DefaultQuery("offset", "0"))
	query := strings.TrimSpace(c.Query("q"))

	if query != "" {
		items, err := h.Service.Search(c.Request.Context(), user.ID, query, limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list highlights"})
			return
		}
		resp := make([]highlightWithArticleResponse, 0, len(items))
		for _, item := range items {
			resp = append(resp, highlightWithArticleResponse{
				highlightResponse: highlightResponse{
					ID:              item.ID,
					UserID:          item.UserID,
					ArticleID:       item.ArticleID,
					Layer:           item.Layer,
					ParagraphIndex:  item.ParagraphIndex,
					TextStartOffset: item.TextStartOffset,
					TextEndOffset:   item.TextEndOffset,
					QuotedText:      item.QuotedText,
					Note:            textFromValue(item.Note),
					Color:           item.Color,
					CreatedAt:       item.CreatedAt.Time.UTC().Format(time.RFC3339Nano),
					UpdatedAt:       item.UpdatedAt.Time.UTC().Format(time.RFC3339Nano),
				},
				ArticleTitle: item.ArticleTitle,
				ArticleLink:  item.ArticleLink,
			})
		}
		c.JSON(http.StatusOK, gin.H{"items": resp})
		return
	}

	items, err := h.Service.ListByUser(c.Request.Context(), user.ID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list highlights"})
		return
	}

	resp := make([]highlightWithArticleResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, highlightWithArticleResponse{
			highlightResponse: highlightResponse{
				ID:              item.ID,
				UserID:          item.UserID,
				ArticleID:       item.ArticleID,
				Layer:           item.Layer,
				ParagraphIndex:  item.ParagraphIndex,
				TextStartOffset: item.TextStartOffset,
				TextEndOffset:   item.TextEndOffset,
				QuotedText:      item.QuotedText,
				Note:            textFromValue(item.Note),
				Color:           item.Color,
				CreatedAt:       item.CreatedAt.Time.UTC().Format(time.RFC3339Nano),
				UpdatedAt:       item.UpdatedAt.Time.UTC().Format(time.RFC3339Nano),
			},
			ArticleTitle: item.ArticleTitle,
			ArticleLink:  item.ArticleLink,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": resp})
}

func (h *HighlightHandler) UpdateNote(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	highlightID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req updateHighlightNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	if err := h.Service.UpdateNote(c.Request.Context(), user.ID, highlightID, req.Note); err != nil {
		if err == errNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "highlight not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update highlight"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *HighlightHandler) Delete(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	highlightID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.Service.Delete(c.Request.Context(), user.ID, highlightID); err != nil {
		if err == errNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "highlight not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete highlight"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func parseLimit(raw string) int32 {
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 50
	}
	return clampLimit(int32(v))
}

func parseOffset(raw string) int32 {
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || v < 0 {
		return 0
	}
	return int32(v)
}

func toHighlightResponse(item gen.Highlight) highlightResponse {
	resp := highlightResponse{
		ID:              item.ID,
		UserID:          item.UserID,
		ArticleID:       item.ArticleID,
		Layer:           item.Layer,
		ParagraphIndex:  item.ParagraphIndex,
		TextStartOffset: item.TextStartOffset,
		TextEndOffset:   item.TextEndOffset,
		QuotedText:      item.QuotedText,
		Note:            textFromValue(item.Note),
		Color:           item.Color,
		CreatedAt:       item.CreatedAt.Time.UTC().Format(time.RFC3339Nano),
		UpdatedAt:       item.UpdatedAt.Time.UTC().Format(time.RFC3339Nano),
	}
	return resp
}

func textFromValue(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	value := v
	return &value
}


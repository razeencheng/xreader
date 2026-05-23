package article

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/razeencheng/xreader/db/gen"
	"github.com/razeencheng/xreader/internal/ai"
	"github.com/razeencheng/xreader/internal/middleware"
)

type ArticleHandler struct {
	Service          *ArticleService
	ContentOwnerID   func(ctx context.Context) (int64, error)
	RetranslateQueue *ai.RetranslateQueue
}

func NewArticleHandler(svc *ArticleService) *ArticleHandler {
	return &ArticleHandler{Service: svc}
}

func (h *ArticleHandler) resolveOwners(c *gin.Context) (stateOwnerID, contentOwnerID int64, isGuest bool) {
	user := middleware.GetUser(c)
	if user.Role == "guest" {
		if h.ContentOwnerID != nil {
			adminID, err := h.ContentOwnerID(c.Request.Context())
			if err == nil {
				return user.ID, adminID, true
			}
		}
	}
	return user.ID, user.ID, false
}

type articleListResponse struct {
	Items      []articleResponse  `json:"items"`
	NextCursor string             `json:"next_cursor,omitempty"`
	Counts     *articleReadCounts `json:"counts,omitempty"`
}

type articleDetailResponse struct {
	articleResponse
	IsRead          bool            `json:"is_read"`
	IsStarred       bool            `json:"is_starred"`
	ReadingProgress json.RawMessage `json:"reading_progress,omitempty"`
}

type articleResponse struct {
	ID              int64   `json:"id"`
	SourceID        int64   `json:"source_id"`
	Title           string  `json:"title"`
	TitleTranslated string  `json:"title_translated,omitempty"`
	Summary         string  `json:"summary,omitempty"`
	SourceTitle     string  `json:"source_title,omitempty"`
	Link            string  `json:"link"`
	Language        string  `json:"language"`
	Author          *string `json:"author,omitempty"`
	PublishedAt     *string `json:"published_at,omitempty"`
	ContentHtml     string  `json:"content_html,omitempty"`
	ContentText     string  `json:"content_text,omitempty"`
	WordCount       int     `json:"word_count,omitempty"`
	IsRead          bool    `json:"is_read"`
	IsStarred       bool    `json:"is_starred"`
}

type articleReadCounts struct {
	Unread int64 `json:"unread"`
	All    int64 `json:"all"`
	Read   int64 `json:"read"`
}

type articleChangeResponse struct {
	ArticleID int64  `json:"article_id"`
	ChangedAt string `json:"changed_at"`
	IsRead    bool   `json:"is_read"`
	IsStarred bool   `json:"is_starred"`
}

func toArticleChangeResponse(row gen.ListStateChangesSinceRow) articleChangeResponse {
	return articleChangeResponse{
		ArticleID: row.ArticleID,
		ChangedAt: row.ChangedAt.Time.UTC().Format(time.RFC3339Nano),
		IsRead:    row.IsRead,
		IsStarred: row.IsStarred,
	}
}

type originalContentResponse struct {
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	ContentHtml string `json:"content_html"`
	ContentText string `json:"content_text"`
}

type articleStateRequest struct {
	IsRead    *bool `json:"is_read"`
	IsStarred *bool `json:"is_starred"`
}

type batchStateRequest struct {
	Scope  string `json:"scope"`
	IsRead bool   `json:"is_read"`
}

type batchStateResponse struct {
	Status     string  `json:"status"`
	Updated    int     `json:"updated"`
	ArticleIDs []int64 `json:"article_ids"`
}

func (h *ArticleHandler) List(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	ctx := c.Request.Context()
	tab := c.DefaultQuery("tab", "today")
	filter := c.DefaultQuery("filter", "")
	q := c.Query("q")
	sourceIDRaw := c.Query("source_id")
	cursorRaw := c.Query("cursor")
	limit := parseLimit(c.DefaultQuery("limit", "50"))

	stateOwnerID, contentOwnerID, isGuest := h.resolveOwners(c)

	items, err := h.itemsForList(ctx, stateOwnerID, contentOwnerID, isGuest, user.NativeLanguage, tab, filter, q, sourceIDRaw, cursorRaw, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Only the non-search list paths go through the enriched query that
	// populates TitleTranslated. The search branch uses a non-enriched query
	// that never sets it, so every (even already-translated) search hit would
	// look "missing" and be re-enqueued on every keystroke. Gate enqueue to
	// the non-search paths.
	if strings.TrimSpace(q) == "" {
		h.enqueueMissingTitleTranslations(items, isGuest, user.NativeLanguage)
	}

	resp := articleListResponse{Items: items}
	if counts, err := h.countsForList(ctx, stateOwnerID, contentOwnerID, isGuest, tab, q, sourceIDRaw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	} else if counts != nil {
		resp.Counts = counts
	}
	if tab == "stream" && len(items) == int(clampListLimit(limit)) {
		if last := items[len(items)-1]; last.PublishedAt != nil {
			resp.NextCursor = *last.PublishedAt
		}
	}
	c.JSON(http.StatusOK, resp)
}

// enqueueMissingTitleTranslations submits on-demand AI title-translation jobs
// for the articles currently visible on this page whose title has no stored
// translation for the requesting user's native language (either never
// attempted, or a prior combined AI call returned an empty title). It is
// bounded to the
// page (items is already limited), de-duped/rate-limited by RetranslateQueue,
// and never blocks the response. Guests are excluded (read-only demo over the
// content owner's data). Polluted rows (title_translated != "") are left alone
// — historical backfill is intentionally out of scope.
//
// Note on repeated polling: after Complete the de-dup reservation clears, so a
// still-untranslated article CAN be re-enqueued by the next list poll. That is
// acceptable by design — the real AI-load cap is the consumer side: a bounded
// (drop-when-full) queue drained by a single worker goroutine throttled at
// catchUpThrottle (~2s/job). Enqueue is a cheap non-blocking map/channel op;
// it does not call the AI provider. Do not "fix" this with extra negative-cache
// state — the throttle already bounds provider QPS.
func (h *ArticleHandler) enqueueMissingTitleTranslations(items []articleResponse, isGuest bool, nativeLang string) {
	if h.RetranslateQueue == nil || isGuest || nativeLang == "" {
		return
	}
	target := ai.NormalizeLangCode(nativeLang)
	for _, it := range items {
		if it.TitleTranslated != "" {
			continue
		}
		titleLang := ai.DetectTitleLanguage(it.Title)
		if titleLang == "" || titleLang == target {
			continue
		}
		// Enqueue with the raw native language tag (e.g. "zh-CN") to match
		// the eager pipeline and the per-language article_ai join.
		h.RetranslateQueue.Enqueue(it.ID, nativeLang)
	}
}

func (h *ArticleHandler) GetByID(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	ctx := c.Request.Context()
	stateOwnerID, contentOwnerID, isGuest := h.resolveOwners(c)

	var result ArticleWithSource
	if isGuest {
		result, err = h.Service.GuestGetByID(ctx, contentOwnerID, id)
	} else {
		result, err = h.Service.GetByID(ctx, stateOwnerID, id)
	}
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
		return
	}

	state, err := h.Service.GetState(ctx, stateOwnerID, id)
	if err != nil {
		state = gen.ArticleState{}
	}

	articleResp := toArticleResponse(result.Article, true)
	articleResp.SourceTitle = result.SourceTitle
	resp := articleDetailResponse{articleResponse: articleResp, IsRead: state.IsRead, IsStarred: state.IsStarred}
	targetLang := user.NativeLanguage
	if targetLang == "" {
		targetLang = "zh-CN"
	}
	if aiRow, err := h.Service.queries.GetArticleAI(ctx, gen.GetArticleAIParams{
		ArticleID:      id,
		TargetLanguage: targetLang,
	}); err == nil {
		resp.TitleTranslated = aiRow.TitleTranslated
		resp.Summary = aiRow.Summary
	}
	if len(state.ReadingProgress) > 0 {
		resp.ReadingProgress = json.RawMessage(state.ReadingProgress)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *ArticleHandler) UpdateState(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req articleStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	ctx := c.Request.Context()
	stateOwnerID, contentOwnerID, isGuest := h.resolveOwners(c)

	if req.IsRead != nil {
		var err error
		if isGuest {
			err = h.Service.GuestSetRead(ctx, stateOwnerID, contentOwnerID, id, *req.IsRead)
		} else {
			err = h.Service.SetRead(ctx, stateOwnerID, id, *req.IsRead)
		}
		if err != nil {
			if errors.Is(err, errForbidden) {
				c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update state"})
			return
		}
	}
	if req.IsStarred != nil {
		var err error
		if isGuest {
			err = h.Service.GuestSetStarred(ctx, stateOwnerID, contentOwnerID, id, *req.IsStarred)
		} else {
			err = h.Service.SetStarred(ctx, stateOwnerID, id, *req.IsStarred)
		}
		if err != nil {
			if errors.Is(err, errForbidden) {
				c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update state"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *ArticleHandler) UpdateProgress(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	body, err := c.GetRawData()
	if err != nil || !json.Valid(body) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	stateOwnerID, contentOwnerID, isGuest := h.resolveOwners(c)
	var progErr error
	if isGuest {
		progErr = h.Service.GuestUpdateProgress(c.Request.Context(), stateOwnerID, contentOwnerID, id, body)
	} else {
		progErr = h.Service.UpdateProgress(c.Request.Context(), stateOwnerID, id, body)
	}
	if progErr != nil {
		if errors.Is(progErr, errForbidden) {
			c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update progress"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *ArticleHandler) BatchState(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	var req batchStateRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Scope == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope required"})
		return
	}

	stateOwnerID, contentOwnerID, isGuest := h.resolveOwners(c)
	var articleIDs []int64
	var err error
	if isGuest {
		articleIDs, err = h.Service.GuestBatchSetRead(c.Request.Context(), stateOwnerID, contentOwnerID, req.Scope, req.IsRead)
	} else {
		articleIDs, err = h.Service.BatchSetRead(c.Request.Context(), stateOwnerID, req.Scope, req.IsRead)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, batchStateResponse{Status: "updated", Updated: len(articleIDs), ArticleIDs: articleIDs})
}

func (h *ArticleHandler) Changes(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	sinceRaw := c.Query("since")
	if sinceRaw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "since parameter required"})
		return
	}

	since, err := parseTime(sinceRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid since format"})
		return
	}

	changes, err := h.Service.ListChanges(c.Request.Context(), user.ID, since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list changes"})
		return
	}

	items := make([]articleChangeResponse, 0, len(changes))
	for _, change := range changes {
		items = append(items, toArticleChangeResponse(change))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *ArticleHandler) LoadOriginal(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	content, err := h.Service.LoadOriginal(c.Request.Context(), user.ID, id)
	if err != nil {
		switch {
		case errors.Is(err, errNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
		case errors.Is(err, errOriginalUnsupportedURL), errors.Is(err, errOriginalUnsafeURL):
			c.JSON(http.StatusBadRequest, gin.H{"error": "original URL is not supported"})
		case errors.Is(err, errOriginalNotHTML):
			c.JSON(http.StatusBadRequest, gin.H{"error": "original page is not readable HTML"})
		case errors.Is(err, errOriginalTooLarge):
			c.JSON(http.StatusBadRequest, gin.H{"error": "original page is too large"})
		case errors.Is(err, errOriginalNoContent):
			c.JSON(http.StatusBadRequest, gin.H{"error": "could not find readable original content"})
		default:
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to load original content"})
		}
		return
	}

	c.JSON(http.StatusOK, originalContentResponse{
		URL:         content.URL,
		Title:       content.Title,
		ContentHtml: content.ContentHTML,
		ContentText: content.ContentText,
	})
}

func (h *ArticleHandler) itemsForList(ctx context.Context, stateOwnerID, contentOwnerID int64, isGuest bool, lang, tab, filter, query, sourceIDRaw, cursorRaw string, limit int32) ([]articleResponse, error) {

	if strings.TrimSpace(query) != "" {
		if isGuest {
			rows, err := h.Service.GuestSearch(ctx, contentOwnerID, query)
			if err != nil {
				return nil, err
			}
			// Convert GuestSearchArticlesRow to articleResponse
			resp := make([]articleResponse, 0, len(rows))
			for _, r := range rows {
				ar := articleResponse{
					ID:       r.ID,
					SourceID: r.SourceID,
					Title:    r.Title,
					Link:     r.Link,
					Language: r.Language,
				}
				if r.PublishedAt.Valid {
					s := r.PublishedAt.Time.UTC().Format(time.RFC3339Nano)
					ar.PublishedAt = &s
				}
				resp = append(resp, ar)
			}
			return resp, nil
		}
		rows, err := h.Service.Search(ctx, stateOwnerID, query)
		if err != nil {
			return nil, err
		}
		return toArticleResponses(rows, false), nil
	}

	if strings.TrimSpace(sourceIDRaw) != "" {
		sourceID, err := strconv.ParseInt(sourceIDRaw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid source_id")
		}
		if isGuest {
			rows, err := h.Service.GuestListBySourceEnriched(ctx, stateOwnerID, contentOwnerID, sourceID, lang, filter)
			if err != nil {
				return nil, err
			}
			return enrichedToArticleResponses(rows), nil
		}
		rows, err := h.Service.ListBySourceEnriched(ctx, stateOwnerID, sourceID, lang, filter)
		if err != nil {
			return nil, err
		}
		return enrichedToArticleResponses(rows), nil
	}

	switch tab {
	case "", "today":
		if isGuest {
			rows, err := h.Service.GuestListTodayEnriched(ctx, stateOwnerID, contentOwnerID, lang, filter)
			if err != nil {
				return nil, err
			}
			return enrichedToArticleResponses(rows), nil
		}
		rows, err := h.Service.ListTodayEnriched(ctx, stateOwnerID, lang, filter)
		if err != nil {
			return nil, err
		}
		return enrichedToArticleResponses(rows), nil
	case "starred":
		if isGuest {
			rows, err := h.Service.GuestListStarredEnriched(ctx, stateOwnerID, contentOwnerID, lang)
			if err != nil {
				return nil, err
			}
			return enrichedToArticleResponses(rows), nil
		}
		rows, err := h.Service.ListStarredEnriched(ctx, stateOwnerID, lang)
		if err != nil {
			return nil, err
		}
		return enrichedToArticleResponses(rows), nil
	case "stream", "all":
		cursor, err := parseTimePtr(cursorRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor")
		}
		if isGuest {
			rows, err := h.Service.GuestListStreamEnriched(ctx, stateOwnerID, contentOwnerID, cursor, clampListLimit(limit), lang, filter)
			if err != nil {
				return nil, err
			}
			return enrichedToArticleResponses(rows), nil
		}
		rows, err := h.Service.ListStreamEnriched(ctx, stateOwnerID, cursor, clampListLimit(limit), lang, filter)
		if err != nil {
			return nil, err
		}
		return enrichedToArticleResponses(rows), nil
	default:
		return nil, fmt.Errorf("invalid tab")
	}
}

func (h *ArticleHandler) countsForList(ctx context.Context, stateOwnerID, contentOwnerID int64, isGuest bool, tab, query, sourceIDRaw string) (*articleReadCounts, error) {
	if strings.TrimSpace(query) != "" {
		return nil, nil
	}

	var counts ArticleReadCounts
	var err error
	if strings.TrimSpace(sourceIDRaw) != "" {
		sourceID, parseErr := strconv.ParseInt(sourceIDRaw, 10, 64)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid source_id")
		}
		if isGuest {
			counts, err = h.Service.GuestCountBySourceReadState(ctx, stateOwnerID, contentOwnerID, sourceID)
		} else {
			counts, err = h.Service.CountBySourceReadState(ctx, stateOwnerID, sourceID)
		}
	} else {
		switch tab {
		case "", "today":
			if isGuest {
				counts, err = h.Service.GuestCountTodayByReadState(ctx, stateOwnerID, contentOwnerID)
			} else {
				counts, err = h.Service.CountTodayByReadState(ctx, stateOwnerID)
			}
		case "stream", "all":
			if isGuest {
				counts, err = h.Service.GuestCountStreamByReadState(ctx, stateOwnerID, contentOwnerID)
			} else {
				counts, err = h.Service.CountStreamByReadState(ctx, stateOwnerID)
			}
		case "starred":
			return nil, nil
		default:
			return nil, fmt.Errorf("invalid tab")
		}
	}
	if err != nil {
		return nil, err
	}
	return &articleReadCounts{Unread: counts.Unread, All: counts.All, Read: counts.Read}, nil
}

func toArticleResponses(items []gen.Article, detail bool) []articleResponse {
	resp := make([]articleResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, toArticleResponse(item, detail))
	}
	return resp
}

func enrichedToArticleResponses(items []EnrichedArticle) []articleResponse {
	resp := make([]articleResponse, 0, len(items))
	for _, item := range items {
		r := articleResponse{
			ID: item.ID, SourceID: item.SourceID, Title: item.Title,
			TitleTranslated: item.TitleTranslated, Summary: item.Summary,
			SourceTitle: item.SourceTitle, Link: item.Link, Language: item.Language,
			IsRead: item.IsRead, IsStarred: item.IsStarred,
			WordCount: countWords(item.ContentText),
		}
		if item.Author.Valid {
			author := item.Author.String
			r.Author = &author
		}
		if item.PublishedAt.Valid {
			publishedAt := item.PublishedAt.Time.UTC().Format(time.RFC3339Nano)
			r.PublishedAt = &publishedAt
		}
		resp = append(resp, r)
	}
	return resp
}

func toArticleResponse(item gen.Article, detail bool) articleResponse {
	resp := articleResponse{ID: item.ID, SourceID: item.SourceID, Title: item.Title, Link: item.Link, Language: item.Language}
	if item.Author.Valid {
		author := item.Author.String
		resp.Author = &author
	}
	if item.PublishedAt.Valid {
		publishedAt := item.PublishedAt.Time.UTC().Format(time.RFC3339Nano)
		resp.PublishedAt = &publishedAt
	}
	if detail {
		resp.ContentHtml = item.ContentHtml
		resp.ContentText = item.ContentText
	}
	return resp
}

func countWords(text string) int {
	words := len(strings.Fields(text))
	chars := len([]rune(strings.Join(strings.Fields(text), "")))
	if estimate := chars / 2; estimate > words {
		return estimate
	}
	return words
}

func parseLimit(raw string) int32 {
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 50
	}
	return clampListLimit(int32(v))
}

func clampListLimit(limit int32) int32 {
	if limit <= 0 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func parseTime(raw string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC(), nil
	}
	return time.Parse(time.RFC3339, raw)
}

func parseTimePtr(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	t, err := parseTime(raw)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

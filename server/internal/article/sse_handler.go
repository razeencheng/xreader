package article

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/db/gen"
	"github.com/razeencheng/xreader/internal/ai"
	"github.com/razeencheng/xreader/internal/middleware"
)

type SSEHandler struct {
	pool           *pgxpool.Pool
	queries        *gen.Queries
	aiClient       ai.AIClient
	batchSize      int
	ContentOwnerID func(ctx context.Context) (int64, error)
}

func NewSSEHandler(pool *pgxpool.Pool, aiClient ai.AIClient, batchSize int) *SSEHandler {
	if batchSize <= 0 {
		batchSize = 1
	}
	return &SSEHandler{
		pool:      pool,
		queries:   gen.New(pool),
		aiClient:  aiClient,
		batchSize: batchSize,
	}
}

func (h *SSEHandler) BodyTranslation(c *gin.Context) {
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

	targetLang := user.NativeLanguage
	if targetLang == "" {
		targetLang = "zh-CN"
	}

	ctx := c.Request.Context()

	article, err := h.queries.GetArticleByID(ctx, articleID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
		return
	}
	// Verify article ownership via its source
	contentOwnerID := user.ID
	if user.Role == "guest" && h.ContentOwnerID != nil {
		if id, err := h.ContentOwnerID(ctx); err == nil {
			contentOwnerID = id
		}
	}
	source, err := h.queries.GetSourceByID(ctx, article.SourceID)
	if err != nil || source.UserID != contentOwnerID {
		c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
		return
	}
	detectedLang := ai.DetectLanguage(article.ContentText, article.Language)
	if detectedLang == targetLang {
		setSSEHeaders(c)
		writeSSENamedEvent(c.Writer, "same-language", map[string]any{})
		return
	}

	paragraphs := ai.SplitParagraphs(article.ContentHtml)
	start, end, err := parseBodyTranslationRange(c, len(paragraphs))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	requestedParagraphs := paragraphs[start:end]

	cached := make(map[int]ai.TranslatedParagraph)
	aiRow, err := h.queries.GetArticleAI(ctx, gen.GetArticleAIParams{ArticleID: articleID, TargetLanguage: targetLang})
	if err == nil {
		var stale bool
		cached, stale = validCachedTranslations(aiRow.BodyTranslationContent, paragraphs)
		if stale {
			cached = make(map[int]ai.TranslatedParagraph)
			if resetErr := h.queries.ResetBodyTranslation(ctx, gen.ResetBodyTranslationParams{
				ArticleID: articleID, TargetLanguage: targetLang,
			}); resetErr != nil {
				log.Printf("sse: reset stale cached body translation for article %d: %v", articleID, resetErr)
			}
		}
	}

	if len(requestedParagraphs) == 0 {
		setSSEHeaders(c)
		writeSSENamedEvent(c.Writer, "done", map[string]any{})
		return
	}

	missing := missingCachedParagraphs(requestedParagraphs, cached)
	if len(missing) == 0 {
		setSSEHeaders(c)
		writeCachedBodyTranslationRange(c.Writer, requestedParagraphs, cached)
		return
	}

	if h.aiClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI service not configured"})
		return
	}

	setSSEHeaders(c)
	h.streamTranslation(ctx, c.Writer, articleID, targetLang, paragraphs, requestedParagraphs, cached)
}

func parseBodyTranslationRange(c *gin.Context, total int) (int, int, error) {
	start := 0
	count := total

	if rawStart := c.Query("start"); rawStart != "" {
		parsed, err := strconv.Atoi(rawStart)
		if err != nil || parsed < 0 {
			return 0, 0, fmt.Errorf("invalid start")
		}
		start = parsed
	}

	if rawCount := c.Query("count"); rawCount != "" {
		parsed, err := strconv.Atoi(rawCount)
		if err != nil || parsed <= 0 {
			return 0, 0, fmt.Errorf("invalid count")
		}
		count = parsed
	}

	if start > total {
		start = total
	}
	end := start + count
	if end > total {
		end = total
	}
	return start, end, nil
}

func validCachedTranslations(payload []byte, paragraphs []ai.Paragraph) (map[int]ai.TranslatedParagraph, bool) {
	cached := make(map[int]ai.TranslatedParagraph)
	if len(payload) == 0 {
		return cached, false
	}

	var content []ai.TranslatedParagraph
	if err := json.Unmarshal(payload, &content); err != nil {
		return cached, true
	}

	for _, item := range content {
		if item.Index < 0 || item.Index >= len(paragraphs) {
			return cached, true
		}
		if normalizeSSEText(item.Original) != paragraphs[item.Index].Original {
			return cached, true
		}
		cached[item.Index] = item
	}

	return cached, false
}

func normalizeSSEText(raw string) string {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}

func missingCachedParagraphs(paragraphs []ai.Paragraph, cached map[int]ai.TranslatedParagraph) []ai.Paragraph {
	missing := make([]ai.Paragraph, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		if _, ok := cached[paragraph.Index]; !ok {
			missing = append(missing, paragraph)
		}
	}
	return missing
}

func (h *SSEHandler) streamTranslation(ctx context.Context, w http.ResponseWriter, articleID int64, targetLang string, allParagraphs []ai.Paragraph, requestedParagraphs []ai.Paragraph, cached map[int]ai.TranslatedParagraph) {
	_ = h.queries.EnsureArticleAI(ctx, gen.EnsureArticleAIParams{
		ArticleID: articleID, TargetLanguage: targetLang,
	})
	_ = h.queries.SetBodyTranslationStatus(ctx, gen.SetBodyTranslationStatusParams{
		ArticleID: articleID, TargetLanguage: targetLang, BodyTranslationStatus: "processing",
	})

	for _, paragraph := range requestedParagraphs {
		if tp, ok := cached[paragraph.Index]; ok {
			writeSSENamedEvent(w, "paragraph", map[string]any{
				"index":       tp.Index,
				"original":    tp.Original,
				"translation": tp.Translation,
			})
		}
	}

	missing := missingCachedParagraphs(requestedParagraphs, cached)
	newlyTranslated := make(map[int]ai.TranslatedParagraph, len(missing))

	for start := 0; start < len(missing); start += h.batchSize {
		end := start + h.batchSize
		if end > len(missing) {
			end = len(missing)
		}

		batch := missing[start:end]
		results, err := h.translateBatch(ctx, batch, targetLang)
		if err != nil {
			log.Printf("sse: translate batch for article %d: %v", articleID, err)
			break
		}

		for _, tp := range results {
			writeSSENamedEvent(w, "paragraph", map[string]any{
				"index":       tp.Index,
				"original":    tp.Original,
				"translation": tp.Translation,
			})
			newlyTranslated[tp.Index] = tp
		}
	}

	writeSSENamedEvent(w, "done", map[string]any{})

	go func() {
		bgCtx := context.Background()
		if err := h.persistMergedBodyTranslation(bgCtx, articleID, targetLang, allParagraphs, newlyTranslated); err != nil {
			log.Printf("sse: persist body translation for article %d: %v", articleID, err)
		}
	}()
}

func (h *SSEHandler) persistMergedBodyTranslation(ctx context.Context, articleID int64, targetLang string, paragraphs []ai.Paragraph, translated map[int]ai.TranslatedParagraph) error {
	if len(translated) == 0 {
		return nil
	}

	merged := make(map[int]ai.TranslatedParagraph, len(translated))
	aiRow, err := h.queries.GetArticleAI(ctx, gen.GetArticleAIParams{ArticleID: articleID, TargetLanguage: targetLang})
	if err == nil {
		cached, stale := validCachedTranslations(aiRow.BodyTranslationContent, paragraphs)
		if !stale {
			for index, item := range cached {
				merged[index] = item
			}
		}
	}
	for index, item := range translated {
		merged[index] = item
	}

	content := orderedCachedTranslations(paragraphs, merged)
	status := "processing"
	if len(content) == len(paragraphs) {
		status = "done"
	}

	data, err := json.Marshal(content)
	if err != nil {
		return err
	}

	return h.queries.SetBodyTranslation(ctx, gen.SetBodyTranslationParams{
		ArticleID: articleID, TargetLanguage: targetLang,
		BodyTranslationContent: data, BodyTranslationStatus: status,
	})
}

func orderedCachedTranslations(paragraphs []ai.Paragraph, cached map[int]ai.TranslatedParagraph) []ai.TranslatedParagraph {
	content := make([]ai.TranslatedParagraph, 0, len(cached))
	for _, paragraph := range paragraphs {
		if item, ok := cached[paragraph.Index]; ok {
			content = append(content, item)
		}
	}
	return content
}

func (h *SSEHandler) translateBatch(ctx context.Context, batch []ai.Paragraph, targetLang string) ([]ai.TranslatedParagraph, error) {
	var prompt strings.Builder
	for _, p := range batch {
		fmt.Fprintf(&prompt, "[%d] %s\n", p.Index, p.Original)
	}

	resp, err := h.aiClient.ChatCompletion(ctx, ai.ChatRequest{
		Messages: []ai.ChatMessage{
			{Role: "system", Content: fmt.Sprintf("Translate each numbered paragraph into %s. Preserve each [index] label exactly, do not reorder paragraphs, and do not merge separate paragraphs.", targetLang)},
			{Role: "user", Content: prompt.String()},
		},
	})
	if err != nil {
		return nil, err
	}

	return ai.ParseParagraphTranslations(batch, resp.Content), nil
}

func writeCachedBodyTranslationRange(w http.ResponseWriter, paragraphs []ai.Paragraph, cached map[int]ai.TranslatedParagraph) {
	for _, paragraph := range paragraphs {
		tp, ok := cached[paragraph.Index]
		if !ok {
			continue
		}
		writeSSENamedEvent(w, "paragraph", map[string]any{
			"index":       tp.Index,
			"original":    tp.Original,
			"translation": tp.Translation,
		})
	}
	writeSSENamedEvent(w, "done", map[string]any{})
}

func writeSSENamedEvent(w http.ResponseWriter, event string, payload map[string]any) {
	data, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func setSSEHeaders(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Status(http.StatusOK)
	c.Writer.Flush()
}

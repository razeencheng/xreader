package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/db/gen"
)

type TranslatedParagraph struct {
	Index       int    `json:"index"`
	Original    string `json:"original"`
	Translation string `json:"translation"`
}

type LazyJob struct {
	pool       *pgxpool.Pool
	queries    *gen.Queries
	client     AIClient
	articleID  int64
	targetLang string
	batchSize  int
}

func NewLazyJob(pool *pgxpool.Pool, client AIClient, articleID int64, targetLang string, batchSize int) *LazyJob {
	if batchSize <= 0 {
		batchSize = 1
	}
	return &LazyJob{
		pool:       pool,
		queries:    gen.New(pool),
		client:     client,
		articleID:  articleID,
		targetLang: targetLang,
		batchSize:  batchSize,
	}
}

func (j *LazyJob) Run(ctx context.Context) error {
	article, err := j.queries.GetArticleByID(ctx, j.articleID)
	if err != nil {
		return fmt.Errorf("get article: %w", err)
	}

	if err := j.ensureAI(ctx); err != nil {
		return err
	}

	if err := j.queries.SetBodyTranslationStatus(ctx, gen.SetBodyTranslationStatusParams{
		ArticleID:             j.articleID,
		TargetLanguage:        j.targetLang,
		BodyTranslationStatus: "processing",
	}); err != nil {
		return fmt.Errorf("set body translation processing: %w", err)
	}

	paragraphs := SplitParagraphs(article.ContentHtml)
	if len(paragraphs) == 0 {
		return j.persistDone(ctx, nil)
	}

	translated := make([]TranslatedParagraph, 0, len(paragraphs))
	for start := 0; start < len(paragraphs); start += j.batchSize {
		end := start + j.batchSize
		if end > len(paragraphs) {
			end = len(paragraphs)
		}

		batch := paragraphs[start:end]
		batchResult, err := j.translateBatch(ctx, batch)
		if err != nil {
			if setErr := j.queries.SetBodyTranslationStatus(ctx, gen.SetBodyTranslationStatusParams{
				ArticleID:             j.articleID,
				TargetLanguage:        j.targetLang,
				BodyTranslationStatus: "failed",
			}); setErr != nil {
				log.Printf("ai: set body translation failed status error: %v", setErr)
			}
			return fmt.Errorf("translate batch: %w", err)
		}

		translated = append(translated, batchResult...)
	}

	return j.persistDone(ctx, translated)
}

func (j *LazyJob) ensureAI(ctx context.Context) error {
	if err := j.queries.EnsureArticleAI(ctx, gen.EnsureArticleAIParams{
		ArticleID:      j.articleID,
		TargetLanguage: j.targetLang,
	}); err != nil {
		return fmt.Errorf("ensure article ai: %w", err)
	}
	return nil
}

func (j *LazyJob) translateBatch(ctx context.Context, batch []Paragraph) ([]TranslatedParagraph, error) {
	var prompt strings.Builder
	for _, paragraph := range batch {
		fmt.Fprintf(&prompt, "[%d] %s\n", paragraph.Index, paragraph.Original)
	}

	resp, err := j.client.ChatCompletion(ctx, ChatRequest{
		Messages: []ChatMessage{
			{
				Role:    "system",
				Content: fmt.Sprintf("Translate each numbered paragraph into %s. Preserve each [index] label exactly, do not reorder paragraphs, and do not merge separate paragraphs.", j.targetLang),
			},
			{
				Role:    "user",
				Content: prompt.String(),
			},
		},
	})
	if err != nil {
		return nil, err
	}

	return ParseParagraphTranslations(batch, resp.Content), nil
}


func ParseParagraphTranslations(batch []Paragraph, response string) []TranslatedParagraph {
	lines := splitNonEmptyTranslationLines(response)
	segments, labelOrder := collectNumberedTranslationSegments(lines, batch)

	useNumberedOrder := len(labelOrder) > 0
	if useNumberedOrder && labelsMatchBatchIndexes(labelOrder, batch) {
		useNumberedOrder = false
	}

	results := make([]TranslatedParagraph, 0, len(batch))
	for i, paragraph := range batch {
		translation := ""
		switch {
		case useNumberedOrder && i < len(labelOrder):
			translation = strings.Join(segments[labelOrder[i]], "\n")
		case len(labelOrder) > 0:
			translation = strings.Join(segments[paragraph.Index], "\n")
		case i < len(lines):
			translation = stripTranslationLabel(lines[i])
		}

		results = append(results, TranslatedParagraph{
			Index:       paragraph.Index,
			Original:    paragraph.Original,
			Translation: strings.TrimSpace(translation),
		})
	}

	return results
}

func splitNonEmptyTranslationLines(response string) []string {
	rawLines := strings.Split(strings.TrimSpace(response), "\n")
	lines := make([]string, 0, len(rawLines))
	for _, raw := range rawLines {
		line := strings.TrimSpace(raw)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func collectNumberedTranslationSegments(lines []string, batch []Paragraph) (map[int][]string, []int) {
	segments := make(map[int][]string)
	labelOrder := make([]int, 0)
	currentIndex := -1
	batchIndexes := make(map[int]struct{}, len(batch))
	for _, paragraph := range batch {
		batchIndexes[paragraph.Index] = struct{}{}
	}

	for _, line := range lines {
		if index, text, ok := parseTranslationLabel(line, batchIndexes, len(batch)); ok {
			currentIndex = index
			if _, exists := segments[index]; !exists {
				labelOrder = append(labelOrder, index)
			}
			if strings.TrimSpace(text) != "" {
				segments[index] = append(segments[index], strings.TrimSpace(text))
			} else if _, exists := segments[index]; !exists {
				segments[index] = []string{}
			}
			continue
		}

		if currentIndex >= 0 {
			segments[currentIndex] = append(segments[currentIndex], line)
		}
	}

	return segments, labelOrder
}

func labelsMatchBatchIndexes(labels []int, batch []Paragraph) bool {
	if len(labels) < len(batch) {
		return false
	}
	for i, paragraph := range batch {
		if labels[i] != paragraph.Index {
			return false
		}
	}
	return true
}

func parseTranslationLabel(line string, batchIndexes map[int]struct{}, batchLen int) (int, string, bool) {
	if strings.HasPrefix(line, "[") {
		if close := strings.Index(line, "]"); close > 1 {
			index, err := strconv.Atoi(strings.TrimSpace(line[1:close]))
			if err == nil {
				return index, trimLabelSeparator(line[close+1:]), true
			}
		}
	}

	for _, separator := range []string{".", ")", "、"} {
		if before, after, ok := strings.Cut(line, separator); ok {
			index, err := strconv.Atoi(strings.TrimSpace(before))
			_, isActualIndex := batchIndexes[index]
			isOrdinalIndex := index >= 0 && index < batchLen
			isOneBasedOrdinal := index > 0 && index <= batchLen
			if err == nil && strings.TrimSpace(after) != "" && (isActualIndex || isOrdinalIndex || isOneBasedOrdinal) {
				return index, strings.TrimSpace(after), true
			}
		}
	}

	return 0, "", false
}

func stripTranslationLabel(line string) string {
	if strings.HasPrefix(line, "[") {
		if close := strings.Index(line, "]"); close > 1 {
			if _, err := strconv.Atoi(strings.TrimSpace(line[1:close])); err == nil {
				return trimLabelSeparator(line[close+1:])
			}
		}
	}
	if _, text, ok := parseTranslationLabel(line, nil, 0); ok {
		return text
	}
	return strings.TrimSpace(line)
}

func trimLabelSeparator(text string) string {
	return strings.TrimLeft(strings.TrimSpace(text), ":：.-–— ")
}

func (j *LazyJob) persistDone(ctx context.Context, translated []TranslatedParagraph) error {
	if translated == nil {
		translated = []TranslatedParagraph{}
	}

	data, err := json.Marshal(translated)
	if err != nil {
		return fmt.Errorf("marshal body translation: %w", err)
	}

	if err := j.queries.SetBodyTranslationContent(ctx, gen.SetBodyTranslationContentParams{
		ArticleID:              j.articleID,
		TargetLanguage:         j.targetLang,
		BodyTranslationContent: data,
	}); err != nil {
		return fmt.Errorf("persist body translation: %w", err)
	}

	return nil
}

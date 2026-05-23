package ai

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/db/gen"
)

type EagerJob struct {
	pool       *pgxpool.Pool
	queries    *gen.Queries
	client     AIClient
	articleID  int64
	targetLang string
}

func NewEagerJob(pool *pgxpool.Pool, client AIClient, articleID int64, targetLang string) *EagerJob {
	return &EagerJob{
		pool:       pool,
		queries:    gen.New(pool),
		client:     client,
		articleID:  articleID,
		targetLang: targetLang,
	}
}

func (j *EagerJob) Run(ctx context.Context) error {
	article, err := j.queries.GetArticleByID(ctx, j.articleID)
	if err != nil {
		return fmt.Errorf("get article: %w", err)
	}

	if err := j.queries.EnsureArticleAI(ctx, gen.EnsureArticleAIParams{
		ArticleID:      j.articleID,
		TargetLanguage: j.targetLang,
	}); err != nil {
		return fmt.Errorf("ensure article_ai: %w", err)
	}

	// Title decision: from the title's own language, normalized so en-US/ja-JP
	// native tags compare correctly against detector codes (en/ja). Independent
	// of the body — the body-based decision was the root cause.
	titleLang := DetectTitleLanguage(article.Title)
	titleNeedsTranslation := titleLang != "" && titleLang != NormalizeLangCode(j.targetLang)

	// Summary decision: unchanged from the original (body-based, raw targetLang)
	// so summary behaviour cannot regress.
	bodyLang := DetectLanguage(article.ContentText, article.Language)
	bodySameLang := bodyLang == j.targetLang
	shortContent := len(article.ContentText) < 280
	summaryNeeded := !bodySameLang && !shortContent

	switch {
	case titleNeedsTranslation && summaryNeeded:
		// Both needed: keep the single combined AI call (the only combined case).
		return j.runCombined(ctx, article)

	case titleNeedsTranslation:
		if err := j.runTitleOnly(ctx, article); err != nil {
			return err
		}
		return j.finishSummary(ctx, article, shortContent)

	default:
		// Title already target-language (or undetermined) -> keep original.
		if err := j.queries.UpsertTitleTranslation(ctx, gen.UpsertTitleTranslationParams{
			ArticleID:       j.articleID,
			TargetLanguage:  j.targetLang,
			TitleTranslated: article.Title,
		}); err != nil {
			return fmt.Errorf("upsert title: %w", err)
		}
		return j.finishSummary(ctx, article, shortContent)
	}
}

// finishSummary records the summary for the non-combined paths, exactly
// matching the original pipeline's summary outcomes: short content is
// skipped("short"); otherwise a summary is generated.
func (j *EagerJob) finishSummary(ctx context.Context, article gen.Article, shortContent bool) error {
	if shortContent {
		return j.queries.UpsertSummary(ctx, gen.UpsertSummaryParams{
			ArticleID:         j.articleID,
			TargetLanguage:    j.targetLang,
			SummaryStatus:     "skipped",
			SummarySkipReason: "short",
		})
	}
	return j.runSummaryOnly(ctx, article)
}

func (j *EagerJob) runCombined(ctx context.Context, article gen.Article) error {
	resp, err := j.client.ChatCompletion(ctx, ChatRequest{
		Messages: []ChatMessage{
			{Role: "system", Content: CombinedTitleSummaryPrompt(j.targetLang)},
			{Role: "user", Content: CombinedTitleSummaryUserMessage(article.Title, article.ContentText)},
		},
	})
	if err != nil {
		log.Printf("ai: combined title+summary failed for article %d: %v", j.articleID, err)
		return fmt.Errorf("combined: %w", err)
	}

	title, summary := parseCombinedResponse(resp.Content)

	if title != "" {
		if err := j.queries.UpsertTitleTranslation(ctx, gen.UpsertTitleTranslationParams{
			ArticleID:       j.articleID,
			TargetLanguage:  j.targetLang,
			TitleTranslated: title,
		}); err != nil {
			return fmt.Errorf("upsert title: %w", err)
		}
	}
	// title == "": do NOT write the original title; leave title_translated = ''
	// so it is not treated as permanently done.

	status := "done"
	if summary == "" {
		status = "failed"
	}
	return j.queries.UpsertSummary(ctx, gen.UpsertSummaryParams{
		ArticleID:      j.articleID,
		TargetLanguage: j.targetLang,
		Summary:        summary,
		SummaryStatus:  status,
	})
}

func (j *EagerJob) runTitleOnly(ctx context.Context, article gen.Article) error {
	resp, err := j.client.ChatCompletion(ctx, ChatRequest{
		Messages: []ChatMessage{
			{Role: "system", Content: TitleTranslationPrompt(j.targetLang)},
			{Role: "user", Content: article.Title},
		},
	})
	if err != nil {
		log.Printf("ai: title translation failed for article %d: %v", j.articleID, err)
		return nil
	}
	return j.queries.UpsertTitleTranslation(ctx, gen.UpsertTitleTranslationParams{
		ArticleID:       j.articleID,
		TargetLanguage:  j.targetLang,
		TitleTranslated: resp.Content,
	})
}

func (j *EagerJob) runSummaryOnly(ctx context.Context, article gen.Article) error {
	contentForSummary := article.ContentText
	const maxSummaryChars = 8000
	if len(contentForSummary) > maxSummaryChars {
		contentForSummary = contentForSummary[:maxSummaryChars]
	}
	resp, err := j.client.ChatCompletion(ctx, ChatRequest{
		Messages: []ChatMessage{
			{Role: "system", Content: SummaryPrompt(j.targetLang)},
			{Role: "user", Content: contentForSummary},
		},
	})
	if err != nil {
		_ = j.queries.UpsertSummary(ctx, gen.UpsertSummaryParams{
			ArticleID:      j.articleID,
			TargetLanguage: j.targetLang,
			SummaryStatus:  "failed",
		})
		return fmt.Errorf("summary: %w", err)
	}
	return j.queries.UpsertSummary(ctx, gen.UpsertSummaryParams{
		ArticleID:      j.articleID,
		TargetLanguage: j.targetLang,
		Summary:        resp.Content,
		SummaryStatus:  "done",
	})
}

func parseCombinedResponse(content string) (title, summary string) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if t, ok := strings.CutPrefix(trimmed, "TITLE:"); ok {
			title = strings.TrimSpace(t)
		} else if s, ok := strings.CutPrefix(trimmed, "SUMMARY:"); ok {
			summary = strings.TrimSpace(s)
		}
	}
	if summary == "" && title != "" {
		parts := strings.SplitN(content, "\n\n", 2)
		if len(parts) == 2 {
			summary = strings.TrimSpace(parts[1])
		}
	}
	return
}

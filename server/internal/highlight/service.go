package highlight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf16"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/db/gen"
	"github.com/razeencheng/xreader/internal/ai"
)

var errNotFound = errors.New("not found")

const defaultTargetLanguage = "zh-CN"

type CreateParams struct {
	ArticleID       int64
	Layer           string
	ParagraphIndex  int32
	TextStartOffset int32
	TextEndOffset   int32
	QuotedText      string
	Note            *string
	TargetLanguage  string
}

type HighlightService struct {
	pool    *pgxpool.Pool
	queries *gen.Queries
}

func NewHighlightService(pool *pgxpool.Pool) *HighlightService {
	return &HighlightService{pool: pool, queries: gen.New(pool)}
}

func (s *HighlightService) Create(ctx context.Context, userID, contentOwnerID int64, params CreateParams) (*gen.Highlight, error) {
	params.Layer = strings.TrimSpace(params.Layer)
	if err := validateLayer(params.Layer); err != nil {
		return nil, err
	}
	if err := validateOffsets(params.TextStartOffset, params.TextEndOffset); err != nil {
		return nil, err
	}
	if err := s.validateQuotedText(ctx, contentOwnerID, params); err != nil {
		return nil, err
	}

	created, err := s.queries.CreateHighlight(ctx, gen.CreateHighlightParams{
		UserID:          userID,
		ArticleID:       params.ArticleID,
		Layer:           params.Layer,
		ParagraphIndex:  params.ParagraphIndex,
		TextStartOffset: params.TextStartOffset,
		TextEndOffset:   params.TextEndOffset,
		QuotedText:      params.QuotedText,
		Note:            noteOrEmpty(params.Note),
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (s *HighlightService) ListByArticle(ctx context.Context, userID, articleID int64) ([]gen.Highlight, error) {
	return s.queries.ListHighlightsByArticle(ctx, gen.ListHighlightsByArticleParams{UserID: userID, ArticleID: articleID})
}

func (s *HighlightService) ListByUser(ctx context.Context, userID int64, limit, offset int32) ([]gen.ListHighlightsByUserRow, error) {
	return s.queries.ListHighlightsByUser(ctx, gen.ListHighlightsByUserParams{UserID: userID, Limit: clampLimit(limit), Offset: clampOffset(offset)})
}

func (s *HighlightService) Search(ctx context.Context, userID int64, query string, limit, offset int32) ([]gen.SearchHighlightsRow, error) {
	return s.queries.SearchHighlights(ctx, gen.SearchHighlightsParams{UserID: userID, Column2: pgtype.Text{String: strings.TrimSpace(query), Valid: strings.TrimSpace(query) != ""}, Limit: clampLimit(limit), Offset: clampOffset(offset)})
}

func (s *HighlightService) UpdateNote(ctx context.Context, userID, highlightID int64, note string) error {
	if _, err := s.queries.GetHighlight(ctx, gen.GetHighlightParams{ID: highlightID, UserID: userID}); err != nil {
		return errNotFound
	}
	return s.queries.UpdateHighlightNote(ctx, gen.UpdateHighlightNoteParams{ID: highlightID, UserID: userID, Note: note})
}

func (s *HighlightService) Delete(ctx context.Context, userID, highlightID int64) error {
	if _, err := s.queries.GetHighlight(ctx, gen.GetHighlightParams{ID: highlightID, UserID: userID}); err != nil {
		return errNotFound
	}
	return s.queries.DeleteHighlight(ctx, gen.DeleteHighlightParams{ID: highlightID, UserID: userID})
}

func (s *HighlightService) validateQuotedText(ctx context.Context, contentOwnerID int64, params CreateParams) error {
	paragraphText, err := s.paragraphText(ctx, contentOwnerID, params)
	if err != nil {
		return err
	}

	expected, err := utf16Substring(paragraphText, params.TextStartOffset, params.TextEndOffset)
	if err != nil {
		return err
	}
	if expected != params.QuotedText {
		return fmt.Errorf("quoted_text does not match selected text")
	}
	return nil
}

func (s *HighlightService) paragraphText(ctx context.Context, contentOwnerID int64, params CreateParams) (string, error) {
	article, err := s.queries.GetArticleByID(ctx, params.ArticleID)
	if err != nil {
		return "", errNotFound
	}

	source, err := s.queries.GetSourceByID(ctx, article.SourceID)
	if err != nil || source.UserID != contentOwnerID {
		return "", errNotFound
	}

	switch params.Layer {
	case "translation":
		targetLanguage := params.TargetLanguage
		if targetLanguage == "" {
			targetLanguage = defaultTargetLanguage
		}
		aiRow, err := s.queries.GetArticleAI(ctx, gen.GetArticleAIParams{ArticleID: params.ArticleID, TargetLanguage: targetLanguage})
		if err != nil {
			return "", errNotFound
		}

		var paragraphs []ai.TranslatedParagraph
		if len(aiRow.BodyTranslationContent) > 0 {
			if err := json.Unmarshal(aiRow.BodyTranslationContent, &paragraphs); err != nil {
				return "", fmt.Errorf("parse translation content: %w", err)
			}
		}
		if params.ParagraphIndex < 0 || int(params.ParagraphIndex) >= len(paragraphs) {
			return "", fmt.Errorf("paragraph index out of range")
		}
		text := strings.TrimSpace(paragraphs[params.ParagraphIndex].Translation)
		if text == "" {
			return "", fmt.Errorf("translation paragraph is empty")
		}
		return text, nil
	default:
		paragraphs := ai.SplitParagraphs(article.ContentHtml)
		if len(paragraphs) == 0 && article.ContentText != "" {
			paragraphs = ai.SplitParagraphs(article.ContentText)
		}
		if params.ParagraphIndex < 0 || int(params.ParagraphIndex) >= len(paragraphs) {
			return "", fmt.Errorf("paragraph index out of range")
		}
		text := strings.TrimSpace(paragraphs[params.ParagraphIndex].Original)
		if text == "" {
			return "", fmt.Errorf("paragraph is empty")
		}
		return text, nil
	}
}

func validateLayer(layer string) error {
	switch layer {
	case "original", "translation":
		return nil
	default:
		return fmt.Errorf("invalid layer")
	}
}

func validateOffsets(start, end int32) error {
	if start < 0 || end < 0 {
		return fmt.Errorf("offsets must be non-negative")
	}
	if start >= end {
		return fmt.Errorf("text_start_offset must be less than text_end_offset")
	}
	return nil
}

func clampLimit(limit int32) int32 {
	if limit <= 0 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func clampOffset(offset int32) int32 {
	if offset < 0 {
		return 0
	}
	return offset
}

func noteOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func utf16Substring(text string, start, end int32) (string, error) {
	if start < 0 || end < 0 {
		return "", fmt.Errorf("offsets must be non-negative")
	}
	if start >= end {
		return "", fmt.Errorf("text_start_offset must be less than text_end_offset")
	}

	units := utf16.Encode([]rune(text))
	if int(end) > len(units) {
		return "", fmt.Errorf("offsets out of range")
	}
	return string(utf16.Decode(units[start:end])), nil
}

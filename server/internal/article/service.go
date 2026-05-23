package article

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/db/gen"
)

var errNotFound = errors.New("not found")
var errForbidden = errors.New("forbidden")

type ArticleService struct {
	pool           *pgxpool.Pool
	queries        *gen.Queries
	originalLoader func(context.Context, string) (OriginalContent, error)
}

func NewArticleService(pool *pgxpool.Pool) *ArticleService {
	return &ArticleService{pool: pool, queries: gen.New(pool), originalLoader: fetchOriginalContent}
}

type EnrichedArticle struct {
	ID              int64
	SourceID        int64
	Title           string
	TitleTranslated string
	Summary         string
	SourceTitle     string
	Link            string
	Language        string
	Author          pgtype.Text
	PublishedAt     pgtype.Timestamptz
	ContentText     string
	IsRead          bool
	IsStarred       bool
}

type ArticleReadCounts struct {
	Unread int64
	All    int64
	Read   int64
}

func (s *ArticleService) ListToday(ctx context.Context, userID int64) ([]gen.Article, error) {
	return s.queries.ListArticlesToday(ctx, userID)
}

func (s *ArticleService) ListTodayEnriched(ctx context.Context, userID int64, lang string, readFilter string) ([]EnrichedArticle, error) {
	rows, err := s.queries.ListArticlesTodayEnriched(ctx, gen.ListArticlesTodayEnrichedParams{
		UserID: userID, TargetLanguage: lang, ReadFilter: normalizeReadFilter(readFilter),
	})
	if err != nil {
		return nil, err
	}
	out := make([]EnrichedArticle, len(rows))
	for i, r := range rows {
		out[i] = EnrichedArticle{
			ID: r.ID, SourceID: r.SourceID, Title: r.Title,
			TitleTranslated: r.TitleTranslated, Summary: r.Summary,
			SourceTitle: r.SourceTitle, Link: r.Link, Language: r.Language,
			Author: r.Author, PublishedAt: r.PublishedAt, ContentText: r.ContentText,
			IsRead: r.IsRead, IsStarred: r.IsStarred,
		}
	}
	return out, nil
}

func (s *ArticleService) ListStream(ctx context.Context, userID int64, cursor *time.Time, limit int32) ([]gen.Article, error) {
	return s.queries.ListArticlesStream(ctx, gen.ListArticlesStreamParams{
		UserID:  userID,
		Column2: timestamptzOrNull(cursor),
		Limit:   clampLimit(limit),
	})
}

func (s *ArticleService) ListStreamEnriched(ctx context.Context, userID int64, cursor *time.Time, limit int32, lang string, readFilter string) ([]EnrichedArticle, error) {
	rows, err := s.queries.ListArticlesStreamEnriched(ctx, gen.ListArticlesStreamEnrichedParams{
		UserID: userID, Column2: timestamptzOrNull(cursor), TargetLanguage: lang, Limit: clampLimit(limit), ReadFilter: normalizeReadFilter(readFilter),
	})
	if err != nil {
		return nil, err
	}
	out := make([]EnrichedArticle, len(rows))
	for i, r := range rows {
		out[i] = EnrichedArticle{
			ID: r.ID, SourceID: r.SourceID, Title: r.Title,
			TitleTranslated: r.TitleTranslated, Summary: r.Summary,
			SourceTitle: r.SourceTitle, Link: r.Link, Language: r.Language,
			Author: r.Author, PublishedAt: r.PublishedAt, ContentText: r.ContentText,
			IsRead: r.IsRead, IsStarred: r.IsStarred,
		}
	}
	return out, nil
}

func (s *ArticleService) ListStarred(ctx context.Context, userID int64) ([]gen.Article, error) {
	return s.queries.ListArticlesStarred(ctx, userID)
}

func (s *ArticleService) ListStarredEnriched(ctx context.Context, userID int64, lang string) ([]EnrichedArticle, error) {
	rows, err := s.queries.ListArticlesStarredEnriched(ctx, gen.ListArticlesStarredEnrichedParams{
		UserID: userID, TargetLanguage: lang,
	})
	if err != nil {
		return nil, err
	}
	out := make([]EnrichedArticle, len(rows))
	for i, r := range rows {
		out[i] = EnrichedArticle{
			ID: r.ID, SourceID: r.SourceID, Title: r.Title,
			TitleTranslated: r.TitleTranslated, Summary: r.Summary,
			SourceTitle: r.SourceTitle, Link: r.Link, Language: r.Language,
			Author: r.Author, PublishedAt: r.PublishedAt, ContentText: r.ContentText,
			IsRead: r.IsRead, IsStarred: r.IsStarred,
		}
	}
	return out, nil
}

func (s *ArticleService) ListBySourceEnriched(ctx context.Context, userID, sourceID int64, lang string, readFilter string) ([]EnrichedArticle, error) {
	rows, err := s.queries.ListArticlesBySourceEnriched(ctx, gen.ListArticlesBySourceEnrichedParams{
		UserID: userID, SourceID: sourceID, TargetLanguage: lang, ReadFilter: normalizeReadFilter(readFilter),
	})
	if err != nil {
		return nil, err
	}
	out := make([]EnrichedArticle, len(rows))
	for i, r := range rows {
		out[i] = EnrichedArticle{
			ID: r.ID, SourceID: r.SourceID, Title: r.Title,
			TitleTranslated: r.TitleTranslated, Summary: r.Summary,
			SourceTitle: r.SourceTitle, Link: r.Link, Language: r.Language,
			Author: r.Author, PublishedAt: r.PublishedAt, ContentText: r.ContentText,
			IsRead: r.IsRead, IsStarred: r.IsStarred,
		}
	}
	return out, nil
}

func (s *ArticleService) ListBySource(ctx context.Context, userID, sourceID int64) ([]gen.Article, error) {
	_ = userID
	return s.queries.ListArticlesBySource(ctx, sourceID)
}

func (s *ArticleService) Search(ctx context.Context, userID int64, query string) ([]gen.Article, error) {
	rows, err := s.queries.SearchArticles(ctx, gen.SearchArticlesParams{
		UserID: userID,
		Q:      strings.TrimSpace(query),
	})
	if err != nil {
		return nil, err
	}

	items := make([]gen.Article, 0, len(rows))
	for _, row := range rows {
		items = append(items, gen.Article{
			ID:          row.ID,
			SourceID:    row.SourceID,
			Title:       row.Title,
			Link:        row.Link,
			Language:    row.Language,
			PublishedAt: row.PublishedAt,
		})
	}
	return items, nil
}

type ArticleWithSource struct {
	gen.Article
	SourceTitle string
}

func (s *ArticleService) GetByID(ctx context.Context, userID, articleID int64) (ArticleWithSource, error) {
	article, err := s.queries.GetArticleByID(ctx, articleID)
	if err != nil {
		return ArticleWithSource{}, errNotFound
	}

	source, err := s.queries.GetSourceByID(ctx, article.SourceID)
	if err != nil || source.UserID != userID {
		return ArticleWithSource{}, errNotFound
	}

	return ArticleWithSource{Article: article, SourceTitle: source.Title}, nil
}

func (s *ArticleService) SetRead(ctx context.Context, userID, articleID int64, isRead bool) error {
	return s.withTx(ctx, func(q *gen.Queries) error {
		_, err := q.SetArticleRead(ctx, gen.SetArticleReadParams{UserID: userID, ID: articleID, IsRead: isRead})
		if errors.Is(err, pgx.ErrNoRows) {
			return errForbidden
		}
		if err != nil {
			return err
		}
		return q.RecordStateChange(ctx, gen.RecordStateChangeParams{UserID: userID, ArticleID: articleID})
	})
}

func (s *ArticleService) SetStarred(ctx context.Context, userID, articleID int64, isStarred bool) error {
	return s.withTx(ctx, func(q *gen.Queries) error {
		_, err := q.SetArticleStarred(ctx, gen.SetArticleStarredParams{UserID: userID, ID: articleID, IsStarred: isStarred})
		if errors.Is(err, pgx.ErrNoRows) {
			return errForbidden
		}
		if err != nil {
			return err
		}
		return q.RecordStateChange(ctx, gen.RecordStateChangeParams{UserID: userID, ArticleID: articleID})
	})
}

func (s *ArticleService) UpdateProgress(ctx context.Context, userID, articleID int64, progress []byte) error {
	_, err := s.queries.UpdateReadingProgress(ctx, gen.UpdateReadingProgressParams{
		UserID:          userID,
		ID:              articleID,
		ReadingProgress: progress,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return errForbidden
	}
	return err
}

func (s *ArticleService) BatchSetRead(ctx context.Context, userID int64, scope string, isRead bool) ([]int64, error) {
	scope = strings.TrimSpace(scope)
	if scope == "tab:today" {
		return s.queries.BatchSetReadToday(ctx, gen.BatchSetReadTodayParams{UserID: userID, IsRead: isRead})
	}
	if scope == "tab:stream" || scope == "tab:all" {
		return s.queries.BatchSetReadStream(ctx, gen.BatchSetReadStreamParams{UserID: userID, IsRead: isRead})
	}
	if after, ok := strings.CutPrefix(scope, "source:"); ok {
		sourceID, err := strconv.ParseInt(after, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid scope: %s", scope)
		}
		return s.queries.BatchSetReadBySource(ctx, gen.BatchSetReadBySourceParams{UserID: userID, ID: sourceID, IsRead: isRead})
	}
	return nil, fmt.Errorf("unknown scope: %s", scope)
}

func (s *ArticleService) CountTodayByReadState(ctx context.Context, userID int64) (ArticleReadCounts, error) {
	row, err := s.queries.CountArticlesTodayByReadState(ctx, userID)
	if err != nil {
		return ArticleReadCounts{}, err
	}
	return ArticleReadCounts{Unread: row.UnreadCount, All: row.AllCount, Read: row.ReadCount}, nil
}

func (s *ArticleService) CountStreamByReadState(ctx context.Context, userID int64) (ArticleReadCounts, error) {
	row, err := s.queries.CountArticlesStreamByReadState(ctx, userID)
	if err != nil {
		return ArticleReadCounts{}, err
	}
	return ArticleReadCounts{Unread: row.UnreadCount, All: row.AllCount, Read: row.ReadCount}, nil
}

func (s *ArticleService) CountBySourceReadState(ctx context.Context, userID, sourceID int64) (ArticleReadCounts, error) {
	row, err := s.queries.CountArticlesBySourceReadState(ctx, gen.CountArticlesBySourceReadStateParams{
		UserID:   userID,
		SourceID: sourceID,
	})
	if err != nil {
		return ArticleReadCounts{}, err
	}
	return ArticleReadCounts{Unread: row.UnreadCount, All: row.AllCount, Read: row.ReadCount}, nil
}

func (s *ArticleService) ListChanges(ctx context.Context, userID int64, since time.Time) ([]gen.ListStateChangesSinceRow, error) {
	return s.queries.ListStateChangesSince(ctx, gen.ListStateChangesSinceParams{
		UserID:    userID,
		ChangedAt: pgtype.Timestamptz{Time: since.UTC(), Valid: true},
	})
}

func (s *ArticleService) GetState(ctx context.Context, userID, articleID int64) (gen.ArticleState, error) {
	return s.queries.GetArticleState(ctx, gen.GetArticleStateParams{UserID: userID, ArticleID: articleID})
}

func (s *ArticleService) LoadOriginal(ctx context.Context, userID, articleID int64) (OriginalContent, error) {
	article, err := s.GetByID(ctx, userID, articleID)
	if err != nil {
		return OriginalContent{}, err
	}
	content, err := s.originalLoader(ctx, article.Link)
	if err != nil {
		return OriginalContent{}, err
	}
	if _, err := s.queries.UpdateArticleContent(ctx, gen.UpdateArticleContentParams{
		ID:          article.ID,
		ContentHtml: content.ContentHTML,
		ContentText: content.ContentText,
	}); err != nil {
		return OriginalContent{}, err
	}
	return content, nil
}

func (s *ArticleService) withTx(ctx context.Context, fn func(*gen.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(gen.New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
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

func normalizeReadFilter(filter string) string {
	normalized := strings.TrimSpace(strings.ToLower(filter))
	switch normalized {
	case "unread", "read":
		return normalized
	default:
		return "all"
	}
}

func timestamptzOrNull(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

// Guest variants — content is owned by contentOwnerID, state is owned by stateOwnerID.

func (s *ArticleService) GuestListTodayEnriched(ctx context.Context, stateOwnerID, contentOwnerID int64, lang string, readFilter string) ([]EnrichedArticle, error) {
	rows, err := s.queries.GuestListArticlesTodayEnriched(ctx, gen.GuestListArticlesTodayEnrichedParams{
		TargetLanguage: lang,
		StateOwnerID:   stateOwnerID,
		ContentOwnerID: contentOwnerID,
		ReadFilter:     normalizeReadFilter(readFilter),
	})
	if err != nil {
		return nil, err
	}
	out := make([]EnrichedArticle, len(rows))
	for i, r := range rows {
		out[i] = EnrichedArticle{
			ID: r.ID, SourceID: r.SourceID, Title: r.Title,
			TitleTranslated: r.TitleTranslated, Summary: r.Summary,
			SourceTitle: r.SourceTitle, Link: r.Link, Language: r.Language,
			Author: r.Author, PublishedAt: r.PublishedAt, ContentText: r.ContentText,
			IsRead: r.IsRead, IsStarred: r.IsStarred,
		}
	}
	return out, nil
}

func (s *ArticleService) GuestListStreamEnriched(ctx context.Context, stateOwnerID, contentOwnerID int64, cursor *time.Time, limit int32, lang string, readFilter string) ([]EnrichedArticle, error) {
	rows, err := s.queries.GuestListArticlesStreamEnriched(ctx, gen.GuestListArticlesStreamEnrichedParams{
		Lim:            clampLimit(limit),
		TargetLanguage: lang,
		StateOwnerID:   stateOwnerID,
		ContentOwnerID: contentOwnerID,
		Cursor:         timestamptzOrNull(cursor),
		ReadFilter:     normalizeReadFilter(readFilter),
	})
	if err != nil {
		return nil, err
	}
	out := make([]EnrichedArticle, len(rows))
	for i, r := range rows {
		out[i] = EnrichedArticle{
			ID: r.ID, SourceID: r.SourceID, Title: r.Title,
			TitleTranslated: r.TitleTranslated, Summary: r.Summary,
			SourceTitle: r.SourceTitle, Link: r.Link, Language: r.Language,
			Author: r.Author, PublishedAt: r.PublishedAt, ContentText: r.ContentText,
			IsRead: r.IsRead, IsStarred: r.IsStarred,
		}
	}
	return out, nil
}

func (s *ArticleService) GuestListStarredEnriched(ctx context.Context, stateOwnerID, contentOwnerID int64, lang string) ([]EnrichedArticle, error) {
	rows, err := s.queries.GuestListArticlesStarredEnriched(ctx, gen.GuestListArticlesStarredEnrichedParams{
		TargetLanguage: lang,
		StateOwnerID:   stateOwnerID,
		ContentOwnerID: contentOwnerID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]EnrichedArticle, len(rows))
	for i, r := range rows {
		out[i] = EnrichedArticle{
			ID: r.ID, SourceID: r.SourceID, Title: r.Title,
			TitleTranslated: r.TitleTranslated, Summary: r.Summary,
			SourceTitle: r.SourceTitle, Link: r.Link, Language: r.Language,
			Author: r.Author, PublishedAt: r.PublishedAt, ContentText: r.ContentText,
			IsRead: r.IsRead, IsStarred: r.IsStarred,
		}
	}
	return out, nil
}

func (s *ArticleService) GuestListBySourceEnriched(ctx context.Context, stateOwnerID, contentOwnerID, sourceID int64, lang string, readFilter string) ([]EnrichedArticle, error) {
	rows, err := s.queries.GuestListArticlesBySourceEnriched(ctx, gen.GuestListArticlesBySourceEnrichedParams{
		TargetLanguage: lang,
		StateOwnerID:   stateOwnerID,
		ContentOwnerID: contentOwnerID,
		SourceID:       sourceID,
		ReadFilter:     normalizeReadFilter(readFilter),
	})
	if err != nil {
		return nil, err
	}
	out := make([]EnrichedArticle, len(rows))
	for i, r := range rows {
		out[i] = EnrichedArticle{
			ID: r.ID, SourceID: r.SourceID, Title: r.Title,
			TitleTranslated: r.TitleTranslated, Summary: r.Summary,
			SourceTitle: r.SourceTitle, Link: r.Link, Language: r.Language,
			Author: r.Author, PublishedAt: r.PublishedAt, ContentText: r.ContentText,
			IsRead: r.IsRead, IsStarred: r.IsStarred,
		}
	}
	return out, nil
}

func (s *ArticleService) GuestSearch(ctx context.Context, contentOwnerID int64, query string) ([]gen.GuestSearchArticlesRow, error) {
	return s.queries.GuestSearchArticles(ctx, gen.GuestSearchArticlesParams{
		ContentOwnerID: contentOwnerID,
		Q:              strings.TrimSpace(query),
	})
}

func (s *ArticleService) GuestGetByID(ctx context.Context, contentOwnerID, articleID int64) (ArticleWithSource, error) {
	article, err := s.queries.GetArticleByID(ctx, articleID)
	if err != nil {
		return ArticleWithSource{}, errNotFound
	}

	source, err := s.queries.GetSourceByID(ctx, article.SourceID)
	if err != nil || source.UserID != contentOwnerID {
		return ArticleWithSource{}, errNotFound
	}

	return ArticleWithSource{Article: article, SourceTitle: source.Title}, nil
}

func (s *ArticleService) GuestSetRead(ctx context.Context, stateOwnerID, contentOwnerID, articleID int64, isRead bool) error {
	return s.withTx(ctx, func(q *gen.Queries) error {
		_, err := q.GuestSetArticleRead(ctx, gen.GuestSetArticleReadParams{
			StateOwnerID:   stateOwnerID,
			IsRead:         isRead,
			ArticleID:      articleID,
			ContentOwnerID: contentOwnerID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return errForbidden
		}
		if err != nil {
			return err
		}
		return q.RecordStateChange(ctx, gen.RecordStateChangeParams{UserID: stateOwnerID, ArticleID: articleID})
	})
}

func (s *ArticleService) GuestSetStarred(ctx context.Context, stateOwnerID, contentOwnerID, articleID int64, isStarred bool) error {
	return s.withTx(ctx, func(q *gen.Queries) error {
		_, err := q.GuestSetArticleStarred(ctx, gen.GuestSetArticleStarredParams{
			StateOwnerID:   stateOwnerID,
			IsStarred:      isStarred,
			ArticleID:      articleID,
			ContentOwnerID: contentOwnerID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return errForbidden
		}
		if err != nil {
			return err
		}
		return q.RecordStateChange(ctx, gen.RecordStateChangeParams{UserID: stateOwnerID, ArticleID: articleID})
	})
}

func (s *ArticleService) GuestUpdateProgress(ctx context.Context, stateOwnerID, contentOwnerID, articleID int64, progress []byte) error {
	_, err := s.queries.GuestUpdateReadingProgress(ctx, gen.GuestUpdateReadingProgressParams{
		StateOwnerID:    stateOwnerID,
		ReadingProgress: progress,
		ArticleID:       articleID,
		ContentOwnerID:  contentOwnerID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return errForbidden
	}
	return err
}

func (s *ArticleService) GuestBatchSetRead(ctx context.Context, stateOwnerID, contentOwnerID int64, scope string, isRead bool) ([]int64, error) {
	scope = strings.TrimSpace(scope)
	if scope == "tab:today" {
		return s.queries.GuestBatchSetReadToday(ctx, gen.GuestBatchSetReadTodayParams{
			StateOwnerID:   stateOwnerID,
			IsRead:         isRead,
			ContentOwnerID: contentOwnerID,
		})
	}
	if scope == "tab:stream" || scope == "tab:all" {
		return s.queries.GuestBatchSetReadStream(ctx, gen.GuestBatchSetReadStreamParams{
			StateOwnerID:   stateOwnerID,
			IsRead:         isRead,
			ContentOwnerID: contentOwnerID,
		})
	}
	if after, ok := strings.CutPrefix(scope, "source:"); ok {
		sourceID, err := strconv.ParseInt(after, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid scope: %s", scope)
		}
		return s.queries.GuestBatchSetReadBySource(ctx, gen.GuestBatchSetReadBySourceParams{
			StateOwnerID:   stateOwnerID,
			IsRead:         isRead,
			SourceID:       sourceID,
			ContentOwnerID: contentOwnerID,
		})
	}
	return nil, fmt.Errorf("unknown scope: %s", scope)
}

func (s *ArticleService) GuestCountTodayByReadState(ctx context.Context, stateOwnerID, contentOwnerID int64) (ArticleReadCounts, error) {
	row, err := s.queries.GuestCountTodayByReadState(ctx, gen.GuestCountTodayByReadStateParams{
		StateOwnerID:   stateOwnerID,
		ContentOwnerID: contentOwnerID,
	})
	if err != nil {
		return ArticleReadCounts{}, err
	}
	return ArticleReadCounts{Unread: row.UnreadCount, All: row.AllCount, Read: row.ReadCount}, nil
}

func (s *ArticleService) GuestCountStreamByReadState(ctx context.Context, stateOwnerID, contentOwnerID int64) (ArticleReadCounts, error) {
	row, err := s.queries.GuestCountStreamByReadState(ctx, gen.GuestCountStreamByReadStateParams{
		StateOwnerID:   stateOwnerID,
		ContentOwnerID: contentOwnerID,
	})
	if err != nil {
		return ArticleReadCounts{}, err
	}
	return ArticleReadCounts{Unread: row.UnreadCount, All: row.AllCount, Read: row.ReadCount}, nil
}

func (s *ArticleService) GuestCountBySourceReadState(ctx context.Context, stateOwnerID, contentOwnerID, sourceID int64) (ArticleReadCounts, error) {
	row, err := s.queries.GuestCountBySourceReadState(ctx, gen.GuestCountBySourceReadStateParams{
		ContentOwnerID: contentOwnerID,
		StateOwnerID:   stateOwnerID,
		SourceID:       sourceID,
	})
	if err != nil {
		return ArticleReadCounts{}, err
	}
	return ArticleReadCounts{Unread: row.UnreadCount, All: row.AllCount, Read: row.ReadCount}, nil
}

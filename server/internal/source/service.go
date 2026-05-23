package source

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/db/gen"
	"github.com/razeencheng/xreader/internal/ai"
)

// ErrSourceNotFound is returned when a source does not exist or does not belong to the user.
var ErrSourceNotFound = errors.New("source not found")

type SourceService struct {
	pool     *pgxpool.Pool
	queries  *gen.Queries
	adapters map[string]SourceAdapter
	aiClient ai.AIClient
}

type SourceListItem struct {
	ID               int64      `json:"id"`
	Title            string     `json:"title"`
	Url              string     `json:"url"`
	Category         string     `json:"category"`
	IconURL          *string    `json:"icon_url,omitempty"`
	LastFetchedAt    *time.Time `json:"last_fetched_at"`
	LastSuccessAt    *time.Time `json:"last_success_at"`
	ConsecutiveFails int32      `json:"consecutive_fails"`
	Health           string     `json:"health"`
	UnreadCount      int64      `json:"unread_count"`
}

func NewSourceService(pool *pgxpool.Pool, adapters ...SourceAdapter) *SourceService {
	m := make(map[string]SourceAdapter)
	for _, a := range adapters {
		m[a.Kind()] = a
	}
	return &SourceService{pool: pool, queries: gen.New(pool), adapters: m}
}

func (s *SourceService) SetAIClient(client ai.AIClient) {
	s.aiClient = client
}

func (s *SourceService) List(ctx context.Context, userID int64) ([]SourceListItem, error) {
	return s.listWithStateOwner(ctx, userID, userID)
}

// GuestList returns the source list owned by contentOwnerID (the admin) but
// with unread counts computed against stateOwnerID (the guest), so guests see
// their own read state rather than the admin's.
func (s *SourceService) GuestList(ctx context.Context, contentOwnerID, stateOwnerID int64) ([]SourceListItem, error) {
	return s.listWithStateOwner(ctx, contentOwnerID, stateOwnerID)
}

func (s *SourceService) listWithStateOwner(ctx context.Context, contentOwnerID, stateOwnerID int64) ([]SourceListItem, error) {
	sources, err := s.queries.ListSourcesByUser(ctx, contentOwnerID)
	if err != nil {
		return nil, err
	}

	counts, err := s.listUnreadCountsFor(ctx, contentOwnerID, stateOwnerID)
	if err != nil {
		return nil, err
	}

	items := make([]SourceListItem, 0, len(sources))
	for _, source := range sources {
		items = append(items, SourceListItem{
			ID:               source.ID,
			Title:            source.Title,
			Url:              source.Url,
			Category:         defaultCategory(source.Category),
			IconURL:          textPtr(source.IconUrl),
			LastFetchedAt:    timestamptzPtr(source.LastFetchedAt),
			LastSuccessAt:    timestamptzPtr(source.LastSuccessAt),
			ConsecutiveFails: source.ConsecutiveFails,
			Health:           source.Health,
			UnreadCount:      counts[source.ID],
		})
	}

	return items, nil
}

func (s *SourceService) Create(ctx context.Context, userID int64, rawURL string, category string) (gen.Source, error) {
	if category == "" {
		category = "General"
	}
	adapter, ok := s.adapters["rss"]
	if !ok {
		return gen.Source{}, fmt.Errorf("no adapter for kind rss")
	}

	discovered, err := discoverFeed(ctx, rawURL, adapter)
	if err != nil {
		return gen.Source{}, fmt.Errorf("discover feed: %w", err)
	}

	normalized, err := Normalize(discovered.URL)
	if err != nil {
		return gen.Source{}, fmt.Errorf("invalid URL: %w", err)
	}

	title := discovered.Metadata.Title
	if title == "" {
		title = discovered.URL
	}

	params := gen.CreateSourceParams{
		UserID:        userID,
		Kind:          "rss",
		Url:           discovered.URL,
		NormalizedUrl: normalized,
		Title:         title,
		IconUrl:       textOrNull(discovered.Metadata.IconURL),
		LanguageHint:  textOrNull(discovered.Metadata.LanguageHint),
		Health:        "unknown",
		Category:      category,
	}
	src, err := s.queries.CreateSource(ctx, params)
	if err == nil {
		return src, nil
	}
	if !isUniqueViolation(err) {
		return gen.Source{}, err
	}

	restored, restoreErr := s.queries.RestoreSourceByUserAndNormalizedURL(ctx, gen.RestoreSourceByUserAndNormalizedURLParams{
		UserID:        userID,
		NormalizedUrl: normalized,
		Url:           discovered.URL,
		Title:         title,
		IconUrl:       params.IconUrl,
		LanguageHint:  params.LanguageHint,
		Category:      category,
	})
	if restoreErr != nil {
		if errors.Is(restoreErr, pgx.ErrNoRows) {
			return gen.Source{}, err
		}
		return gen.Source{}, fmt.Errorf("restore source: %w", restoreErr)
	}
	return restored, nil
}

func (s *SourceService) Rename(ctx context.Context, userID int64, sourceID int64, title string) error {
	src, err := s.queries.GetSourceByID(ctx, sourceID)
	if err != nil || src.UserID != userID {
		return ErrSourceNotFound
	}
	return s.queries.UpdateSourceTitle(ctx, gen.UpdateSourceTitleParams{ID: sourceID, Title: title})
}

func (s *SourceService) UpdateCategory(ctx context.Context, userID int64, sourceID int64, category string) error {
	src, err := s.queries.GetSourceByID(ctx, sourceID)
	if err != nil || src.UserID != userID {
		return ErrSourceNotFound
	}
	if category == "" {
		category = "General"
	}
	return s.queries.UpdateSourceCategory(ctx, gen.UpdateSourceCategoryParams{ID: sourceID, Category: category})
}

func (s *SourceService) Delete(ctx context.Context, userID int64, sourceID int64) error {
	src, err := s.queries.GetSourceByID(ctx, sourceID)
	if err != nil || src.UserID != userID {
		return ErrSourceNotFound
	}
	return s.queries.SoftDeleteSource(ctx, sourceID)
}

func (s *SourceService) Refresh(ctx context.Context, userID int64, sourceID int64) (int, error) {
	src, err := s.queries.GetSourceByID(ctx, sourceID)
	if err != nil || src.UserID != userID {
		return 0, ErrSourceNotFound
	}

	adapter, ok := s.adapters[src.Kind]
	if !ok {
		return 0, fmt.Errorf("no adapter for kind %s", src.Kind)
	}

	inserted, articleIDs, err := runFetch(ctx, s.queries, adapter, src)
	if err != nil {
		return inserted, err
	}
	s.runEagerAI(ctx, articleIDs)
	return inserted, nil
}

func (s *SourceService) runEagerAI(ctx context.Context, articleIDs []int64) {
	if s.aiClient == nil || len(articleIDs) == 0 {
		return
	}

	targetLanguages, err := s.queries.ListDistinctNativeLanguages(ctx)
	if err != nil {
		log.Printf("source: list native languages for eager AI: %v", err)
		return
	}

	for _, articleID := range articleIDs {
		for _, targetLang := range targetLanguages {
			job := ai.NewEagerJob(s.pool, s.aiClient, articleID, targetLang)
			if err := job.Run(ctx); err != nil {
				log.Printf("source: eager AI for article %d (%s): %v", articleID, targetLang, err)
			}
		}
	}
}

func (s *SourceService) listUnreadCounts(ctx context.Context, userID int64) (map[int64]int64, error) {
	return s.listUnreadCountsFor(ctx, userID, userID)
}

// listUnreadCountsFor returns unread counts for sources owned by contentOwnerID,
// where read state is looked up for stateOwnerID. This allows guests (stateOwnerID)
// to see their own read progress against the admin's source list (contentOwnerID).
func (s *SourceService) listUnreadCountsFor(ctx context.Context, contentOwnerID, stateOwnerID int64) (map[int64]int64, error) {
	const query = `
		SELECT s.id, COUNT(a.id) FILTER (WHERE st.is_read IS NULL OR st.is_read = false) AS unread_count
		FROM sources s
		LEFT JOIN articles a ON a.source_id = s.id
		LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = $2
		WHERE s.user_id = $1 AND s.deleted_at IS NULL
		GROUP BY s.id
	`

	rows, err := s.pool.Query(ctx, query, contentOwnerID, stateOwnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[int64]int64)
	for rows.Next() {
		var sourceID int64
		var unreadCount int64
		if err := rows.Scan(&sourceID, &unreadCount); err != nil {
			return nil, err
		}
		counts[sourceID] = unreadCount
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return counts, nil
}

func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func textPtr(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}

	trimmed := strings.TrimSpace(value.String)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func timestamptzPtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}

	t := value.Time
	return &t
}

func defaultCategory(category string) string {
	trimmed := strings.TrimSpace(category)
	if trimmed == "" {
		return "General"
	}
	return trimmed
}

func runFetch(ctx context.Context, queries *gen.Queries, adapter SourceAdapter, src gen.Source) (inserted int, articleIDs []int64, err error) {
	isInitialFetch := !src.LastSuccessAt.Valid
	adapterSrc := Source{
		ID:            src.ID,
		URL:           src.Url,
		NormalizedURL: src.NormalizedUrl,
		Title:         src.Title,
		Kind:          src.Kind,
	}

	items, fetchErr := adapter.Fetch(ctx, adapterSrc)
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}

	if fetchErr != nil {
		fails := src.ConsecutiveFails + 1
		health := "warn"
		if fails >= 6 {
			health = "fail"
		}
		_ = queries.UpdateSourceFetchStatus(ctx, gen.UpdateSourceFetchStatusParams{
			ID:               src.ID,
			LastFetchedAt:    now,
			LastSuccessAt:    src.LastSuccessAt,
			ConsecutiveFails: fails,
			Health:           health,
		})
		return 0, nil, fmt.Errorf("fetch source %d: %w", src.ID, fetchErr)
	}

	for _, item := range items {
		normalizedLink, normErr := Normalize(item.Link)
		if normErr != nil {
			continue
		}

		lang := item.LanguageHint
		if lang == "" {
			lang = "unknown"
		}

		article, upsertErr := queries.UpsertArticle(ctx, gen.UpsertArticleParams{
			SourceID:       src.ID,
			ExternalID:     item.ExternalID,
			Link:           item.Link,
			NormalizedLink: normalizedLink,
			Title:          item.Title,
			Language:       lang,
			ContentHtml:    item.ContentHTML,
			ContentText:    stripHTMLText(item.ContentHTML),
			Author:         pgtype.Text{},
			PublishedAt:    pgtype.Timestamptz{Time: item.PublishedAt, Valid: true},
			FetchedAt:      now,
		})
		if upsertErr != nil {
			if errors.Is(upsertErr, pgx.ErrNoRows) {
				continue
			}
			continue
		}
		articleIDs = append(articleIDs, article.ID)
		inserted++
	}

	if isInitialFetch {
		if _, markErr := queries.MarkInitialSourceBacklogRead(ctx, src.ID); markErr != nil {
			return inserted, articleIDs, fmt.Errorf("mark initial backlog read for source %d: %w", src.ID, markErr)
		}
	}

	if updateErr := queries.UpdateSourceFetchStatus(ctx, gen.UpdateSourceFetchStatusParams{
		ID:               src.ID,
		LastFetchedAt:    now,
		LastSuccessAt:    now,
		ConsecutiveFails: 0,
		Health:           "ok",
	}); updateErr != nil {
		return inserted, articleIDs, fmt.Errorf("update source %d fetch status: %w", src.ID, updateErr)
	}

	return inserted, articleIDs, nil
}

func stripHTMLText(html string) string {
	var b strings.Builder
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

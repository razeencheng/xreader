package sync

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
	"github.com/razeencheng/xreader/internal/source"
)

type FetchJob struct {
	pool    *pgxpool.Pool
	queries *gen.Queries
	adapter source.SourceAdapter
}

func NewFetchJob(pool *pgxpool.Pool, adapter source.SourceAdapter) *FetchJob {
	return &FetchJob{pool: pool, queries: gen.New(pool), adapter: adapter}
}

func (j *FetchJob) Run(ctx context.Context, src gen.Source) (inserted int, articleIDs []int64, err error) {
	isInitialFetch := !src.LastSuccessAt.Valid
	adapterSrc := source.Source{
		ID:            src.ID,
		URL:           src.Url,
		NormalizedURL: src.NormalizedUrl,
		Title:         src.Title,
		Kind:          src.Kind,
	}

	items, fetchErr := j.adapter.Fetch(ctx, adapterSrc)
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}

	if fetchErr != nil {
		fails := src.ConsecutiveFails + 1
		health := "warn"
		if fails >= 6 {
			health = "fail"
		}
		if updateErr := j.queries.UpdateSourceFetchStatus(ctx, gen.UpdateSourceFetchStatusParams{
			ID:               src.ID,
			LastFetchedAt:    now,
			LastSuccessAt:    src.LastSuccessAt,
			ConsecutiveFails: fails,
			Health:           health,
		}); updateErr != nil {
			log.Printf("fetch source %d: update status after failure: %v", src.ID, updateErr)
		}
		return 0, nil, fmt.Errorf("fetch source %d: %w", src.ID, fetchErr)
	}

	for _, item := range items {
		normalizedLink, normErr := source.Normalize(item.Link)
		if normErr != nil {
			log.Printf("fetch source %d: skip article with bad link %q: %v", src.ID, item.Link, normErr)
			continue
		}

		lang := item.LanguageHint
		if lang == "" {
			lang = "unknown"
		}

		contentText := stripHTML(item.ContentHTML)
		article, upsertErr := j.queries.UpsertArticle(ctx, gen.UpsertArticleParams{
			SourceID:       src.ID,
			ExternalID:     item.ExternalID,
			Link:           item.Link,
			NormalizedLink: normalizedLink,
			Title:          item.Title,
			Language:       lang,
			ContentHtml:    item.ContentHTML,
			ContentText:    contentText,
			PublishedAt:    pgtype.Timestamptz{Time: item.PublishedAt, Valid: true},
			FetchedAt:      now,
		})
		if upsertErr != nil {
			if errors.Is(upsertErr, pgx.ErrNoRows) {
				continue
			}
			log.Printf("fetch source %d: upsert article %q: %v", src.ID, item.Title, upsertErr)
			continue
		}
		articleIDs = append(articleIDs, article.ID)
		inserted++
	}

	if isInitialFetch {
		if _, markErr := j.queries.MarkInitialSourceBacklogRead(ctx, src.ID); markErr != nil {
			return inserted, articleIDs, fmt.Errorf("mark initial backlog read for source %d: %w", src.ID, markErr)
		}
	}

	if updateErr := j.queries.UpdateSourceFetchStatus(ctx, gen.UpdateSourceFetchStatusParams{
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

func stripHTML(html string) string {
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

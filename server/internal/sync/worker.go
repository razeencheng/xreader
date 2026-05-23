package sync

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/db/gen"
	"github.com/razeencheng/xreader/internal/ai"
	"github.com/razeencheng/xreader/internal/guest"
	"github.com/razeencheng/xreader/internal/source"
)

type Worker struct {
	pool        *pgxpool.Pool
	queries     *gen.Queries
	job         *FetchJob
	aiClient    ai.AIClient
	interval    time.Duration
	maxWorkers  int
	retranslate *ai.RetranslateQueue
	lastCatchUp time.Time // set/read only by the single catchUpAI goroutine
}

func NewWorker(pool *pgxpool.Pool, adapter source.SourceAdapter, aiClient ai.AIClient, retranslate *ai.RetranslateQueue) *Worker {
	return &Worker{
		pool:        pool,
		queries:     gen.New(pool),
		job:         NewFetchJob(pool, adapter),
		aiClient:    aiClient,
		interval:    60 * time.Second,
		maxWorkers:  8,
		retranslate: retranslate,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	log.Println("worker: starting fetch loop")
	go w.catchUpAI(ctx)
	go w.retranslateLoop(ctx)
	go w.guestCleanupLoop(ctx)
	w.tick(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("worker: shutting down")
			return ctx.Err()
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	sources, err := w.queries.ListSourcesDueForFetch(ctx)
	if err != nil {
		log.Printf("worker: list sources: %v", err)
		return
	}
	if len(sources) == 0 {
		return
	}

	log.Printf("worker: fetching %d sources", len(sources))

	var targetLanguages []string
	if w.aiClient != nil {
		targetLanguages, err = w.queries.ListDistinctNativeLanguages(ctx)
		if err != nil {
			log.Printf("worker: list native languages for eager AI: %v", err)
			targetLanguages = nil
		}
	}

	sem := make(chan struct{}, w.maxWorkers)
	var wg sync.WaitGroup

	for _, src := range sources {
		sem <- struct{}{}
		wg.Add(1)
		go func(s gen.Source) {
			defer wg.Done()
			defer func() { <-sem }()

			inserted, articleIDs, err := w.job.Run(ctx, s)
			if err != nil {
				log.Printf("worker: source %d (%s): %v", s.ID, s.Title, err)
				return
			}
			if inserted > 0 {
				log.Printf("worker: source %d (%s): %d new articles", s.ID, s.Title, inserted)
			}

			if w.aiClient != nil && len(targetLanguages) > 0 {
				for _, aid := range articleIDs {
					for _, targetLang := range targetLanguages {
						job := ai.NewEagerJob(w.pool, w.aiClient, aid, targetLang)
						if err := job.Run(ctx); err != nil {
							log.Printf("worker: eager AI for article %d (%s): %v", aid, targetLang, err)
						}
					}
				}
			}
		}(src)
	}

	wg.Wait()
}

const catchUpThrottle = 2 * time.Second
const catchUpInterval = 15 * time.Minute

func (w *Worker) catchUpAI(ctx context.Context) {
	if w.aiClient == nil {
		return
	}
	w.runCatchUp(ctx) // run once immediately (preserve startup behavior)
	ticker := time.NewTicker(catchUpInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runCatchUp(ctx)
		}
	}
}

func (w *Worker) runCatchUp(ctx context.Context) {
	if w.aiClient == nil {
		return
	}
	// In-memory guard: skip if catch-up itself ran within the interval. This
	// replaces the old "any article_ai updated in last 10 min" DB gate, which
	// on an active site suppressed catch-up indefinitely. lastCatchUp is set
	// before doing work so a re-entrant/rapid call skips.
	if !w.lastCatchUp.IsZero() && time.Since(w.lastCatchUp) < catchUpInterval {
		log.Println("worker: AI catch-up skipped (ran recently)")
		return
	}
	w.lastCatchUp = time.Now()

	languages, err := w.queries.ListDistinctNativeLanguages(ctx)
	if err != nil || len(languages) == 0 {
		return
	}

	for _, lang := range languages {
		ids, err := w.queries.ListArticlesMissingAI(ctx, gen.ListArticlesMissingAIParams{
			TargetLanguage: lang,
			Limit:          200,
		})
		if err != nil || len(ids) == 0 {
			continue
		}
		log.Printf("worker: AI catch-up: %d articles for %s (throttle %v)", len(ids), lang, catchUpThrottle)
		for _, id := range ids {
			if ctx.Err() != nil {
				return
			}
			job := ai.NewEagerJob(w.pool, w.aiClient, id, lang)
			if err := job.Run(ctx); err != nil {
				log.Printf("worker: AI catch-up article %d (%s): %v", id, lang, err)
			}
			time.Sleep(catchUpThrottle)
		}
	}
	log.Println("worker: AI catch-up complete")
}

// retranslateLoop drains on-demand title-translation jobs enqueued by the
// article list handler, running the existing eager pipeline per job with the
// same throttle as catch-up so we never storm the AI provider.
func (w *Worker) retranslateLoop(ctx context.Context) {
	if w.aiClient == nil || w.retranslate == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-w.retranslate.Jobs():
			w.runRetranslate(ctx, job)
			select {
			case <-ctx.Done():
				return
			case <-time.After(catchUpThrottle):
			}
		}
	}
}

// runRetranslate processes one queued job. Two deferred calls run LIFO at
// return/panic: Complete (registered last) runs first and always releases the
// in-flight de-dup reservation so the article can be re-enqueued by a future
// list view; the recover closure (registered first) runs last and catches any
// panic from EagerJob.Run — recover works from any deferred call of the
// panicking frame — so one bad article cannot kill the long-lived consumer.
func (w *Worker) runRetranslate(ctx context.Context, job ai.RetranslateJob) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("worker: on-demand retranslate panic for article %d (%s): %v",
				job.ArticleID, job.TargetLang, r)
		}
	}()
	defer w.retranslate.Complete(job)
	eager := ai.NewEagerJob(w.pool, w.aiClient, job.ArticleID, job.TargetLang)
	if err := eager.Run(ctx); err != nil {
		log.Printf("worker: on-demand retranslate article %d (%s): %v",
			job.ArticleID, job.TargetLang, err)
	}
}

func (w *Worker) guestCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			guestSvc := guest.NewService(w.pool)
			cleaned, err := guestSvc.CleanupExpired(ctx)
			if err != nil {
				log.Printf("worker: guest cleanup error: %v", err)
			} else if cleaned > 0 {
				log.Printf("worker: cleaned up %d expired guest users", cleaned)
			}
		}
	}
}

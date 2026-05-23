package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/db/gen"
	"github.com/razeencheng/xreader/internal/ai"
)

func main() {
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	cfg, err := ai.NewSettingsService(ai.NewPostgresSettingsRepository(pool)).LoadResolved(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	client := ai.NewClient(cfg)
	queries := gen.New(pool)

	ctx := context.Background()
	targetLanguages, err := queries.ListDistinctNativeLanguages(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if len(targetLanguages) == 0 {
		log.Println("no native languages configured; nothing to backfill")
		return
	}

	rows, err := pool.Query(ctx, "SELECT id, title FROM articles ORDER BY id")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	type article struct {
		ID    int64
		Title string
	}
	var articles []article
	for rows.Next() {
		var a article
		if err := rows.Scan(&a.ID, &a.Title); err != nil {
			log.Fatal(err)
		}
		articles = append(articles, a)
	}

	fmt.Printf("Processing %d articles for %d target languages...\n", len(articles), len(targetLanguages))
	for _, a := range articles {
		for _, targetLang := range targetLanguages {
			job := ai.NewEagerJob(pool, client, a.ID, targetLang)
			if err := job.Run(ctx); err != nil {
				log.Printf("article %d (%s) target %s: %v", a.ID, a.Title, targetLang, err)
				continue
			}
			fmt.Printf("article %d target %s: done\n", a.ID, targetLang)
		}
	}
	fmt.Println("Backfill complete.")
}

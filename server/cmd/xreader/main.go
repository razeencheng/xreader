package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razeencheng/xreader/internal/admin"
	"github.com/razeencheng/xreader/internal/ai"
	"github.com/razeencheng/xreader/internal/platform"
	"github.com/razeencheng/xreader/internal/source"
	syncpkg "github.com/razeencheng/xreader/internal/sync"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "seed-admin" {
		runSeedAdmin(os.Args[2:])
		return
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL not set")
	}
	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" || sessionSecret == "change-me" {
		if os.Getenv("XREADER_DEV_MODE") != "true" {
			log.Fatal("SESSION_SECRET is missing or insecure. " +
				"Set a strong random value (e.g. `openssl rand -hex 32`). " +
				"To bypass in local development, set XREADER_DEV_MODE=true.")
		}
		if sessionSecret == "" {
			sessionSecret = "change-me"
		}
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	if err := runMigrations(dbURL); err != nil {
		log.Fatalf("migration failed: %v", err)
	}
	log.Println("migrations: up to date")

	// Check if setup is needed and generate/display setup token
	var adminCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM auth_allowlist").Scan(&adminCount); err != nil {
		log.Fatalf("check admin count: %v", err)
	}
	setupToken := ""
	if adminCount == 0 {
		setupToken = os.Getenv("SETUP_TOKEN")
		if setupToken == "" {
			b := make([]byte, 24)
			if _, err := rand.Read(b); err != nil {
				log.Fatalf("generate setup token: %v", err)
			}
			setupToken = hex.EncodeToString(b)
		}
		log.Printf("\n==================================================")
		log.Printf("  SETUP TOKEN: %s", setupToken)
		log.Printf("  Open http://localhost:%s/setup to complete setup", port)
		log.Printf("==================================================\n")
	}

	// Shared on-demand AI retranslate queue: produced by the article list
	// handler, consumed by the worker goroutine.
	retranslateQueue := ai.NewRetranslateQueue(512)

	// Start worker in background goroutine
	go func() {
		log.Println("worker: starting fetch loop")
		settings := ai.NewSettingsService(ai.NewPostgresSettingsRepository(pool))
		aiClient := ai.NewDynamicClient(settings)
		adapter := source.NewRSSAdapter()
		worker := syncpkg.NewWorker(pool, adapter, aiClient, retranslateQueue)
		if err := worker.Run(ctx); err != nil && err != context.Canceled {
			log.Printf("worker: %v", err)
		}
	}()

	// Start HTTP server
	router := platform.NewRouter(platform.RouterDeps{
		Pool:             pool,
		SessionSecret:    sessionSecret,
		StaticFS:         staticFS,
		SetupToken:       setupToken,
		RetranslateQueue: retranslateQueue,
	})

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		log.Printf("http: listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http: shutdown error: %v", err)
	}
}

func runSeedAdmin(args []string) {
	var username string
	for _, arg := range args {
		if strings.HasPrefix(arg, "--github-username=") {
			username = strings.TrimPrefix(arg, "--github-username=")
		}
	}
	if username == "" {
		fmt.Fprintf(os.Stderr, "Usage: xreader seed-admin --github-username=<username>\n")
		os.Exit(1)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	svc := admin.NewAllowlistService(pool)
	if err := svc.SeedAdmin(ctx, username); err != nil {
		log.Fatalf("seed admin: %v", err)
	}

	fmt.Printf("Seeded admin: %s\n", username)
}

func runMigrations(dbURL string) error {
	// Try /migrations first (Docker), then db/migrations (local dev)
	source := "file:///migrations"
	if _, err := os.Stat("/migrations"); os.IsNotExist(err) {
		source = "file://db/migrations"
	}
	m, err := migrate.New(source, dbURL)
	if err != nil {
		return fmt.Errorf("migration init: %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migration up: %w", err)
	}
	return nil
}

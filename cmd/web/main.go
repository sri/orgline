package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"

	"orgline/internal/db/migrate"
	"orgline/internal/db/sqlite"
	"orgline/internal/server"
)

func main() {
	ctx := context.Background()

	addr := envOrDefault("ORGLINE_ADDR", ":8080")
	dbPath := envOrDefault("ORGLINE_DB_PATH", "orgline.db")
	frontendDevURL := os.Getenv("ORGLINE_DEV_FRONTEND_URL")

	db, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		log.Fatal(err)
	}

	if err := migrate.Up(ctx, db); err != nil {
		_ = db.Close()
		log.Fatal(err)
	}

	if err := db.Close(); err != nil {
		log.Fatal(err)
	}

	cfg := server.Config{
		Addr:           addr,
		FrontendDevURL: frontendDevURL,
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("starting server on http://localhost%s", cfg.Addr)

	err = srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

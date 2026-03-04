package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"orgline/internal/db/migrate"
	"orgline/internal/db/sqlite"
	"orgline/internal/server"
)

func main() {
	ctx := context.Background()

	addrFlag := flag.String("addr", os.Getenv("ORGLINE_ADDR"), "HTTP listen address (for example, ':8080' or '127.0.0.1:8080'). Overrides -port when set.")
	portFlag := flag.Int("port", envIntOrDefault("ORGLINE_PORT", 8080), "HTTP listen port (used when -addr is empty).")
	dbPathFlag := flag.String("db-path", envOrDefault("ORGLINE_DB_PATH", "orgline.db"), "Path to SQLite database file.")
	readHeaderTimeoutFlag := flag.Duration("read-header-timeout", envDurationOrDefault("ORGLINE_READ_HEADER_TIMEOUT", 5*time.Second), "HTTP server read header timeout.")
	readTimeoutFlag := flag.Duration("read-timeout", envDurationOrDefault("ORGLINE_READ_TIMEOUT", 15*time.Second), "HTTP server read timeout.")
	writeTimeoutFlag := flag.Duration("write-timeout", envDurationOrDefault("ORGLINE_WRITE_TIMEOUT", 15*time.Second), "HTTP server write timeout.")
	idleTimeoutFlag := flag.Duration("idle-timeout", envDurationOrDefault("ORGLINE_IDLE_TIMEOUT", 60*time.Second), "HTTP server idle timeout.")
	devModeFlag := flag.Bool("dev", envBoolOrDefault("ORGLINE_DEV_MODE", false), "Enable development endpoints and auto-reload helpers.")
	devBuildIDFlag := flag.String("dev-build-id", envOrDefault("ORGLINE_DEV_BUILD_ID", ""), "Development build id used for browser auto-reload.")
	flag.Parse()

	addr, err := resolveAddr(*addrFlag, *portFlag)
	if err != nil {
		log.Fatal(err)
	}

	db, err := sqlite.Open(ctx, *dbPathFlag)
	if err != nil {
		log.Fatal(err)
	}

	if err := migrate.Up(ctx, db); err != nil {
		_ = db.Close()
		log.Fatal(err)
	}

	cfg := server.Config{
		Addr:              addr,
		DB:                db,
		DevMode:           *devModeFlag,
		DevBuildID:        *devBuildIDFlag,
		ReadHeaderTimeout: *readHeaderTimeoutFlag,
		ReadTimeout:       *readTimeoutFlag,
		WriteTimeout:      *writeTimeoutFlag,
		IdleTimeout:       *idleTimeoutFlag,
	}

	srv, err := server.New(cfg)
	if err != nil {
		_ = db.Close()
		log.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("close sqlite database: %v", err)
		}
	}()

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

func envIntOrDefault(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("invalid %s=%q, using %d", key, value, fallback)
		return fallback
	}

	return parsed
}

func envDurationOrDefault(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		log.Printf("invalid %s=%q, using %s", key, value, fallback)
		return fallback
	}

	return parsed
}

func envBoolOrDefault(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		log.Printf("invalid %s=%q, using %t", key, value, fallback)
		return fallback
	}

	return parsed
}

func resolveAddr(addr string, port int) (string, error) {
	if addr != "" {
		return addr, nil
	}

	if port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid port %d, expected 1-65535", port)
	}

	return fmt.Sprintf(":%d", port), nil
}

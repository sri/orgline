package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"orgline/internal/db/migrate"
	"orgline/internal/db/sqlite"
)

func TestHelloAPIHandler(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/hello", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	var response struct {
		Message string `json:"message"`
	}

	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got, want := response.Message, "Hello World"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestNewServerMissingDB(t *testing.T) {
	_, err := New(Config{
		Addr: ":0",
	})
	if err == nil {
		t.Fatal("expected error when DB is not configured")
	}
}

func TestNewServerInvalidDevURL(t *testing.T) {
	db := setupTestDB(t)

	_, err := New(Config{
		Addr:           ":0",
		DB:             db,
		FrontendDevURL: "://bad-url",
	})
	if err == nil {
		t.Fatal("expected error for invalid frontend dev url")
	}
}

func TestNewServerTimeoutDefaults(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	if got, want := srv.ReadHeaderTimeout, 5*time.Second; got != want {
		t.Fatalf("read header timeout = %s, want %s", got, want)
	}
	if got, want := srv.ReadTimeout, 15*time.Second; got != want {
		t.Fatalf("read timeout = %s, want %s", got, want)
	}
	if got, want := srv.WriteTimeout, 15*time.Second; got != want {
		t.Fatalf("write timeout = %s, want %s", got, want)
	}
	if got, want := srv.IdleTimeout, 60*time.Second; got != want {
		t.Fatalf("idle timeout = %s, want %s", got, want)
	}
}

func TestItemsAPIHandler(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	var response struct {
		Items []struct {
			UUID     string `json:"uuid"`
			Body     string `json:"body"`
			Children []struct {
				UUID string `json:"uuid"`
				Body string `json:"body"`
			} `json:"children"`
		} `json:"items"`
	}

	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response.Items) == 0 {
		t.Fatal("expected sample items")
	}

	if response.Items[0].UUID == "" || response.Items[0].Body == "" {
		t.Fatal("expected uuid and body on root item")
	}

	if len(response.Items) < 2 {
		t.Fatal("expected at least two root items")
	}

	if got, want := response.Items[0].Body, "Q1 Company Planning"; got != want {
		t.Fatalf("first root body = %q, want %q", got, want)
	}

	if got, want := response.Items[1].Body, "Product Launch Checklist"; got != want {
		t.Fatalf("second root body = %q, want %q", got, want)
	}
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := migrate.Up(ctx, db); err != nil {
		_ = db.Close()
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close sqlite db: %v", err)
		}
	})

	return db
}

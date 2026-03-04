package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHelloAPIHandler(t *testing.T) {
	srv, err := New(Config{Addr: ":0"})
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

func TestNewServerInvalidDevURL(t *testing.T) {
	_, err := New(Config{
		Addr:           ":0",
		FrontendDevURL: "://bad-url",
	})
	if err == nil {
		t.Fatal("expected error for invalid frontend dev url")
	}
}

func TestNewServerTimeoutDefaults(t *testing.T) {
	srv, err := New(Config{Addr: ":0"})
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

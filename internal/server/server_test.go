package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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

func TestFrontendRootHandler(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type = %q, expected text/html", ct)
	}

	if !strings.Contains(rec.Body.String(), "id=\"outline-root\"") {
		t.Fatal("expected outline root in HTML")
	}
}

func TestFrontendNotFound(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusNotFound; got != want {
		t.Fatalf("status = %d, want %d", got, want)
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
			IsOpen   bool   `json:"is_open"`
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

	if !response.Items[0].IsOpen || !response.Items[1].IsOpen {
		t.Fatal("expected seeded root items to be open")
	}
}

func TestUpdateItemBodyAPIHandler(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	uuid := "11111111-1111-1111-1111-111111111112"
	body := `{"body":"Finalize hiring plan (updated)"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/items/"+uuid, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusNoContent; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	const selectStatement = `SELECT body FROM workflow_items WHERE uuid = ?`
	var updatedBody string
	if err := db.QueryRow(selectStatement, uuid).Scan(&updatedBody); err != nil {
		t.Fatalf("query updated item: %v", err)
	}
	if got, want := updatedBody, "Finalize hiring plan (updated)"; got != want {
		t.Fatalf("updated body = %q, want %q", got, want)
	}
}

func TestUpdateItemBodyAPIHandlerBadRequest(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	uuid := "11111111-1111-1111-1111-111111111112"
	req := httptest.NewRequest(http.MethodPatch, "/api/items/"+uuid, bytes.NewBufferString(`{"body":"   "}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusBadRequest; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func TestUpdateItemOpenStateAPIHandler(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	uuid := "11111111-1111-1111-1111-111111111111"
	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/items/"+uuid+"/open-state",
		bytes.NewBufferString(`{"is_open":false}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusNoContent; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	const selectStatement = `SELECT is_open FROM workflow_items WHERE uuid = ?`
	var isOpen int
	if err := db.QueryRow(selectStatement, uuid).Scan(&isOpen); err != nil {
		t.Fatalf("query updated open state: %v", err)
	}
	if isOpen != 0 {
		t.Fatalf("is_open = %d, want 0", isOpen)
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	recList := httptest.NewRecorder()
	srv.Handler.ServeHTTP(recList, reqList)

	if got, want := recList.Code, http.StatusOK; got != want {
		t.Fatalf("list status = %d, want %d", got, want)
	}

	var response struct {
		Items []struct {
			UUID   string `json:"uuid"`
			IsOpen bool   `json:"is_open"`
		} `json:"items"`
	}
	if err := json.NewDecoder(recList.Body).Decode(&response); err != nil {
		t.Fatalf("decode list response: %v", err)
	}

	found := false
	for _, item := range response.Items {
		if item.UUID == uuid {
			found = true
			if item.IsOpen {
				t.Fatal("expected item to remain closed after reload")
			}
		}
	}
	if !found {
		t.Fatalf("item %s not found in list response", uuid)
	}
}

func TestCreateItemAfterEnterAsSibling(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	currentUUID := "11111111-1111-1111-1111-111111111112"
	req := httptest.NewRequest(http.MethodPost, "/api/items/"+currentUUID+"/enter", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusCreated; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	var response struct {
		UUID string `json:"uuid"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if response.UUID == "" {
		t.Fatal("expected created uuid")
	}

	var parentUUID sql.NullString
	var childOrder int
	var body string
	var isOpen int
	const query = `
SELECT parent_uuid, child_order, body, is_open
FROM workflow_items
WHERE uuid = ?
`
	if err := db.QueryRow(query, response.UUID).Scan(&parentUUID, &childOrder, &body, &isOpen); err != nil {
		t.Fatalf("query created item: %v", err)
	}

	if !parentUUID.Valid || parentUUID.String != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("parent_uuid = %+v, want 1111...1111", parentUUID)
	}
	if childOrder != 2 {
		t.Fatalf("child_order = %d, want 2", childOrder)
	}
	if body != "" {
		t.Fatalf("body = %q, want empty", body)
	}
	if isOpen != 1 {
		t.Fatalf("is_open = %d, want 1", isOpen)
	}

	var movedSiblingOrder int
	if err := db.QueryRow(
		"SELECT child_order FROM workflow_items WHERE uuid = ?",
		"11111111-1111-1111-1111-111111111113",
	).Scan(&movedSiblingOrder); err != nil {
		t.Fatalf("query moved sibling: %v", err)
	}
	if movedSiblingOrder != 3 {
		t.Fatalf("moved sibling child_order = %d, want 3", movedSiblingOrder)
	}
}

func TestCreateItemAfterEnterAsFirstChildWhenChildrenExist(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	currentUUID := "11111111-1111-1111-1111-111111111111"
	req := httptest.NewRequest(http.MethodPost, "/api/items/"+currentUUID+"/enter", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusCreated; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	var response struct {
		UUID string `json:"uuid"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if response.UUID == "" {
		t.Fatal("expected created uuid")
	}

	var parentUUID sql.NullString
	var childOrder int
	const query = `
SELECT parent_uuid, child_order
FROM workflow_items
WHERE uuid = ?
`
	if err := db.QueryRow(query, response.UUID).Scan(&parentUUID, &childOrder); err != nil {
		t.Fatalf("query created child item: %v", err)
	}

	if !parentUUID.Valid || parentUUID.String != currentUUID {
		t.Fatalf("parent_uuid = %+v, want %s", parentUUID, currentUUID)
	}
	if childOrder != 1 {
		t.Fatalf("child_order = %d, want 1", childOrder)
	}

	var previousFirstChildOrder int
	if err := db.QueryRow(
		"SELECT child_order FROM workflow_items WHERE uuid = ?",
		"11111111-1111-1111-1111-111111111112",
	).Scan(&previousFirstChildOrder); err != nil {
		t.Fatalf("query previous first child: %v", err)
	}
	if previousFirstChildOrder != 2 {
		t.Fatalf("previous first child order = %d, want 2", previousFirstChildOrder)
	}
}

func TestIndentItemAPIHandler(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	uuid := "11111111-1111-1111-1111-111111111113"
	req := httptest.NewRequest(http.MethodPost, "/api/items/"+uuid+"/indent", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusNoContent; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	var parentUUID sql.NullString
	var childOrder int
	if err := db.QueryRow(
		"SELECT parent_uuid, child_order FROM workflow_items WHERE uuid = ?",
		uuid,
	).Scan(&parentUUID, &childOrder); err != nil {
		t.Fatalf("query moved item: %v", err)
	}

	if !parentUUID.Valid || parentUUID.String != "11111111-1111-1111-1111-111111111112" {
		t.Fatalf("parent_uuid = %+v, want previous sibling", parentUUID)
	}
	if childOrder != 1 {
		t.Fatalf("child_order = %d, want 1", childOrder)
	}
}

func TestOutdentItemAPIHandler(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	uuid := "11111111-1111-1111-1111-111111111114"
	req := httptest.NewRequest(http.MethodPost, "/api/items/"+uuid+"/outdent", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusNoContent; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	var parentUUID sql.NullString
	var childOrder int
	if err := db.QueryRow(
		"SELECT parent_uuid, child_order FROM workflow_items WHERE uuid = ?",
		uuid,
	).Scan(&parentUUID, &childOrder); err != nil {
		t.Fatalf("query moved item: %v", err)
	}

	if !parentUUID.Valid || parentUUID.String != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("parent_uuid = %+v, want root parent", parentUUID)
	}
	if childOrder != 3 {
		t.Fatalf("child_order = %d, want 3", childOrder)
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

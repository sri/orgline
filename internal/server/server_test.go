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
	"orgline/internal/workflow"
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

func TestFrontendRootHandlerIncludesDevBootstrapWhenEnabled(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr:       ":0",
		DB:         db,
		DevMode:    true,
		DevBuildID: "dev-build-123",
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

	body := rec.Body.String()
	if !strings.Contains(body, "__ORGLINE_DEV_MODE=true") {
		t.Fatal("expected dev mode bootstrap in HTML")
	}
	if !strings.Contains(body, "__ORGLINE_DEV_BUILD_ID=\"dev-build-123\"") {
		t.Fatal("expected dev build id bootstrap in HTML")
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

func TestDevBuildEndpointEnabledOnlyInDevMode(t *testing.T) {
	db := setupTestDB(t)

	devServer, err := New(Config{
		Addr:       ":0",
		DB:         db,
		DevMode:    true,
		DevBuildID: "build-xyz",
	})
	if err != nil {
		t.Fatalf("new dev server: %v", err)
	}

	devReq := httptest.NewRequest(http.MethodGet, "/api/dev/build", nil)
	devRec := httptest.NewRecorder()
	devServer.Handler.ServeHTTP(devRec, devReq)

	if got, want := devRec.Code, http.StatusOK; got != want {
		t.Fatalf("dev endpoint status = %d, want %d", got, want)
	}

	var devResponse struct {
		BuildID string `json:"build_id"`
	}
	if err := json.NewDecoder(devRec.Body).Decode(&devResponse); err != nil {
		t.Fatalf("decode dev response: %v", err)
	}
	if got, want := devResponse.BuildID, "build-xyz"; got != want {
		t.Fatalf("build id = %q, want %q", got, want)
	}

	prodServer, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new prod server: %v", err)
	}

	prodReq := httptest.NewRequest(http.MethodGet, "/api/dev/build", nil)
	prodRec := httptest.NewRecorder()
	prodServer.Handler.ServeHTTP(prodRec, prodReq)
	if got, want := prodRec.Code, http.StatusNotFound; got != want {
		t.Fatalf("prod endpoint status = %d, want %d", got, want)
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
			UUID       string `json:"uuid"`
			Body       string `json:"body"`
			IsOpen     bool   `json:"is_open"`
			IsFavorite bool   `json:"is_favorite"`
			Children   []struct {
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
	if response.Items[0].IsFavorite || response.Items[1].IsFavorite {
		t.Fatal("expected seeded root items to be non-favorite by default")
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

func TestDeleteItemAPIHandler(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	uuid := "11111111-1111-1111-1111-111111111112"
	req := httptest.NewRequest(http.MethodDelete, "/api/items/"+uuid, nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusNoContent; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	var deletedCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM workflow_items WHERE uuid = ?", uuid).Scan(&deletedCount); err != nil {
		t.Fatalf("query deleted item count: %v", err)
	}
	if deletedCount != 0 {
		t.Fatalf("deleted item count = %d, want 0", deletedCount)
	}

	var siblingOrder int
	if err := db.QueryRow(
		"SELECT child_order FROM workflow_items WHERE uuid = ?",
		"11111111-1111-1111-1111-111111111113",
	).Scan(&siblingOrder); err != nil {
		t.Fatalf("query sibling after delete: %v", err)
	}
	if siblingOrder != 1 {
		t.Fatalf("sibling child_order = %d, want 1", siblingOrder)
	}
}

func TestDeleteItemAPIHandlerNotFound(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/items/not-a-real-id", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusNotFound; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func TestDeleteItemAPIHandlerRejectsDeletingLastItem(t *testing.T) {
	db := setupTestDB(t)

	if _, err := db.Exec(
		"DELETE FROM workflow_items WHERE uuid <> ?",
		"11111111-1111-1111-1111-111111111111",
	); err != nil {
		t.Fatalf("reduce to one item: %v", err)
	}

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/items/11111111-1111-1111-1111-111111111111", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusConflict; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM workflow_items").Scan(&count); err != nil {
		t.Fatalf("count items: %v", err)
	}
	if got, want := count, 1; got != want {
		t.Fatalf("item count = %d, want %d", got, want)
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

func TestUpdateItemFavoriteStateAPIHandler(t *testing.T) {
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
		"/api/items/"+uuid+"/favorite-state",
		bytes.NewBufferString(`{"is_favorite":true}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusNoContent; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	const selectStatement = `SELECT is_favorite FROM workflow_items WHERE uuid = ?`
	var isFavorite int
	if err := db.QueryRow(selectStatement, uuid).Scan(&isFavorite); err != nil {
		t.Fatalf("query updated favorite state: %v", err)
	}
	if isFavorite != 1 {
		t.Fatalf("is_favorite = %d, want 1", isFavorite)
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	recList := httptest.NewRecorder()
	srv.Handler.ServeHTTP(recList, reqList)

	if got, want := recList.Code, http.StatusOK; got != want {
		t.Fatalf("list status = %d, want %d", got, want)
	}

	var response struct {
		Items []struct {
			UUID       string `json:"uuid"`
			IsFavorite bool   `json:"is_favorite"`
		} `json:"items"`
	}
	if err := json.NewDecoder(recList.Body).Decode(&response); err != nil {
		t.Fatalf("decode list response: %v", err)
	}

	found := false
	for _, item := range response.Items {
		if item.UUID == uuid {
			found = true
			if !item.IsFavorite {
				t.Fatal("expected item to remain favorite after reload")
			}
		}
	}
	if !found {
		t.Fatalf("item %s not found in list response", uuid)
	}
}

func TestCreateRootItemAPIHandler(t *testing.T) {
	db := setupTestDB(t)

	if _, err := db.Exec("DELETE FROM workflow_items"); err != nil {
		t.Fatalf("delete all workflow items: %v", err)
	}

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/items", nil)
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
	if err := db.QueryRow(
		"SELECT parent_uuid, child_order, body FROM workflow_items WHERE uuid = ?",
		response.UUID,
	).Scan(&parentUUID, &childOrder, &body); err != nil {
		t.Fatalf("query created root item: %v", err)
	}

	if parentUUID.Valid {
		t.Fatalf("parent_uuid = %+v, want NULL for root item", parentUUID)
	}
	if childOrder != 1 {
		t.Fatalf("child_order = %d, want 1", childOrder)
	}
	if body != "" {
		t.Fatalf("body = %q, want empty", body)
	}
}

func TestCreateChildItemAPIHandler(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	t.Run("insert first child when children already exist", func(t *testing.T) {
		parentUUID := "11111111-1111-1111-1111-111111111111"
		req := httptest.NewRequest(http.MethodPost, "/api/items/"+parentUUID+"/child", nil)
		rec := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusCreated; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}

		var response struct {
			UUID string `json:"uuid"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatalf("decode create child response: %v", err)
		}
		if response.UUID == "" {
			t.Fatal("expected created child uuid")
		}

		var parent sql.NullString
		var order int
		if err := db.QueryRow(
			"SELECT parent_uuid, child_order FROM workflow_items WHERE uuid = ?",
			response.UUID,
		).Scan(&parent, &order); err != nil {
			t.Fatalf("query created child: %v", err)
		}
		if !parent.Valid || parent.String != parentUUID {
			t.Fatalf("parent_uuid = %+v, want %s", parent, parentUUID)
		}
		if order != 1 {
			t.Fatalf("child_order = %d, want 1", order)
		}

		var oldFirstChildOrder int
		if err := db.QueryRow(
			"SELECT child_order FROM workflow_items WHERE uuid = ?",
			"11111111-1111-1111-1111-111111111112",
		).Scan(&oldFirstChildOrder); err != nil {
			t.Fatalf("query previous first child order: %v", err)
		}
		if oldFirstChildOrder != 2 {
			t.Fatalf("previous first child order = %d, want 2", oldFirstChildOrder)
		}
	})

	t.Run("create child under leaf parent", func(t *testing.T) {
		parentUUID := "11111111-1111-1111-1111-111111111113"
		req := httptest.NewRequest(http.MethodPost, "/api/items/"+parentUUID+"/child", nil)
		rec := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusCreated; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}

		var response struct {
			UUID string `json:"uuid"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatalf("decode create child response: %v", err)
		}
		if response.UUID == "" {
			t.Fatal("expected created child uuid")
		}

		var parent sql.NullString
		var order int
		if err := db.QueryRow(
			"SELECT parent_uuid, child_order FROM workflow_items WHERE uuid = ?",
			response.UUID,
		).Scan(&parent, &order); err != nil {
			t.Fatalf("query created child under leaf: %v", err)
		}
		if !parent.Valid || parent.String != parentUUID {
			t.Fatalf("parent_uuid = %+v, want %s", parent, parentUUID)
		}
		if order != 1 {
			t.Fatalf("child_order = %d, want 1", order)
		}
	})
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

func TestUpdateItemBodyAPIHandlerNotFound(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/items/not-a-real-id",
		bytes.NewBufferString(`{"body":"updated"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusNotFound; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func TestUpdateItemBodyAPIHandlerInvalidJSON(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/items/11111111-1111-1111-1111-111111111112",
		bytes.NewBufferString(`{"body":"updated","unknown":true}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusBadRequest; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func TestUpdateItemOpenStateAPIHandlerInvalidJSONAndNotFound(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodPatch,
			"/api/items/11111111-1111-1111-1111-111111111111/open-state",
			bytes.NewBufferString(`{"is_open":false,"unknown":true}`),
		)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusBadRequest; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodPatch,
			"/api/items/not-a-real-id/open-state",
			bytes.NewBufferString(`{"is_open":false}`),
		)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusNotFound; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}
	})
}

func TestUpdateItemFavoriteStateAPIHandlerInvalidJSONAndNotFound(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodPatch,
			"/api/items/11111111-1111-1111-1111-111111111111/favorite-state",
			bytes.NewBufferString(`{"is_favorite":true,"unknown":true}`),
		)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusBadRequest; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodPatch,
			"/api/items/not-a-real-id/favorite-state",
			bytes.NewBufferString(`{"is_favorite":true}`),
		)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusNotFound; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}
	})
}

func TestCreateIndentOutdentHandlersNotFound(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr: ":0",
		DB:   db,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	testCases := []struct {
		name string
		path string
	}{
		{
			name: "child",
			path: "/api/items/not-a-real-id/child",
		},
		{
			name: "enter",
			path: "/api/items/not-a-real-id/enter",
		},
		{
			name: "indent",
			path: "/api/items/not-a-real-id/indent",
		},
		{
			name: "outdent",
			path: "/api/items/not-a-real-id/outdent",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, nil)
			rec := httptest.NewRecorder()
			srv.Handler.ServeHTTP(rec, req)

			if got, want := rec.Code, http.StatusNotFound; got != want {
				t.Fatalf("status = %d, want %d", got, want)
			}
		})
	}
}

func TestMutationHandlersMissingUUID(t *testing.T) {
	db := setupTestDB(t)

	store, err := workflow.NewStore(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	testCases := []struct {
		name        string
		method      string
		body        string
		contentType string
		handler     http.HandlerFunc
	}{
		{
			name:        "update body",
			method:      http.MethodPatch,
			body:        `{"body":"updated"}`,
			contentType: "application/json",
			handler:     updateItemBodyAPIHandler(store),
		},
		{
			name:    "delete item",
			method:  http.MethodDelete,
			handler: deleteItemAPIHandler(store),
		},
		{
			name:        "update open state",
			method:      http.MethodPatch,
			body:        `{"is_open":true}`,
			contentType: "application/json",
			handler:     updateItemOpenStateAPIHandler(store),
		},
		{
			name:        "update favorite state",
			method:      http.MethodPatch,
			body:        `{"is_favorite":true}`,
			contentType: "application/json",
			handler:     updateItemFavoriteStateAPIHandler(store),
		},
		{
			name:    "create item after enter",
			method:  http.MethodPost,
			handler: createItemAfterEnterAPIHandler(store),
		},
		{
			name:    "create child item",
			method:  http.MethodPost,
			handler: createChildItemAPIHandler(store),
		},
		{
			name:    "indent item",
			method:  http.MethodPost,
			handler: indentItemAPIHandler(store),
		},
		{
			name:    "outdent item",
			method:  http.MethodPost,
			handler: outdentItemAPIHandler(store),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var bodyReader *strings.Reader
			if tc.body == "" {
				bodyReader = strings.NewReader("")
			} else {
				bodyReader = strings.NewReader(tc.body)
			}

			req := httptest.NewRequest(tc.method, "/", bodyReader)
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}

			rec := httptest.NewRecorder()
			tc.handler.ServeHTTP(rec, req)
			if got, want := rec.Code, http.StatusBadRequest; got != want {
				t.Fatalf("status = %d, want %d", got, want)
			}
		})
	}
}

func TestNewDevModeGeneratesBuildIDWhenMissing(t *testing.T) {
	db := setupTestDB(t)

	srv, err := New(Config{
		Addr:    ":0",
		DB:      db,
		DevMode: true,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/dev/build", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	var response struct {
		BuildID string `json:"build_id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if strings.TrimSpace(response.BuildID) == "" {
		t.Fatal("expected non-empty build_id")
	}
}

func TestDevBuildAPIHandlerHeaders(t *testing.T) {
	handler := devBuildAPIHandler("build-id-1")
	req := httptest.NewRequest(http.MethodGet, "/api/dev/build", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := rec.Header().Get("Cache-Control"), "no-store"; got != want {
		t.Fatalf("cache-control = %q, want %q", got, want)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type = %q, expected application/json", ct)
	}
}

func TestInjectDevBootstrapWithoutHeadMarker(t *testing.T) {
	input := []byte("<html><body>hello</body></html>")
	got := injectDevBootstrap(input, "build-id")
	if string(got) != string(input) {
		t.Fatalf("expected unchanged html when </head> marker is missing")
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

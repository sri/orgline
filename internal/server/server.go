package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"orgline/internal/frontend"
	"orgline/internal/workflow"
)

type Config struct {
	Addr              string
	DB                *sql.DB
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

func New(cfg Config) (*http.Server, error) {
	if cfg.ReadHeaderTimeout <= 0 {
		cfg.ReadHeaderTimeout = 5 * time.Second
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = 15 * time.Second
	}
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = 15 * time.Second
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 60 * time.Second
	}

	workflowStore, err := workflow.NewStore(cfg.DB)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/hello", helloAPIHandler)
	mux.HandleFunc("GET /api/items", itemsAPIHandler(workflowStore))
	mux.HandleFunc("PATCH /api/items/{uuid}", updateItemBodyAPIHandler(workflowStore))
	mux.HandleFunc("PATCH /api/items/{uuid}/open-state", updateItemOpenStateAPIHandler(workflowStore))
	mux.HandleFunc("POST /api/items/{uuid}/enter", createItemAfterEnterAPIHandler(workflowStore))
	mux.HandleFunc("POST /api/items/{uuid}/indent", indentItemAPIHandler(workflowStore))
	mux.HandleFunc("POST /api/items/{uuid}/outdent", outdentItemAPIHandler(workflowStore))

	frontendHandler, err := newFrontendHandler()
	if err != nil {
		return nil, err
	}

	mux.Handle("/", frontendHandler)

	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}, nil
}

func helloAPIHandler(w http.ResponseWriter, _ *http.Request) {
	response := struct {
		Message string `json:"message"`
	}{
		Message: "Hello World",
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "encode response", http.StatusInternalServerError)
	}
}

func itemsAPIHandler(store *workflow.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := store.ListTree(r.Context())
		if err != nil {
			http.Error(w, "load items", http.StatusInternalServerError)
			return
		}

		response := struct {
			Items []workflow.Item `json:"items"`
		}{
			Items: items,
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "encode response", http.StatusInternalServerError)
		}
	}
}

func updateItemBodyAPIHandler(store *workflow.Store) http.HandlerFunc {
	type request struct {
		Body string `json:"body"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		if uuid == "" {
			http.Error(w, "missing uuid", http.StatusBadRequest)
			return
		}

		var payload request
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		body := strings.TrimSpace(payload.Body)
		if body == "" {
			http.Error(w, "body is required", http.StatusBadRequest)
			return
		}

		err := store.UpdateBody(r.Context(), uuid, body)
		if err != nil {
			if errors.Is(err, workflow.ErrItemNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "update item", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func updateItemOpenStateAPIHandler(store *workflow.Store) http.HandlerFunc {
	type request struct {
		IsOpen bool `json:"is_open"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		if uuid == "" {
			http.Error(w, "missing uuid", http.StatusBadRequest)
			return
		}

		var payload request
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		err := store.UpdateOpenState(r.Context(), uuid, payload.IsOpen)
		if err != nil {
			if errors.Is(err, workflow.ErrItemNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "update item open state", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func createItemAfterEnterAPIHandler(store *workflow.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		if uuid == "" {
			http.Error(w, "missing uuid", http.StatusBadRequest)
			return
		}

		newUUID, err := store.CreateAfterEnter(r.Context(), uuid)
		if err != nil {
			if errors.Is(err, workflow.ErrItemNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "create item", http.StatusInternalServerError)
			return
		}

		response := struct {
			UUID string `json:"uuid"`
		}{
			UUID: newUUID,
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "encode response", http.StatusInternalServerError)
		}
	}
}

func indentItemAPIHandler(store *workflow.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		if uuid == "" {
			http.Error(w, "missing uuid", http.StatusBadRequest)
			return
		}

		_, err := store.IndentItem(r.Context(), uuid)
		if err != nil {
			if errors.Is(err, workflow.ErrItemNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "indent item", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func outdentItemAPIHandler(store *workflow.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		if uuid == "" {
			http.Error(w, "missing uuid", http.StatusBadRequest)
			return
		}

		_, err := store.OutdentItem(r.Context(), uuid)
		if err != nil {
			if errors.Is(err, workflow.ErrItemNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "outdent item", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func newFrontendHandler() (http.Handler, error) {
	indexHTML, err := frontend.IndexHTML()
	if err != nil {
		return nil, err
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(indexHTML)
	}), nil
}

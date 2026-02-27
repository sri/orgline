package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"orgline/internal/frontend"
)

type Config struct {
	Addr           string
	FrontendDevURL string
}

func New(cfg Config) (*http.Server, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/hello", helloAPIHandler)

	frontendHandler, err := newFrontendHandler(cfg)
	if err != nil {
		return nil, err
	}

	mux.Handle("/", frontendHandler)

	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
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

func newFrontendHandler(cfg Config) (http.Handler, error) {
	if cfg.FrontendDevURL != "" {
		target, err := url.Parse(cfg.FrontendDevURL)
		if err != nil {
			return nil, fmt.Errorf("parse frontend dev url: %w", err)
		}

		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "frontend dev server unavailable", http.StatusBadGateway)
		}

		return proxy, nil
	}

	distFS, err := frontend.DistFS()
	if err != nil {
		return nil, fmt.Errorf("load embedded frontend: %w", err)
	}

	if _, err := fs.Stat(distFS, "site/index.html"); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(
				w,
				"frontend assets are missing; run `just prod` to build and embed them",
				http.StatusServiceUnavailable,
			)
		}), nil
	}

	siteFS, err := fs.Sub(distFS, "site")
	if err != nil {
		return nil, fmt.Errorf("load embedded frontend site fs: %w", err)
	}

	return http.FileServerFS(siteFS), nil
}

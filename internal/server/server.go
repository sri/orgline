package server

import (
	"net/http"
	"time"
)

type Config struct {
	Addr string
}

func New(cfg Config) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", helloWorldHandler)

	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func helloWorldHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("Hello World"))
}

package main

import (
	"errors"
	"log"
	"net/http"

	"orgline/internal/server"
)

func main() {
	cfg := server.Config{
		Addr: ":8080",
	}

	srv := server.New(cfg)
	log.Printf("starting server on http://localhost%s", cfg.Addr)

	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

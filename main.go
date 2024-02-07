package main

import (
	"net/http"
	"os"
	"time"

	"github.com/Ztkent/data-manager/internal/config"
	"github.com/Ztkent/data-manager/internal/data"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "html/index.html")
	})

	r.Get("/network", func(w http.ResponseWriter, r *http.Request) {
		data.StartProcessor()
		time.Sleep(1 * time.Second)
		http.ServeFile(w, r, "html/network.html")
	})

	r.Post("/crawl", func(w http.ResponseWriter, r *http.Request) {
		url := r.FormValue("crawlInput")
		w.Write([]byte("Crawling started at " + url))
		config.StartCrawlerWithConfig(config.NewDefaultConfig())
	})

	r.Get("/export", func(w http.ResponseWriter, r *http.Request) {
		filePath := "pkg/data-crawler/results.db"
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Disposition", "attachment; filename=results.db")
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, filePath)
	})
	http.ListenAndServe(":8080", r)
}

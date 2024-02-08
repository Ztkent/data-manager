package main

import (
	"context"
	"net/http"

	"github.com/Ztkent/data-manager/internal/routes"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

func main() {
	// Initialize router and middleware
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	// Initialize crawlMap and crawlChan
	crawlMap := make(map[string]context.CancelFunc)
	crawlChan := make(chan string)
	crawlManager := routes.Manager{CrawlMap: crawlMap, CrawlChan: crawlChan}

	// Define routes
	defineRoutes(r, &crawlManager)

	// Handle any finished crawlers
	go crawlManager.HandleFinishedCrawlers()

	// Start server
	http.ListenAndServe(":8080", r)
}

func defineRoutes(r *chi.Mux, crawlManager *routes.Manager) {
	r.Get("/", routes.ServeIndex)
	r.Get("/network", routes.ServeNetwork)
	r.Get("/export", routes.ServeResults)
	// r.Post("/uploda", crawlManager.UploadHandler())
	r.Post("/crawl", crawlManager.CrawlHandler())
	r.Post("/crawl-random", crawlManager.CrawlRandomHandler())
	r.Post("/kill-all-crawlers", crawlManager.KillAllCrawlersHandler())
	r.Get("/active-crawlers", crawlManager.ActiveCrawlersHandler())
}

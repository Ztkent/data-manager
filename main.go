package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ztkent/data-manager/internal/db"
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
	crawlManager := routes.CrawlManager{
		CrawlMap:  crawlMap,
		CrawlChan: crawlChan,
		SqliteDB:  db.NewDatabase(db.ConnectSqlite()),
	}

	// Define routes
	defineRoutes(r, &crawlManager)

	// Handle any finished crawlers
	go crawlManager.HandleFinishedCrawlers()

	// Start server
	http.ListenAndServe(":8080", r)
}

func defineRoutes(r *chi.Mux, crawlManager *routes.CrawlManager) {
	r.Get("/", routes.ServeHome)
	r.Get("/network", routes.ServeNetwork)
	r.Post("/gen-network", routes.GenNetwork)
	r.Get("/export", routes.ServeResults)
	r.Post("/crawl", crawlManager.CrawlHandler())
	r.Post("/crawl-random", crawlManager.CrawlRandomHandler())
	r.Post("/kill-all-crawlers", crawlManager.KillAllCrawlersHandler())
	r.Post("/kill-crawler", crawlManager.KillCrawlerHandler())
	r.Get("/active-crawlers", crawlManager.ActiveCrawlersHandler())
	r.Get("/dismiss-toast", crawlManager.DismissToastHandler())
	r.Get("/recent-urls", crawlManager.RecentURLsHandler())

	// Serve static images
	workDir, _ := os.Getwd()
	filesDir := filepath.Join(workDir, "html", "img")
	FileServer(r, "/img", http.Dir(filesDir))
	FileServer(r, "/favicon.ico", http.Dir(filesDir))
}

func FileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit any URL parameters.")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	r.Get(path+"*", func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix(path, http.FileServer(root)).ServeHTTP(w, r)
	})
}

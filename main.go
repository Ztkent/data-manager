package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ztkent/data-manager/internal/routes"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

func main() {
	// Handle any required environment variables
	checkRequiredEnvs()

	// Initialize router and middleware
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	crawlMaster := routes.CrawlMaster{
		ActiveManagers: make(map[string]*routes.CrawlManager),
	}

	// Define routes
	defineRoutes(r, &crawlMaster)

	// Handle any finished crawlers
	go crawlMaster.HandleFinishedCrawlers()
	// TODO: Clean up any resources from old crawl managers.

	// Start server
	http.ListenAndServe(":8080", r)
}

func defineRoutes(r *chi.Mux, crawlMaster *routes.CrawlMaster) {
	r.Get("/", crawlMaster.ServeHome())
	r.Get("/network", crawlMaster.ServeNetwork())
	r.Post("/gen-network", crawlMaster.GenNetwork())
	r.Post("/crawl", crawlMaster.CrawlHandler())
	r.Post("/crawl-random", crawlMaster.CrawlRandomHandler())
	r.Post("/kill-all-crawlers", crawlMaster.KillAllCrawlersHandler())
	r.Post("/kill-crawler", crawlMaster.KillCrawlerHandler())
	r.Get("/active-crawlers", crawlMaster.ActiveCrawlersHandler())
	r.Get("/recent-urls", crawlMaster.RecentURLsHandler())
	r.Get("/export", crawlMaster.ExportDB())
	r.Get("/dismiss-toast", crawlMaster.DismissToastHandler())
	r.Post("/about-modal", crawlMaster.AboutModalHandler())

	// Ensure that the user has been assigned, and is using a valid JWT
	r.Post("/ensure-jwt", crawlMaster.EnsureJWTHandler())

	// Serve static files
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

func checkRequiredEnvs() {
	secret := os.Getenv("JWT_SECRET_TOKEN")
	if secret == "" {
		log.Fatal("JWT_SECRET_TOKEN is not set")
	}
}

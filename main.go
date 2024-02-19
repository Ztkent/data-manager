package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ztkent/data-manager/internal/db"
	"github.com/Ztkent/data-manager/internal/routes"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
)

func main() {
	// Handle any required environment variables
	checkRequiredEnvs()

	// Connect Redis
	redis, err := db.ConnectRedis()
	if err != nil {
		log.Fatal("Failed to Connect to Redis: " + err.Error())
	}
	fmt.Println("Successfully connected to Redis")

	// Connect PG
	pgDB, err := db.ConnectPostgres()
	if err != nil {
		log.Fatal("Failed to Connect to PG: " + err.Error())
	}
	fmt.Println("Successfully connected to PG")

	// Initialize crawl master, which will manage all crawl users
	crawlMaster := routes.CrawlMaster{
		ActiveManagers: make(map[string]*routes.CrawlManager),
		DB:             db.NewMasterDatabase(pgDB),
		Redis:          redis,
	}

	// Initialize router and middleware
	r := chi.NewRouter()
	// Log request and recover from panics
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Define routes
	defineRoutes(r, &crawlMaster)

	// Handle any finished crawlers
	go crawlMaster.HandleFinishedCrawlers()
	go crawlMaster.ResourceManger()

	// Start server
	fmt.Println("Server is running on port 8080")
	if os.Getenv("ENV") == "dev" {
		log.Fatal(http.ListenAndServe(":8080", r))
	}
	log.Fatal(http.ListenAndServeTLS(":8080", os.Getenv("CERT_PATH"), os.Getenv("CERT_KEY_PATH"), r))
}

func defineRoutes(r *chi.Mux, crawlMaster *routes.CrawlMaster) {
	// Apply a rate limiter to all routes
	r.Use(httprate.Limit(
		10,             // requests
		60*time.Second, // per duration
		httprate.WithKeyFuncs(httprate.KeyByIP, httprate.KeyByEndpoint),
	))

	// Auth
	r.Post("/ensure-uuid", crawlMaster.EnsureUUIDHandler())         // Make sure every active user is assigned a UUID
	r.Post("/login", crawlMaster.Login())                           // Login Modal
	r.Post("/logout", crawlMaster.Logout())                         // Logout Modal
	r.Post("/submit-register", crawlMaster.SubmitRegister())        // Submit Registration attempt
	r.Post("/submit-login", crawlMaster.SubmitLogin())              // Submit Login attempt
	r.Post("/confirm-login", crawlMaster.ConfirmLoginAttempt(true)) // Confirm Login attempt
	r.Post("/validate-login", crawlMaster.ValidateLogin())          // Validate if active user is logged in

	// Static
	r.Get("/", crawlMaster.ServeHome())                        // Homepage
	r.Get("/tc", crawlMaster.ServeTC())                        // Terms and Conditions
	r.Get("/dismiss-toast", crawlMaster.DismissToastHandler()) // Dismiss any toast messages
	r.Post("/about-modal", crawlMaster.AboutModalHandler())    // About Modal
	r.Post("/export-modal", crawlMaster.ExportModal())         // Data Export Modal

	// Network
	r.Post("/gen-network", crawlMaster.GenNetwork()) // Regularly regenerate network graph
	r.Get("/network", crawlMaster.ServeNetwork())    // Serve network graph
	// Crawl
	r.Post("/crawl", crawlMaster.CrawlHandler())                       // Crawl a specific URL
	r.Post("/crawl-random", crawlMaster.CrawlRandomHandler())          // Crawl a random URL from the test-sites list
	r.Post("/kill-crawler", crawlMaster.KillCrawlerHandler())          // Kill a specific crawler
	r.Post("/kill-all-crawlers", crawlMaster.KillAllCrawlersHandler()) // Kill all crawlers for this user
	// Data
	r.Get("/active-crawlers", crawlMaster.ActiveCrawlersHandler())  // Get all active crawlers for this user
	r.Get("/recent-urls", crawlMaster.RecentURLsHandler())          // Get some recent URLs for this user
	r.Post("/file-collection", crawlMaster.FileCollectionHandler()) // Get some recent files for this user
	r.Get("/export", crawlMaster.ExportDB())                        // Handle data export requests

	// Serve static files
	workDir, _ := os.Getwd()
	filesDir := filepath.Join(workDir, "internal", "html", "img")
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
	envs := []string{
		"JWT_SECRET_TOKEN",
		"REDIS_HOST",
		"REDIS_PORT",
		"POSTGRES_USER",
		"POSTGRES_PASSWORD",
		"POSTGRES_DB",
		"POSTGRES_HOST",
		"POSTGRES_PORT",
		"CERT_PATH",
		"CERT_KEY_PATH",
	}
	for _, env := range envs {
		if value := os.Getenv(env); value == "" {
			log.Fatalf("%s environment variable is not set", env)
		}
	}
}

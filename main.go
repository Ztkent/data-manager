package main

import (
	"fmt"
	"log"
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
	// Handle any required environment variables
	checkRequiredEnvs()

	// Connect Redis
	redis, err := db.ConnectRedis()
	if err != nil {
		log.Fatal("Failed to Connect to Redis: " + err.Error())
	}
	fmt.Println("Successfully connected to Redis")
	// Connect Master PG
	pgDB, err := db.ConnectPostgres()
	if err != nil {
		log.Fatal("Failed to Connect to Master PG: " + err.Error())
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
	r.Use(middleware.Logger)

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
	// Auth
	r.Post("/ensure-uuid", crawlMaster.EnsureUUIDHandler())
	r.Post("/login", crawlMaster.Login())
	r.Post("/logout", crawlMaster.Logout())
	r.Post("/submit-register", crawlMaster.SubmitRegister())
	r.Post("/submit-login", crawlMaster.SubmitLogin())
	r.Post("/confirm-login", crawlMaster.ConfirmLogin())

	// Service
	r.Get("/", crawlMaster.ServeHome())
	r.Get("/tc", crawlMaster.ServeTC())
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

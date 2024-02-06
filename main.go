package main

import (
	"log"
	"net/http"
	"os/exec"

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
		http.ServeFile(w, r, "html/network.html")
	})

	// startCrawlerWithOptions("pkg/data-crawler/crawl.json")
	// startProcessor()
	http.ListenAndServe(":8080", r)
}

func startCrawlerWithOptions(path string) {
	cmd := exec.Command("./pkg/data-crawler/v0.1.0/data-crawler", "-c", path)
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func startCrawler() {
	cmd := exec.Command("./pkg/data-crawler/v0.1.0/data-crawler")
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func startProcessor() {
	cmd := exec.Command("python3", "pkg/data-processor/data_processor.py", "--database", "pkg/data-crawler/results.db")
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

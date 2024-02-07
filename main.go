package main

import (
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

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
		// if html/network.html is not found, return some blank page
		if _, err := os.Stat("html/network.html"); os.IsNotExist(err) {
			startProcessor()
			time.Sleep(1 * time.Second)
		}
		http.ServeFile(w, r, "html/network.html")
	})

	r.Post("/crawl", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Crawling started"))
		startCrawlerWithOptions("pkg/data-crawler/crawl.json")
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

func startCrawlerWithOptions(path string) {
	go func() {
		cmd := exec.Command("./pkg/data-crawler/v0.1.0/data-crawler", "-c", path)
		err := cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	}()
}

func startCrawler() {
	go func() {
		cmd := exec.Command("./pkg/data-crawler/v0.1.0/data-crawler")
		err := cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	}()
}

func startProcessor() {
	go func() {
		cmd := exec.Command("python3", "pkg/data-processor/data_processor.py", "--database", "pkg/data-crawler/results.db")
		err := cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	}()
}

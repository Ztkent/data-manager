package routes

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/Ztkent/data-manager/internal/config"
)

type Manager struct {
	CrawlMap  map[string]context.CancelFunc
	CrawlChan chan string
	sync.Mutex
}

func ServeIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "html/index.html")
}

func ServeNetwork(w http.ResponseWriter, r *http.Request) {
	// Generate a network file
	cmd := exec.Command("python3", "pkg/data-processor/data_processor.py", "--database", "pkg/data-crawler/results.db")
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	time.Sleep(1 * time.Second)
	http.ServeFile(w, r, "html/network.html")
}

func ServeResults(w http.ResponseWriter, r *http.Request) {
	filePath := "pkg/data-crawler/results.db"
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=results.db")
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, filePath)
}

func (m *Manager) CrawlHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		url := r.FormValue("crawlInput")
		curr_config := config.NewDefaultConfig()
		curr_config.StartingURL = url

		ctxCrawler, cancel := context.WithCancel(context.Background())
		m.CrawlMap[url] = cancel

		err := config.StartCrawlerWithConfig(ctxCrawler, curr_config, m.CrawlChan)
		if err != nil {
			http.Error(w, "Error starting crawler", http.StatusInternalServerError)
		}
	}
}

func (m *Manager) KillAllCrawlersHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for _, cancel := range m.CrawlMap {
			cancel()
		}
	}
}

func (m *Manager) ActiveCrawlersHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for url := range m.CrawlMap {
			w.Write([]byte(url + "\n"))
		}
	}
}

func (m *Manager) HandleFinishedCrawlers() {
	for {
		select {
		case url := <-m.CrawlChan:
			delete(m.CrawlMap, url)
		}
	}
}

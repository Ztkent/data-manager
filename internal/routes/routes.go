package routes

import (
	"bufio"
	"context"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"sync"

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
	// Generate a network file with the processor
	cmd := exec.Command("python3", "pkg/data-processor/data_processor.py", "--database", "pkg/data-crawler/results.db")
	err := cmd.Run()
	if err != nil {
		http.Error(w, "Error generating network file", http.StatusInternalServerError)
		return
	}
	http.ServeFile(w, r, "html/network.html")
}

func ServeResults(w http.ResponseWriter, r *http.Request) {
	filePath := "pkg/data-crawler/results.db"
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "Results DB not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=results.db")
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, filePath)
}

func (m *Manager) CrawlHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		curr_config, err := config.ParseFormToConfig(r.Form)
		if err != nil {
			http.Error(w, "Error parsing config settings, using default", http.StatusBadRequest)
			curr_config = config.NewDefaultConfig()
		}

		if curr_config.StartingURL == "" {
			http.Error(w, "No URL provided", http.StatusBadRequest)
			return
		}

		ctxCrawler, cancel := context.WithCancel(context.Background())
		m.CrawlMap[curr_config.StartingURL] = cancel
		err = config.StartCrawlerWithConfig(ctxCrawler, curr_config, m.CrawlChan)
		if err != nil {
			http.Error(w, "Error starting crawler", http.StatusInternalServerError)
		}
	}
}

func (m *Manager) CrawlRandomHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		randomURL, err := selectRandomUrl()
		if err != nil {
			http.Error(w, "Error selecting starting url", http.StatusInternalServerError)
			return
		} else if randomURL == "" {
			http.Error(w, "Failed to randomly select url", http.StatusNotFound)
			return
		}

		curr_config, err := config.ParseFormToConfig(r.Form)
		if err != nil {
			http.Error(w, "Error parsing config settings, using default", http.StatusBadRequest)
			curr_config = config.NewDefaultConfig()
		}

		curr_config.StartingURL = randomURL
		ctxCrawler, cancel := context.WithCancel(context.Background())
		m.CrawlMap[randomURL] = cancel
		err = config.StartCrawlerWithConfig(ctxCrawler, curr_config, m.CrawlChan)
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

func (m *Manager) KillCrawlerHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		url := r.FormValue("url")
		cancel, ok := m.CrawlMap[url]
		if !ok {
			http.Error(w, "Crawler not found", http.StatusNotFound)
			return
		}
		cancel()
		m.ActiveCrawlersHandler()(w, r)
	}
}

func (m *Manager) ActiveCrawlersHandler() http.HandlerFunc {
	type Crawler struct {
		URL string
	}
	return func(w http.ResponseWriter, r *http.Request) {
		crawlers := make([]Crawler, 0, len(m.CrawlMap))
		for url := range m.CrawlMap {
			crawlers = append(crawlers, Crawler{URL: url})
		}
		sort.Slice(crawlers, func(i, j int) bool {
			return crawlers[i].URL < crawlers[j].URL
		})

		// Render the active_crawlers template, which displays the active crawlers
		tmpl, err := template.ParseFiles("html/active_crawlers.gohtml")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = tmpl.Execute(w, crawlers)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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

func selectRandomUrl() (string, error) {
	file, err := os.Open("internal/routes/test-sites")
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	urls := make([]string, 0)
	for scanner.Scan() {
		urls = append(urls, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	randomURL := urls[rand.Intn(len(urls))]
	return randomURL, nil
}

func logForm(r *http.Request) {
	r.ParseForm()
	for key, values := range r.Form {
		for _, value := range values {
			log.Printf("Form key: %s, value: %s\n", key, value)
		}
	}
}

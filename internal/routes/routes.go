package routes

import (
	"bufio"
	"context"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/Ztkent/data-manager/internal/config"
	"github.com/Ztkent/data-manager/internal/db"
)

// Every user should get their own crawl manager
type CrawlManager struct {
	CrawlMap  map[string]context.CancelFunc
	CrawlChan chan string
	SqliteDB  db.Database
	sync.Mutex
}
type Toast struct {
	ToastContent string
	Border       string
}

func ServeHome(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "html/home.html")
}

func ServeNetwork(w http.ResponseWriter, r *http.Request) {
	// check if the network file exists
	if _, err := os.Stat("html/network.html"); os.IsNotExist(err) {
		// call GenNetwork if the file does not exist
		GenNetwork(w, r)
		return
	}

	http.ServeFile(w, r, "html/network.html")
}

func GenNetwork(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("python3", "pkg/data-processor/data_processor.py", "--database", "pkg/data-crawler/results.db")

	// Check if we have a physics toggle in the request
	logForm(r)
	check := r.FormValue("checked")
	if check == "true" {
		cmd.Args = append(cmd.Args, "--physics", "true")
	}

	// Generate a network file with the processor
	err := cmd.Run()
	if err != nil {
		http.Error(w, "Error generating network file", http.StatusInternalServerError)
		return
	}
	// Render the active_crawlers template, which displays the active crawlers
	tmpl, err := template.ParseFiles("html/network_iframe.gohtml")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = tmpl.Execute(w, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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

func (m *CrawlManager) CrawlHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		curr_config, err := config.ParseFormToConfig(r.Form)
		if err != nil {
			http.Error(w, "Error parsing config settings, using default", http.StatusBadRequest)
			curr_config = config.NewDefaultConfig()
		}

		if curr_config.StartingURL == "" {
			serveFailToast(w, "No URL provided")
			return
		}

		// ensure the StartingURL is properly formatted
		valid, reason := ValidateStartingURL(curr_config.StartingURL)
		if !valid {
			serveFailToast(w, fmt.Sprintf("%s", reason))
			return
		}

		ctxCrawler, cancel := context.WithCancel(context.Background())
		m.CrawlMap[curr_config.StartingURL] = cancel
		err = config.StartCrawlerWithConfig(ctxCrawler, curr_config, m.CrawlChan)
		if err != nil {
			serveFailToast(w, "Error starting crawler: "+curr_config.StartingURL)
			return
		}
	}
}

func ValidateStartingURL(startingURL string) (bool, string) {
	u, err := url.Parse(startingURL)
	if err != nil {
		return false, "Invalid URL"
	}
	// Valid the url
	if u.Scheme != "http" && u.Scheme != "https" {
		return false, "Invalid URL scheme, must be http or https"
	} else if u.Host == "" {
		return false, "Empty URL host"
	} else if !strings.HasPrefix(u.Host, "www.") {
		return false, "Invalid URL host, must start with www."
	}

	// More generic failure, but be sure
	match, _ := regexp.MatchString(`^https?://www\.[\w.-]+\.[A-Za-z]{2,}$`, startingURL)
	if !match {
		return false, "Invalid Crawl URL"
	}

	return true, ""
}

func (m *CrawlManager) CrawlRandomHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		randomURL, err := selectRandomUrl()
		if err != nil {
			serveFailToast(w, "Error selecting starting url")
			return
		} else if randomURL == "" {
			serveFailToast(w, "Failed to randomly select url")
			return
		}

		r.Form.Set("StartingURL", randomURL)
		curr_config, err := config.ParseFormToConfig(r.Form)
		if err != nil {
			http.Error(w, "Error parsing config settings, using default", http.StatusBadRequest)
			curr_config = config.NewDefaultConfig()
		}

		ctxCrawler, cancel := context.WithCancel(context.Background())
		m.CrawlMap[randomURL] = cancel
		err = config.StartCrawlerWithConfig(ctxCrawler, curr_config, m.CrawlChan)
		if err != nil {
			serveFailToast(w, "Error starting crawler: "+curr_config.StartingURL)
			return
		}
	}
}

func (m *CrawlManager) KillAllCrawlersHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		numCrawler := len(m.CrawlMap)
		if numCrawler == 0 {
			serveFailToast(w, "No active crawlers to kill")
			return
		}
		for _, cancel := range m.CrawlMap {
			cancel()
		}
		message := "1 crawler killed"
		if numCrawler > 1 {
			message = fmt.Sprintf("%d crawlers killed", numCrawler)
		}
		serveSuccessToast(w, message)
	}
}

func (m *CrawlManager) KillCrawlerHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		url := r.FormValue("url")
		cancel, ok := m.CrawlMap[url]
		if !ok {
			http.Error(w, "Crawler not found", http.StatusNotFound)
		}
		cancel()
		m.ActiveCrawlersHandler()(w, r)
	}
}

func (m *CrawlManager) ActiveCrawlersHandler() http.HandlerFunc {
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

func (m *CrawlManager) HandleFinishedCrawlers() {
	for {
		select {
		case url := <-m.CrawlChan:
			delete(m.CrawlMap, url)
		}
	}
}

func serveFailToast(w http.ResponseWriter, message string) {
	// Render the crawl_status template, which displays the toast
	tmpl, err := template.ParseFiles("html/crawl_status_toast.gohtml")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	toast := &Toast{ToastContent: message, Border: "border-red-200"}
	err = tmpl.Execute(w, toast)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

func serveSuccessToast(w http.ResponseWriter, message string) {
	// Render the crawl_status template, which displays the toast
	tmpl, err := template.ParseFiles("html/crawl_status_toast.gohtml")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	toast := &Toast{ToastContent: message, Border: "border-green-200"}
	err = tmpl.Execute(w, toast)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

func (m *CrawlManager) DismissToastHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
	}
}

func (m *CrawlManager) RecentURLsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		visited, err := m.SqliteDB.GetRecentVisited()
		if err != nil {
			log.Default().Println(err)
			return
		}
		tmpl, err := template.ParseFiles("html/recent_visited.gohtml")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = tmpl.Execute(w, visited)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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

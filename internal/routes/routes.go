package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"sync"
	"time"

	"github.com/Ztkent/data-manager/internal/config"
	"github.com/Ztkent/data-manager/internal/db"
)

// Manage all users
type CrawlMaster struct {
	ActiveManagers map[string]*CrawlManager
	sync.RWMutex
}

// Manage a single user
type CrawlManager struct {
	UserID    string
	CrawlMap  map[string]context.CancelFunc
	CrawlChan chan string
	SqliteDB  db.Database
	CreatedAt *time.Time
	UpdatedAt *time.Time
	sync.RWMutex
}

// Crawl Manager
func (m *CrawlManager) GetDBPath() string {
	return fmt.Sprintf("user/data-crawler/results_%s.db", m.UserID)
}

func (m *CrawlManager) GetNetworkPath() string {
	return fmt.Sprintf("user/network/network_%s.html", m.UserID)
}

func (m *CrawlManager) GetConfigPath() string {
	return fmt.Sprintf("user/config/config_%s.json", m.UserID)
}

func (m *CrawlManager) StartCrawlerWithConfig(ctx context.Context, curr_config *config.Config) error {
	json, err := json.Marshal(curr_config)
	if err != nil {
		return err
	}
	path := config.WriteJsonToFile(json, m.GetConfigPath())
	go func() {
		cmd := exec.CommandContext(ctx, "./pkg/data-crawler/v0.1.0/data-crawler", "-c", path)
		err := cmd.Run()
		if err != nil {
			fmt.Println(err)
		}
		// Notify the channel that the crawler is done
		m.CrawlChan <- curr_config.StartingURL
	}()
	return nil
}

// Crawl Master
func (m *CrawlMaster) GetCrawlManagerForRequest(r *http.Request) (*CrawlManager, error) {
	jwt, err := r.Cookie("jwt")
	if err != nil {
		return nil, fmt.Errorf("Failed to get JWT from request")
	}

	// Get the crawl manager for the user
	var crawlManager *CrawlManager
	m.RLock()
	crawlManager = m.ActiveManagers[jwt.Value]
	m.RUnlock()

	if crawlManager == nil {
		if crawlManager == nil {
			now := time.Now()
			crawlManager = &CrawlManager{
				UserID:    jwt.Value,
				CrawlMap:  make(map[string]context.CancelFunc),
				CrawlChan: make(chan string),
				CreatedAt: &now,
				UpdatedAt: &now,
			}
			crawlManager.SqliteDB = db.NewDatabase(db.ConnectSqlite(crawlManager.GetDBPath()))

			m.Lock()
			m.ActiveManagers[jwt.Value] = crawlManager
			m.Unlock()
		}
	}
	return crawlManager, nil
}

func (m *CrawlMaster) ServeHome() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "html/home.html")
	}
}

func (m *CrawlMaster) ServeNetwork() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// check if the network file exists
		if _, err := os.Stat(crawlManager.GetNetworkPath()); os.IsNotExist(err) {
			// call GenNetwork if the file does not exist
			m.GenNetwork()(w, r)
			return
		}

		http.ServeFile(w, r, crawlManager.GetNetworkPath())
	}
}

func (m *CrawlMaster) GenNetwork() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		cmd := exec.Command("python3", "pkg/data-processor/data_processor.py", "--database", crawlManager.GetDBPath(), "--output", crawlManager.GetNetworkPath())
		// Generate a network file with the processor
		err = cmd.Run()
		if err != nil {
			http.Error(w, "Error generating network file", http.StatusInternalServerError)
			return
		}
		// Render the active_crawlers template, which displays the active crawlers
		tmpl, err := template.ParseFiles("html/templates/network_iframe.gohtml")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = tmpl.Execute(w, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (m *CrawlMaster) ExportDB() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := os.Stat(crawlManager.GetDBPath()); os.IsNotExist(err) {
			http.Error(w, "Results DB not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Disposition", "attachment; filename=results.db")
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, crawlManager.GetDBPath())
	}
}

func (m *CrawlMaster) CrawlHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		r.ParseForm()
		curr_config, err := config.ParseFormToConfig(r.Form, crawlManager.GetDBPath())
		if err != nil {
			http.Error(w, "Error parsing config settings, using default", http.StatusBadRequest)
			curr_config = config.NewDefaultConfig()
		}

		if curr_config.StartingURL == "" {
			serveFailToast(w, "No URL provided")
			return
		}

		// ensure the StartingURL is properly formatted
		valid, reason := validateStartingURL(curr_config.StartingURL)
		if !valid {
			serveFailToast(w, fmt.Sprintf("%s", reason))
			return
		}

		ctxCrawler, cancel := context.WithCancel(context.Background())
		crawlManager.CrawlMap[curr_config.StartingURL] = cancel
		err = crawlManager.StartCrawlerWithConfig(ctxCrawler, curr_config)
		if err != nil {
			serveFailToast(w, "Error starting crawler: "+curr_config.StartingURL)
			return
		}
	}
}

func (m *CrawlMaster) CrawlRandomHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

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
		curr_config, err := config.ParseFormToConfig(r.Form, crawlManager.GetDBPath())
		if err != nil {
			http.Error(w, "Error parsing config settings, using default", http.StatusBadRequest)
			curr_config = config.NewDefaultConfig()
		}

		ctxCrawler, cancel := context.WithCancel(context.Background())
		crawlManager.CrawlMap[randomURL] = cancel
		err = crawlManager.StartCrawlerWithConfig(ctxCrawler, curr_config)
		if err != nil {
			serveFailToast(w, "Error starting crawler: "+curr_config.StartingURL)
			return
		}
	}
}

func (m *CrawlMaster) KillAllCrawlersHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		numCrawler := len(crawlManager.CrawlMap)
		if numCrawler == 0 {
			serveFailToast(w, "No active crawlers to kill")
			return
		}
		for _, cancel := range crawlManager.CrawlMap {
			cancel()
		}
		message := "1 crawler killed"
		if numCrawler > 1 {
			message = fmt.Sprintf("%d crawlers killed", numCrawler)
		}
		serveSuccessToast(w, message)
	}
}

func (m *CrawlMaster) KillCrawlerHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		url := r.FormValue("url")
		cancel, ok := crawlManager.CrawlMap[url]
		if !ok {
			http.Error(w, "Crawler not found", http.StatusNotFound)
		}
		cancel()
		m.ActiveCrawlersHandler()(w, r)
	}
}

func (m *CrawlMaster) ActiveCrawlersHandler() http.HandlerFunc {
	type Crawler struct {
		URL string
	}
	return func(w http.ResponseWriter, r *http.Request) {
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		crawlers := make([]Crawler, 0, len(crawlManager.CrawlMap))
		for url := range crawlManager.CrawlMap {
			crawlers = append(crawlers, Crawler{URL: url})
		}
		sort.Slice(crawlers, func(i, j int) bool {
			return crawlers[i].URL < crawlers[j].URL
		})

		// Render the active_crawlers template, which displays the active crawlers
		tmpl, err := template.ParseFiles("html/templates/active_crawlers.gohtml")
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

func (m *CrawlMaster) RecentURLsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		visited, err := crawlManager.SqliteDB.GetRecentVisited()
		if err != nil {
			log.Default().Println(err)
			return
		}
		tmpl, err := template.ParseFiles("html/templates/recent_visited.gohtml")
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

func (m *CrawlMaster) HandleFinishedCrawlers() {
	for {
		time.Sleep(1 * time.Second)
		func() {
			m.RLock()
			defer m.RUnlock()
			for _, crawler := range m.ActiveManagers {
				select {
				case url := <-crawler.CrawlChan:
					crawler.Lock()
					defer crawler.Unlock()
					delete(crawler.CrawlMap, url)
				default:
					continue
				}
			}
		}()
	}
}

func (m *CrawlMaster) DismissToastHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
	}
}

func (m *CrawlMaster) EnsureJWTHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := r.Cookie("jwt")
		if err == http.ErrNoCookie {
			// Cookie does not exist, set it
			jwt, err := generateJWT()
			if err != nil {
				http.Error(w, "Failed to generate JWT", http.StatusInternalServerError)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     "jwt",
				Value:    jwt,
				HttpOnly: true,
				Secure:   false, // Set to true if your site uses HTTPS
				SameSite: http.SameSiteStrictMode,
			})
		} else if err != nil {
			// Some other error occurred
			http.Error(w, "Failed to read cookie", http.StatusInternalServerError)
		}
	}
}

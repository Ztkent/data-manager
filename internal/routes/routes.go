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
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Ztkent/data-manager/internal/config"
	"github.com/Ztkent/data-manager/internal/db"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// Manage all users
type CrawlMaster struct {
	ActiveManagers map[string]*CrawlManager
	DB             db.MasterDatabase
	Redis          *redis.Client
	sync.RWMutex
}

// Manage a single user
type CrawlManager struct {
	UserID    string
	CrawlMap  map[string]context.CancelFunc
	CrawlChan chan string
	SqliteDB  db.ManagerDatabase
	CreatedAt *time.Time
	UpdatedAt *time.Time
	sync.RWMutex
}

const MAX_CRALWERS = 5 // Maximum number of concurrent crawlers

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
		cmd := exec.CommandContext(ctx, "./pkg/data-crawler/data-crawler", "-c", path)
		err := cmd.Run()
		if err != nil {
			fmt.Println(err)
		}
		// Notify the channel that the crawler is done
		m.CrawlChan <- curr_config.StartingURL
	}()
	return nil
}

func (m *CrawlManager) AddCrawlerToMap(curr_config *config.Config, cancel context.CancelFunc) error {
	m.Lock()
	defer m.Unlock()

	// Limit the number of concurrent crawlers
	if len(m.CrawlMap) >= MAX_CRALWERS {
		return fmt.Errorf("Too many active crawlers")
	}
	m.CrawlMap[curr_config.StartingURL] = cancel
	return nil
}

// Crawl Master
func (m *CrawlMaster) GetCrawlManagerForRequest(r *http.Request) (*CrawlManager, error) {
	uuid, err := r.Cookie("uuid")
	if err != nil {
		return nil, fmt.Errorf("Failed to get UUID from request")
	}

	// Get the crawl manager for the user
	var crawlManager *CrawlManager
	m.RLock()
	crawlManager = m.ActiveManagers[uuid.Value]
	m.RUnlock()

	if crawlManager == nil {
		if crawlManager == nil {
			now := time.Now()
			crawlManager = &CrawlManager{
				UserID:    uuid.Value,
				CrawlMap:  make(map[string]context.CancelFunc),
				CrawlChan: make(chan string),
				CreatedAt: &now,
				UpdatedAt: &now,
			}
			crawlManager.SqliteDB = db.NewManagerDatabase(db.ConnectSqlite(crawlManager.GetDBPath()))

			m.Lock()
			m.ActiveManagers[uuid.Value] = crawlManager
			m.Unlock()
		}
	}
	return crawlManager, nil
}

func (m *CrawlMaster) ServeHome() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "internal/html/home.html")
	}
}

func (m *CrawlMaster) ServeTC() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "internal/html/tc.html")
	}
}

func (m *CrawlMaster) Login() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("close") == "true" {
			// Close the modal
			return
		} else if r.URL.Query().Get("register") == "true" {
			// Render the register template
			tmpl, err := template.ParseFiles("internal/html/templates/register_modal.gohtml")
			if err != nil {
				log.Default().Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			err = tmpl.Execute(w, nil)
			if err != nil {
				log.Default().Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		// Render the login template
		tmpl, err := template.ParseFiles("internal/html/templates/login_modal.gohtml")
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = tmpl.Execute(w, nil)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
func (m *CrawlMaster) ConfirmLoginAttempt(alert bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Render the logout button if the user is logged in
		if alert {
			serveSuccessToast(w, "Login Successful")
		}
		tmpl, err := template.ParseFiles("internal/html/templates/logout_button.gohtml")
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = tmpl.Execute(w, nil)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (m *CrawlMaster) ValidateLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if the user is logged in
		err := checkIfUserLoggedIn(r, w, m)
		if err != nil {
			// User is not logged in
			http.Error(w, "User is not logged in", http.StatusUnauthorized)
			return
		}
		m.ConfirmLoginAttempt(false)(w, r)
	}
}

func (m *CrawlMaster) SubmitLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		email := r.FormValue("email")
		pass := r.FormValue("password")

		// Validate the email and password
		valid := validateEmail(email)
		if !valid {
			log.Default().Println("Invalid email")
			http.Error(w, "Invalid email", http.StatusBadRequest)
			// TODO: return the login modal with the error message
			return
		}
		validPass, reason := validatePassword(pass, pass)
		if !validPass {
			log.Default().Println("Invalid password: ", reason)
			http.Error(w, reason, http.StatusBadRequest)
			// TODO: return the login modal with the error message
			return
		}
		userId, token, err := m.DB.LoginUser(email, pass)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, "Login Failed", http.StatusInternalServerError)
			// TODO: return the login modal with the error message
			return
		}

		// Set the correct cookies for a logged-in user
		http.SetCookie(w, &http.Cookie{
			Name:     "uuid",
			Value:    userId,
			HttpOnly: true,
			Secure:   true, // Set to true if your site uses HTTPS
			SameSite: http.SameSiteStrictMode,
		})
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    token,
			HttpOnly: true,
			Secure:   true, // Set to true if your site uses HTTPS
			SameSite: http.SameSiteStrictMode,
		})

		// return hx-post targeting the login button to change it to a logout button
		w.Write([]byte(`<div id="confirmLogin" hx-post="/confirm-login" hx-trigger="load" hx-target="#logDiv"> </div>`))
		return

	}
}

func (m *CrawlMaster) SubmitRegister() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		email := r.FormValue("email")
		pass := r.FormValue("password")
		repeatPass := r.FormValue("repeat-password")
		// Validate the email and password
		valid := validateEmail(email)
		if !valid {
			http.Error(w, "Invalid email", http.StatusBadRequest)
			// TODO: return the login modal with the error message
			return
		}
		validPass, reason := validatePassword(pass, repeatPass)
		if !validPass {
			http.Error(w, reason, http.StatusBadRequest)
			// TODO: return the login modal with the error message
			return
		}

		id, err := getRequestCookie(r, "uuid")
		if err != nil {
			log.Default().Println(err)
			http.Error(w, "Failed to get UUID", http.StatusInternalServerError)
			return
		}

		err = m.DB.CreateUser(id, email, pass)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			// TODO: return the login modal with the error message
			return
		}

		// Log the user in
		m.SubmitLogin()(w, r)
	}
}

func (m *CrawlMaster) Logout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Remove the cookies
		clearCookies(w)
		// Render the active_crawlers template, which displays the active crawlers
		serveSuccessToast(w, "Logout Successful")
		tmpl, err := template.ParseFiles("internal/html/templates/login_button.gohtml")
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = tmpl.Execute(w, nil)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (m *CrawlMaster) ServeNetwork() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := checkIfUserLoggedIn(r, w, m)
		if err != nil {
			serveFailToast(w, "User is not logged in")
			return
		}
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			log.Default().Println(err)
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
		err := checkIfUserLoggedIn(r, w, m)
		if err != nil {
			serveFailToast(w, "User is not logged in")
			return
		}
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		cmd := exec.Command("python3", "pkg/data-processor/data_processor.py", "--database", crawlManager.GetDBPath(), "--output", crawlManager.GetNetworkPath())
		// Generate a network file with the processor
		err = cmd.Run()
		if err != nil {
			log.Default().Println(err)
			http.Error(w, "Error generating network file", http.StatusInternalServerError)
			return
		}
		// Render the active_crawlers template, which displays the active crawlers
		tmpl, err := template.ParseFiles("internal/html/templates/network_iframe.gohtml")
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = tmpl.Execute(w, nil)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (m *CrawlMaster) Download() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		fileType := r.URL.Query().Get("type")
		idStr := r.URL.Query().Get("file")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, "Failed to parse file id", http.StatusBadRequest)
			return
		}
		if fileType == "" || idStr == "" {
			http.Error(w, "Missing file type or id", http.StatusBadRequest)
			return
		}

		err = checkIfUserLoggedIn(r, w, m)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		dataPath, err := crawlManager.SqliteDB.DownloadFile(fileType, id)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		// Set the extention and download file name
		fileName := fmt.Sprintf("%s_%d.html", fileType, id)
		if fileType == "Image" {
			ext := filepath.Ext(name)
			fileName = fmt.Sprintf("%s_%d%s", fileType, id, ext)
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, dataPath)
	}
}

func (m *CrawlMaster) ExportDB() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := checkIfUserLoggedIn(r, w, m)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if _, err := os.Stat(crawlManager.GetDBPath()); os.IsNotExist(err) {
			log.Default().Println(err)
			http.Error(w, "Results DB not found", http.StatusNotFound)
			return
		}

		dataPath := crawlManager.GetDBPath()
		filePath := "results.db"
		if r.URL.Query().Get("csv") == "true" {
			// Export the database to a CSV file
			dataPath, err = crawlManager.SqliteDB.ExportToCSV(crawlManager.GetDBPath(), "visited")
			if err != nil {
				log.Default().Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			filePath = "results.csv"
		}

		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filePath))
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, dataPath)

		// Delete the temporary files, if any
		if r.URL.Query().Get("csv") == "true" {
			err := os.Remove(dataPath)
			if err != nil {
				log.Default().Println(err)
			}
		}
	}
}

func (m *CrawlMaster) ExportModal() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := checkIfUserLoggedIn(r, w, m)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if _, err := os.Stat(crawlManager.GetDBPath()); os.IsNotExist(err) {
			log.Default().Println(err)
			http.Error(w, "Results DB not found", http.StatusNotFound)
			return
		}

		tmpl, err := template.ParseFiles("internal/html/templates/export_modal.gohtml")
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = tmpl.Execute(w, nil)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (m *CrawlMaster) CrawlHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := checkIfUserLoggedIn(r, w, m)
		if err != nil {
			serveFailToast(w, "User is not logged in")
			return
		}

		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		r.ParseForm()
		curr_config, err := config.ParseFormToConfig(r.Form, crawlManager.GetDBPath())
		if err != nil {
			log.Default().Println(err)
			curr_config = config.NewDefaultConfig()
			http.Error(w, "Error parsing config settings, using default", http.StatusBadRequest)
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

		// Add the crawler to the map, check the limit
		ctxCrawler, cancel := context.WithCancel(context.Background())
		err = crawlManager.AddCrawlerToMap(curr_config, cancel)
		if err != nil {
			log.Default().Println(err)
			serveFailToast(w, err.Error())
			return
		}

		err = crawlManager.StartCrawlerWithConfig(ctxCrawler, curr_config)
		if err != nil {
			log.Default().Println(err)
			serveFailToast(w, "Error starting crawler: "+curr_config.StartingURL)
			return
		}
	}
}

func (m *CrawlMaster) CrawlRandomHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := checkIfUserLoggedIn(r, w, m)
		if err != nil {
			serveFailToast(w, "User is not logged in")
			return
		}
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			log.Default().Println(err)
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
			log.Default().Println(err)
			curr_config = config.NewDefaultConfig()
			http.Error(w, "Error parsing config settings, using default", http.StatusBadRequest)
		}

		// Add the crawler to the map, check the limit
		ctxCrawler, cancel := context.WithCancel(context.Background())
		err = crawlManager.AddCrawlerToMap(curr_config, cancel)
		if err != nil {
			log.Default().Println(err)
			serveFailToast(w, err.Error())
			return
		}

		err = crawlManager.StartCrawlerWithConfig(ctxCrawler, curr_config)
		if err != nil {
			log.Default().Println(err)
			serveFailToast(w, "Error starting crawler: "+curr_config.StartingURL)
			return
		}
	}
}

func (m *CrawlMaster) KillAllCrawlersHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := checkIfUserLoggedIn(r, w, m)
		if err != nil {
			serveFailToast(w, "User is not logged in")
			return
		}
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			log.Default().Println(err)
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
		err := checkIfUserLoggedIn(r, w, m)
		if err != nil {
			serveFailToast(w, "User is not logged in")
			return
		}
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		url := r.FormValue("url")
		cancel, ok := crawlManager.CrawlMap[url]
		if !ok {
			http.Error(w, "Crawler not found", http.StatusNotFound)
		} else {
			cancel()
		}
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
			log.Default().Println(err)
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
		tmpl, err := template.ParseFiles("internal/html/templates/active_crawlers.gohtml")
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = tmpl.Execute(w, crawlers)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (m *CrawlMaster) RecentURLsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tmpl, err := template.ParseFiles("internal/html/templates/recent_visited.gohtml")
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		visited, err := crawlManager.SqliteDB.GetRecentVisited()
		if err != nil {
			log.Default().Println(err)
		}
		err = tmpl.Execute(w, visited)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
func (m *CrawlMaster) FileCollectionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := checkIfUserLoggedIn(r, w, m)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		crawlManager, err := m.GetCrawlManagerForRequest(r)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Get the recent file collection for the user
		fileType := r.FormValue("fileType")
		fc, err := crawlManager.SqliteDB.GetFilesForType(fileType)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Render the file_collection template, which displays the file collection
		tmpl, err := template.ParseFiles("internal/html/templates/file_collection.gohtml")
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = tmpl.Execute(w, fc)
		if err != nil {
			log.Default().Println(err)
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

func (m *CrawlMaster) ResourceManger() http.HandlerFunc {
	for {
		func() {
			active_users := m.GetRecentlyActiveUsers()
			for _, path := range []string{"user/data-crawler", "user/network", "user/config"} {
				files, err := os.ReadDir(path)
				if err != nil {
					log.Default().Println(err)
					return
				}
				for _, file := range files {
					if len(file.Name()) > 0 {
						id := ""
						if path == "user/data-crawler" {
							id = strings.TrimPrefix(strings.TrimSuffix(file.Name(), ".db"), "results_")
						} else if path == "user/network" {
							id = strings.TrimPrefix(strings.TrimSuffix(file.Name(), ".html"), "network_")
						} else if path == "user/config" {
							id = strings.TrimPrefix(strings.TrimSuffix(file.Name(), ".json"), "config_")
						}
						if _, ok := active_users[id]; !ok {
							err := os.Remove(fmt.Sprintf("%s/%s", path, file.Name()))
							if err != nil {
								log.Default().Println(err)
							}
						}
					}
				}
			}
		}()
		time.Sleep(1 * time.Minute)
	}
}

func (m *CrawlMaster) GetRecentlyActiveUsers() map[string]bool {
	m.RLock()
	defer m.RUnlock()
	active_users := make(map[string]bool)
	for _, crawler := range m.ActiveManagers {
		active_users[crawler.UserID] = true
	}
	// Support users who have been active in the last 3 days
	dbActiveUsers, err := m.DB.GetRecentlyActiveUsers()
	if err != nil {
		log.Default().Println(err)
	} else {
		for _, user := range dbActiveUsers {
			active_users[user] = true
		}
	}

	return active_users
}

func (m *CrawlMaster) AboutModalHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("close") == "true" {
			return
		}

		tmpl, err := template.ParseFiles("internal/html/templates/about_modal.gohtml")
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

func (m *CrawlMaster) EnsureUUIDHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := r.Cookie("uuid")
		if err == http.ErrNoCookie {
			// Cookie does not exist, set it
			token := uuid.New().String()
			http.SetCookie(w, &http.Cookie{
				Name:     "uuid",
				Value:    token,
				HttpOnly: true,
				Secure:   true, // Set to true if your site uses HTTPS
				SameSite: http.SameSiteStrictMode,
			})
		} else if err != nil {
			// Some other error occurred
			http.Error(w, "Failed to read cookie", http.StatusInternalServerError)
		}
	}
}

func (m *CrawlMaster) DismissToastHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
	}
}

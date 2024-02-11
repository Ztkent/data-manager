package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type Config struct {
	StartingURL           string   `json:"starting_url"`
	PermittedDomains      []string `json:"permitted_domains"`
	BlacklistDomains      []string `json:"blacklist_domains"`
	RotateUserAgents      bool     `json:"rotate_user_agents"`
	RespectRobots         bool     `json:"respect_robots"`
	FreeCrawl             bool     `json:"free_crawl"`
	MaxURLsToVisit        int      `json:"max_urls_to_visit"`
	MaxThreads            int      `json:"max_threads"`
	CrawlerTimeout        int      `json:"crawler_timeout"`
	CrawlerRequestTimeout int      `json:"crawler_request_timeout"`
	CrawlerRequestDelayMs int      `json:"crawler_request_delay_ms"`
	CollectHTML           bool     `json:"collect_html"`
	CollectImages         bool     `json:"collect_images"`
	Debug                 bool     `json:"debug"`
	LiveLogging           bool     `json:"live_logging"`
	SqliteEnabled         bool     `json:"sqlite_enabled"`
	SqlitePath            string   `json:"sqlite_path"`
}

func NewDefaultConfig() *Config {
	return &Config{
		StartingURL:           "",
		PermittedDomains:      []string{},
		BlacklistDomains:      []string{},
		RotateUserAgents:      true,
		RespectRobots:         true,
		FreeCrawl:             true,
		MaxURLsToVisit:        5,
		MaxThreads:            10,
		CrawlerTimeout:        3600,
		CrawlerRequestTimeout: 60,
		CrawlerRequestDelayMs: 1000,
		CollectHTML:           false,
		CollectImages:         false,
		Debug:                 false,
		LiveLogging:           false,
		SqliteEnabled:         true,
		SqlitePath:            "pkg/data-crawler/results.db",
	}
}

func StartCrawlerWithConfig(ctx context.Context, config *Config, crawlChan chan string) error {
	// TODO: This should be per user
	json, err := json.Marshal(config)
	if err != nil {
		return err
	}
	path := WriteJsonToFile(json)
	go func() {
		cmd := exec.CommandContext(ctx, "./pkg/data-crawler/v0.1.0/data-crawler", "-c", path)
		err := cmd.Run()
		if err != nil {
			fmt.Println(err)
		}
		// Notify the channel that the crawler is done
		crawlChan <- config.StartingURL
	}()
	return nil
}

func WriteJsonToFile(json []byte) string {
	// TODO: This should be per user
	file, err := os.Create("pkg/data-crawler/config.json")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	file.Write(json)
	return "pkg/data-crawler/config.json"
}

func ParseFormToConfig(form map[string][]string, outputPath string) (*Config, error) {
	config := NewDefaultConfig()
	// Unchecked checkboxes do not appear in the form data.
	config.RotateUserAgents = false
	config.RespectRobots = false
	config.FreeCrawl = false
	config.CollectHTML = false

	for key, values := range form {
		if len(values) > 0 {
			value := values[0]
			switch key {
			case "StartingURL":
				config.StartingURL = value
			case "PermittedDomains":
				config.PermittedDomains = strings.Split(value, ",")
			case "BlacklistDomains":
				config.BlacklistDomains = strings.Split(value, ",")
			case "RotateUserAgents":
				config.RotateUserAgents = (value == "on")
			case "RespectRobots":
				config.RespectRobots = (value == "on")
			case "FreeCrawl":
				config.FreeCrawl = (value == "on")
			case "CollectHTML":
				config.CollectHTML = (value == "on")
			case "CollectImages":
				config.CollectImages = (value == "on")
			case "MaxURLsToVisit", "CrawlerTimeout", "CrawlerRequestTimeout", "CrawlerRequestDelayMs":
				intValue, err := strconv.Atoi(value)
				if err != nil {
					return nil, err
				}
				switch key {
				case "MaxURLsToVisit":
					config.MaxURLsToVisit = intValue
				case "CrawlerTimeout":
					config.CrawlerTimeout = intValue
				case "CrawlerRequestTimeout":
					config.CrawlerRequestTimeout = intValue
				case "CrawlerRequestDelayMs":
					config.CrawlerRequestDelayMs = intValue
				}
			}
		}
	}
	if !config.FreeCrawl && config.StartingURL != "" &&
		len(config.PermittedDomains) == 1 && config.PermittedDomains[0] == "" {
		// If no permitted domains are specified, use the starting URL as the only permitted domain
		url := config.StartingURL
		re := regexp.MustCompile(`^https?://`)
		url = re.ReplaceAllString(url, "")
		if !strings.HasPrefix(url, "www.") {
			url = "www." + url
		}
		config.PermittedDomains[0] = url
	}
	config.SqlitePath = outputPath
	return config, nil
}

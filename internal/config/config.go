package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
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
		StartingURL:           "https://www.cnn.com",
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
	file, err := os.Create("pkg/data-crawler/config.json")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	file.Write(json)
	return "pkg/data-crawler/config.json"
}

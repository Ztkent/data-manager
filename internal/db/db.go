package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"           // PostgreSQL driver
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

type database struct {
	db *sql.DB
}

type MasterDatabase interface {
}

type ManagerDatabase interface {
	GetRecentVisited() ([]Visited, error)
}

func ConnectSqlite(filePath string) *sql.DB {
	db, err := sql.Open("sqlite3", filePath)
	if err != nil {
		log.Default().Println(err)
		return nil
	}
	return db
}

func NewManagerDatabase(db *sql.DB) ManagerDatabase {
	return &database{db: db}
}

type Visited struct {
	ID            int
	URL           string
	Referrer      string
	LastVisitedAt time.Time
	IsComplete    bool
	IsBlocked     bool
}

func (db *database) GetRecentVisited() ([]Visited, error) {
	rows, err := db.db.Query(`
        SELECT id, url, referrer, last_visited_at, is_complete, is_blocked
        FROM visited
        ORDER BY last_visited_at DESC
        LIMIT 25
    `)
	if err != nil {
		return nil, fmt.Errorf("could not query sqlite: %v", err)
	}
	defer rows.Close()
	var visiteds []Visited
	for rows.Next() {
		var v Visited
		if err := rows.Scan(&v.ID, &v.URL, &v.Referrer, &v.LastVisitedAt, &v.IsComplete, &v.IsBlocked); err != nil {
			return nil, fmt.Errorf("could not scan sqlite: %v", err)
		}
		visiteds = append(visiteds, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate sqlite: %v", err)
	}

	sort.Slice(visiteds, func(i, j int) bool {
		if visiteds[i].LastVisitedAt.Equal(visiteds[j].LastVisitedAt) {
			return visiteds[i].ID > visiteds[j].ID
		}
		return visiteds[i].LastVisitedAt.After(visiteds[j].LastVisitedAt)
	})

	return visiteds, nil
}

func ConnectRedis() (*redis.Client, error) {
	// Get the Redis connection details from the environment variables
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")
	redisUser := os.Getenv("REDIS_USER")

	// Connect to the Redis instance
	client := redis.NewClient(&redis.Options{
		Addr:     redisHost + ":" + redisPort,
		Username: redisUser,
		DB:       0,
	})

	// Test the connection
	ctx5, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Ping(ctx5).Result()
	if err != nil {
		return nil, err
	}
	return client, nil
}

func ConnectPostgres() (*sql.DB, error) {
	// Get the PostgreSQL connection details from the environment variables
	postgresHost := os.Getenv("POSTGRES_HOST")
	postgresPort := os.Getenv("POSTGRES_PORT")
	postgresUser := os.Getenv("POSTGRES_USER")
	postgresPassword := os.Getenv("POSTGRES_PASSWORD")
	postgresDB := os.Getenv("POSTGRES_DB")

	// Create the connection string
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		postgresHost, postgresPort, postgresUser, postgresPassword, postgresDB)

	// Connect to the PostgreSQL instance
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// Test the connection
	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}

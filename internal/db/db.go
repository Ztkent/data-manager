package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type database struct {
	db *sql.DB
}

type Database interface {
	GetRecentVisited() ([]Visited, error)
}

func ConnectSqlite() *sql.DB {
	db, err := sql.Open("sqlite3", "pkg/data-crawler/results.db")
	if err != nil {
		log.Default().Println(err)
		return nil
	}
	return db
}

func NewDatabase(db *sql.DB) Database {
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
	return visiteds, nil
}

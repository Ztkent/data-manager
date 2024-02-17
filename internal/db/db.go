package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	_ "github.com/lib/pq"           // PostgreSQL driver
	_ "github.com/mattn/go-sqlite3" // SQLite driver
	"golang.org/x/crypto/bcrypt"
)

type database struct {
	db *sql.DB
}

type MasterDatabase interface {
	CreateUser(userID, email, password string) error
	LoginUser(email, password string) (string, string, error)
	UpdateUserAuth(userID, token string) error
	GetRecentlyActiveUsers() ([]string, error)
}

type ManagerDatabase interface {
	GetRecentVisited() ([]Visited, error)
}

func NewManagerDatabase(db *sql.DB) ManagerDatabase {
	return &database{db: db}
}

func NewMasterDatabase(db *sql.DB) MasterDatabase {
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

func (db *database) CreateUser(userID, email, password string) error {
	if db.db == nil {
		return fmt.Errorf("database is nil")
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("could not hash password: %v", err)
	}
	_, err = db.db.Exec(`
        INSERT INTO users (user_id, email, password, created_at, updated_at)
        VALUES ($1, $2, $3, NOW(), NOW())
    `, userID, email, hashedPassword)
	if err != nil {
		// if users want to create a second account while we have a cookie active, we need to generate a new user_id
		if strings.Contains(err.Error(), "users_user_id_key") {
			userID = uuid.New().String()
		}
		_, err = db.db.Exec(`
        INSERT INTO users (user_id, email, password, created_at, updated_at)
        VALUES ($1, $2, $3, NOW(), NOW())
    `, userID, email, hashedPassword)
	}
	return err
}

func (db *database) LoginUser(email, password string) (string, string, error) {
	if db.db == nil {
		return "", "", fmt.Errorf("database is nil")
	}
	var userId string
	var hashedPassword string
	err := db.db.QueryRow(`
		SELECT user_id, password
		FROM users
		WHERE email = $1
	`, email).Scan(&userId, &hashedPassword)
	if err != nil {
		return "", "", fmt.Errorf("could not find user: %v", err)
	}
	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		return "", "", fmt.Errorf("could not compare password: %v", err)
	}

	token, err := generateJWT()
	if err != nil {
		return "", "", fmt.Errorf("could not generate JWT: %v", err)
	}

	err = db.UpdateUserAuth(userId, token)
	if err != nil {
		return "", "", fmt.Errorf("could not update user auth: %v", err)
	}
	return userId, token, nil
}

func (db *database) UpdateUserAuth(userID, token string) error {
	if db.db == nil {
		return fmt.Errorf("database is nil")
	}
	_, err := db.db.Exec(`
        INSERT INTO auth (user_id, session_token, created_at, updated_at)
        VALUES ($1, $2, NOW(), NOW())
        ON CONFLICT (user_id) DO UPDATE 
        SET session_token = $2, updated_at = NOW()
    `, userID, token)
	if err != nil {
		return fmt.Errorf("could not upsert user auth: %v", err)
	}
	return nil
}

func (db *database) GetRecentVisited() ([]Visited, error) {
	if db.db == nil {
		return nil, fmt.Errorf("database is nil")
	}
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

func (db *database) GetRecentlyActiveUsers() ([]string, error) {
	if db.db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	rows, err := db.db.Query(`
		SELECT user_id
		FROM auth
		WHERE updated_at > NOW() - INTERVAL '72 hours'
	`)
	if err != nil {
		return nil, fmt.Errorf("could not query postgres: %v", err)
	}
	defer rows.Close()
	var users []string
	for rows.Next() {
		var user string
		if err := rows.Scan(&user); err != nil {
			return nil, fmt.Errorf("could not scan postgres: %v", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate postgres: %v", err)
	}
	return users, nil
}

func ConnectSqlite(filePath string) *sql.DB {
	db, err := connectWithBackoff("sqlite3", filePath, 3)
	if err != nil {
		return nil
	}
	return db
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
	db, err := connectWithBackoff("postgres", connStr, 3)
	if err != nil {
		return nil, err
	}
	// Test the connection
	err = db.Ping()
	if err != nil {
		return nil, err
	}

	// Run the migrations
	err = RunMigrations(db)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func RunMigrations(db *sql.DB) error {
	// Read the migration directory
	files, err := os.ReadDir("internal/migration")
	if err != nil {
		return err
	}

	// Filter and sort the files
	sqlFiles := make([]fs.DirEntry, 0)
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".sql") {
			sqlFiles = append(sqlFiles, file)
		}
	}
	sort.Slice(sqlFiles, func(i, j int) bool {
		return sqlFiles[i].Name() < sqlFiles[j].Name()
	})

	// Execute each file as a SQL script
	for _, file := range sqlFiles {
		data, err := os.ReadFile("internal/migration/" + file.Name())
		if err != nil {
			return err
		}
		_, err = db.Exec(string(data))
		if err != nil {
			return err
		}
	}

	return nil
}

func generateJWT() (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": time.Now().Add(time.Hour * 24).Unix(),
	})
	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET_TOKEN")))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func connectWithBackoff(driver string, connStr string, maxRetries int) (*sql.DB, error) {
	var db *sql.DB
	var err error
	for i := 0; i < maxRetries; i++ {
		db, err = sql.Open(driver, connStr)
		if err != nil {
			fmt.Println("Failed attempt to connect to " + driver + ": " + err.Error())
			time.Sleep(time.Duration(i+1) * (3 * time.Second))
			continue
		}
		err = db.Ping()
		if err != nil {
			fmt.Println("Failed attempt to connect to " + driver + ": " + err.Error())
			time.Sleep(time.Duration(i+1) * (3 * time.Second))
			continue
		}
		return db, nil
	}
	return nil, err
}

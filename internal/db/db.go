package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
	ConfirmUUIDandToken(userID, token string) error
}

type ManagerDatabase interface {
	GetRecentVisited() ([]Visited, error)
	ExportToCSV(path string, table string) (string, error)
	GetFilesForType(fileType string) (FileCollection, error)
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

type File struct {
	ID       int
	FileName string
	FileType string
	FileSize string
	FileDate string
}

type FileCollection struct {
	FileType string
	Files    []File
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

func (db *database) ExportToCSV(path string, table string) (string, error) {
	if db.db == nil {
		return "", fmt.Errorf("database is nil")
	}

	// Create a new CSV file
	uuid := uuid.New().String()
	fileName := table + "_" + uuid + ".csv"
	filePath := filepath.Join("user/data-crawler/", fileName)
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("could not create file: %v", err)
	}
	defer file.Close()
	rows, err := db.db.Query("SELECT * FROM " + table)
	if err != nil {
		return "", fmt.Errorf("could not query sqlite: %v", err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("could not get columns: %v", err)
	}
	// Write the header row
	for i, colName := range columns {
		if i > 0 {
			file.WriteString(",")
		}
		file.WriteString(colName)
	}
	file.WriteString("\n")

	// Write the data rows
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for rows.Next() {
		for i := range columns {
			valuePtrs[i] = &values[i]
		}
		rows.Scan(valuePtrs...)
		for i := range columns {
			if i > 0 {
				file.WriteString(",")
			}
			switch v := values[i].(type) {
			case int64:
				file.WriteString(fmt.Sprintf("%d", v))
			case bool:
				file.WriteString(fmt.Sprintf("%t", v))
			case time.Time:
				file.WriteString(v.Format("2006-01-02 15:04:05"))
			default:
				file.WriteString(fmt.Sprintf("%s", v))
			}
		}
		file.WriteString("\n")
	}

	return filePath, nil
}

func (db *database) GetFilesForType(fileType string) (FileCollection, error) {
	if db.db == nil {
		return FileCollection{}, fmt.Errorf("database is nil")
	}

	var query string
	switch fileType {
	case "HTML":
		query = "SELECT id, url, html, updated_at FROM html ORDER BY updated_at DESC LIMIT 50"
	case "Image":
		query = "SELECT id, referrer, url, image, name, updated_at FROM images WHERE success = 1 AND image IS NOT NULL ORDER BY updated_at DESC LIMIT 50"
	default:
		return FileCollection{}, fmt.Errorf(fmt.Sprintf("invalid file type: %s", fileType))
	}

	rows, err := db.db.Query(query)
	if err != nil {
		return FileCollection{}, fmt.Errorf("could not query sqlite: %v", err)
	}
	defer rows.Close()
	var files []File
	for rows.Next() {
		var f File
		if fileType == "HTML" {
			var id int
			var url string
			var html string
			var updatedAt time.Time
			if err := rows.Scan(&id, &url, &html, &updatedAt); err != nil {
				return FileCollection{}, fmt.Errorf("could not scan sqlite: %v", err)
			}
			f.ID = id
			f.FileName = url
			f.FileType = "HTML"
			f.FileSize = fmt.Sprintf("%d", len(html))
			f.FileDate = updatedAt.Format("2006-01-02 15:04:05")
		} else {
			var id int
			var referrer string
			var url string
			var image string
			var name string
			var updatedAt time.Time

			if err := rows.Scan(&id, &referrer, &url, &image, &name, &updatedAt); err != nil {
				return FileCollection{}, fmt.Errorf("could not scan sqlite: %v", err)
			}
			f.ID = id
			f.FileName = url
			f.FileType = "Image"
			f.FileSize = fmt.Sprintf("%d", len(image))
			f.FileDate = updatedAt.Format("2006-01-02 15:04:05")
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return FileCollection{}, fmt.Errorf("could not iterate sqlite: %v", err)
	}

	return FileCollection{
		FileType: fileType,
		Files:    files,
	}, nil
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

func (db *database) ConfirmUUIDandToken(userID, token string) error {
	if db.db == nil {
		return fmt.Errorf("database is nil")
	}

	// confirm that the uuid and token match what we have in the database
	row := db.db.QueryRow("SELECT COUNT(*) FROM auth WHERE user_id = $1 AND session_token = $2", userID, token)

	var count int
	err := row.Scan(&count)
	if err != nil {
		return fmt.Errorf("could not query user auth: %v", err)
	}

	if count == 0 {
		return fmt.Errorf("uuid and token do not match")
	}

	return nil
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

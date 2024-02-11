package routes

import (
	"bufio"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt"
)

func generateJWT() (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": time.Now().Add(time.Hour * 24).Unix(),
	})
	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_TOKEN")))
	if err != nil {
		return "", err
	}
	return tokenString, nil
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

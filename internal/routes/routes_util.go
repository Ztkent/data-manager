package routes

import (
	"bufio"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
)

type Toast struct {
	ToastContent string
	Border       string
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

func validateStartingURL(startingURL string) (bool, string) {
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

func serveFailToast(w http.ResponseWriter, message string) {
	// Render the crawl_status template, which displays the toast
	tmpl, err := template.ParseFiles("html/templates/crawl_status_toast.gohtml")
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
	tmpl, err := template.ParseFiles("html/templates/crawl_status_toast.gohtml")
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

func logForm(r *http.Request) {
	r.ParseForm()
	for key, values := range r.Form {
		for _, value := range values {
			log.Printf("Form key: %s, value: %s\n", key, value)
		}
	}
}

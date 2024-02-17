package routes

import (
	"bufio"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"
)

type Toast struct {
	ToastContent string
	Border       string
}

func validatePassword(password string, repeatPass string) (bool, string) {
	if password != repeatPass {
		return false, "Passwords do not match."
	}
	if len(password) < 8 {
		return false, "Password must be at least 8 characters long."
	}
	hasUppercase := false
	hasLowercase := false
	hasNumber := false

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUppercase = true
		case unicode.IsLower(char):
			hasLowercase = true
		case unicode.IsNumber(char):
			hasNumber = true
		}
	}
	if !hasUppercase {
		return false, "Password must contain at least one uppercase letter."
	}
	if !hasLowercase {
		return false, "Password must contain at least one lowercase letter."
	}
	if !hasNumber {
		return false, "Password must contain at least one number."
	}
	return true, ""
}

func validateEmail(email string) bool {
	emailRegex := `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
	validEmail := regexp.MustCompile(emailRegex)
	return validEmail.MatchString(email)
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
	tmpl, err := template.ParseFiles("internal/html/templates/crawl_status_toast.gohtml")
	if err != nil {
		log.Default().Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	toast := &Toast{ToastContent: message, Border: "border-red-200"}
	err = tmpl.Execute(w, toast)
	if err != nil {
		log.Default().Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

func serveSuccessToast(w http.ResponseWriter, message string) {
	// Render the crawl_status template, which displays the toast
	tmpl, err := template.ParseFiles("internal/html/templates/crawl_status_toast.gohtml")
	if err != nil {
		log.Default().Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	toast := &Toast{ToastContent: message, Border: "border-green-200"}
	err = tmpl.Execute(w, toast)
	if err != nil {
		log.Default().Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

func getRequestCookie(r *http.Request, name string) (string, error) {
	cookie, err := r.Cookie(name)
	if err == http.ErrNoCookie {
		return "", fmt.Errorf("Cookie not found")
	}
	return cookie.Value, nil
}

func clearCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:    "uuid",
		Value:   "",
		Expires: time.Unix(0, 0),
		Path:    "/",
	})
	http.SetCookie(w, &http.Cookie{
		Name:    "session_token",
		Value:   "",
		Expires: time.Unix(0, 0),
		Path:    "/",
	})
}

func logForm(r *http.Request) {
	r.ParseForm()
	for key, values := range r.Form {
		for _, value := range values {
			log.Printf("Form key: %s, value: %s\n", key, value)
		}
	}
}

package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

var sessions = map[string]string{} // sessionID -> userID

func generateSessionID() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("Failed to generate session ID: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// School structure
type School struct {
	ID   string `json:"id"` // Fix C: Changed single quotes to backticks (`)
	Name string `json:"name"`
}

type Grade struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Equipment struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

type EquipmentListResponse struct {
	Items []Equipment `json:"items"`
}

func JSONError(w http.ResponseWriter, err string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	errEncode := json.NewEncoder(w).Encode(map[string]string{"error": err})
	if errEncode != nil {
		log.Printf("Failed to encode JSON error response: %v", errEncode)
	}
}

// =====NEW=====
// login
type User struct {
	UserID   string `json:"userid"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Allow only the frontend origin
		allowed_origin := os.Getenv("CLIENT_ORIGIN")
		if allowed_origin == "" {
			allowed_origin = "http://localhost:3000" // default for local development
		}
		if origin == allowed_origin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		// For production, use your real frontend URL above

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		// NEW - changing the value
		w.Header().Set(
			"Access-Control-Allow-Headers",
			"Content-Type, Authorization",
		)
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	}
}

func main() {

	// NEW - for credit card API
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found")
	}

	// Handler for getSchools, getGrades, getEquipment
	http.HandleFunc("/api/schools", enableCORS(getSchoolsHandler))
	http.HandleFunc("/api/grades", enableCORS(getGradesHandler))
	http.HandleFunc("/api/equipment", enableCORS(getEquipmentListsHandler))
	http.HandleFunc("/api/auth/status", enableCORS(authStatusHandler))
	http.HandleFunc("/api/login", enableCORS(postLoginHandler))
	http.HandleFunc("/api/logout", enableCORS(logoutHandler))
	http.HandleFunc("/api/cart", enableCORS(getPostCartHandler))
	http.HandleFunc("/api/create-checkout-session", enableCORS(CreateCheckoutSession))

	// Start the API Gateway server
	port := "8080" // Changed port to string without colon for easier fmt use
	// Using fmt.Sprintf to format the port with a colon for ListenAndServe
	serverAddr := fmt.Sprintf(":%s", port)

	// New - supporting remote DB
	InitDB()

	// Fix E: Corrected format specifier to %s
	fmt.Printf("API Gateway starting on port %s\n", port)

	// Use the formatted address to listen
	log.Fatal(http.ListenAndServe(serverAddr, nil))
}
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

var sessions = map[string]string{} // sessionID -> userID

func generateSessionID() string {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
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

type User struct {
	UserID   string `json:"userid"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// NEW - adding purchase history
type OrderItem struct {
	EquipmentName string  `json:"equipment_name"`
	Quantity      int     `json:"quantity"`
	Price         float64 `json:"price"`
	TotalPrice    float64 `json:"total_price"`
}

type Order struct {
	ID           string      `json:"order_id"`
	GradeID      string      `json:"grade_id"`
	PurchaseDate string      `json:"purchase_date"`
	TotalAmount  float64     `json:"total_amount"`
	Items        []OrderItem `json:"items"`
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

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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
	http.HandleFunc("/create-checkout-session", enableCORS(CreateCheckoutSession))
	http.HandleFunc("/api/history", enableCORS(getOrderHistoryHandler))

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

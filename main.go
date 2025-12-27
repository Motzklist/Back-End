package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"
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

type Class struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Equipment struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

type EquipmentListResponse struct {
	Items []Equipment `json:"items"`
}

func JSONError(w http.ResponseWriter, err string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err})
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
		// Allow requests from any origin during development (change this for production!)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	}
}

func main() {
	// Handler for getSchools, getGrades, getClasses, getEquipment
	http.HandleFunc("/api/schools", enableCORS(getSchoolsHandler))
	http.HandleFunc("/api/grades", enableCORS(getGradesHandler))
	http.HandleFunc("/api/classes", enableCORS(getClassesHandler))
	http.HandleFunc("/api/equipment", enableCORS(getEquipmentListsHandler))
	http.HandleFunc("/api/auth/status", enableCORS(authStatusHandler))
	http.HandleFunc("/api/login", enableCORS(postLoginHandler))
	http.HandleFunc("/api/cart", enableCORS(getPostCartHandler))

	// Start the API Gateway server
	port := "8080" // Changed port to string without colon for easier fmt use
	// Using fmt.Sprintf to format the port with a colon for ListenAndServe
	serverAddr := fmt.Sprintf(":%s", port)

	// Fix E: Corrected format specifier to %s
	fmt.Printf("API Gateway starting on port %s\n", port)

	// Use the formatted address to listen
	log.Fatal(http.ListenAndServe(serverAddr, nil))
}

func getSchoolsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// LATER: connect to database, extract corresponding list and parse it
	schools := GetSchools()

	// Convert to Json
	if err := json.NewEncoder(w).Encode(schools); err != nil {
		JSONError(w, "Failed to encode schools response", http.StatusInternalServerError)
		log.Printf("Error encoding response: %v", err)
		return
	}
	log.Printf("Successfully served /api/schools request")
}

func getGradesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract the school_id query parameter
	schoolID := r.URL.Query().Get("school_id")

	// 1. Input Validation: Check if the required parameter is missing
	if schoolID == "" {
		JSONError(w, "Missing required query parameter: school_id", http.StatusBadRequest)
		return
	}

	log.Printf("Received request for grades in school ID: %s", schoolID)

	// LATER: The mock data here would be filtered based on schoolID
	// For now, we return the full mock list regardless of the ID.

	// LATER: connect to database, extract corresponding list and parse it

	grades := GetGradesBySchoolID(schoolID)

	// Convert to Json
	if err := json.NewEncoder(w).Encode(grades); err != nil {
		JSONError(w, "Failed to encode grades response", http.StatusInternalServerError)
		log.Printf("Error encoding response: %v", err)
		return
	}
	log.Printf("Successfully served /api/grades request")
}

func getClassesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract the required query parameters
	schoolID := r.URL.Query().Get("school_id")
	gradeID := r.URL.Query().Get("grade_id")

	// 1. Input Validation: Check if any required parameter is missing
	if schoolID == "" || gradeID == "" {
		JSONError(w, "Missing required query parameter: school_id, grade_id", http.StatusBadRequest)
		return
	}

	log.Printf("Received request for classes in school ID: %s, Grade ID: %s", schoolID, gradeID)

	// LATER: The mock data here would be filtered based on schoolID and gradeID
	// For now, we return the full mock list regardless of the IDs.

	// LATER: connect to database, extract corresponding list and parse it
	classes := GetClassesByGradeID(schoolID, gradeID)

	// Convert to Json
	if err := json.NewEncoder(w).Encode(classes); err != nil {
		JSONError(w, "Failed to encode classes response", http.StatusInternalServerError)
		log.Printf("Error encoding response: %v", err)
		return
	}
	log.Printf("Successfully served /api/classes request")

}

func getEquipmentListsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract the required query parameters (unchanged)
	schoolID := r.URL.Query().Get("school_id")
	gradeID := r.URL.Query().Get("grade_id")
	classID := r.URL.Query().Get("class_id")

	// 1. Input Validation (unchanged)
	if schoolID == "" || gradeID == "" || classID == "" {
		JSONError(w, "Missing required query parameters: school_id, grade_id, or class_id", http.StatusBadRequest)
		return
	}

	log.Printf("Received request for equipment list: School=%s, Grade=%s, Class=%s", schoolID, gradeID, classID)

	// LATER: connect to database, extract corresponding list and parse it
	equipment := GetEquipmentList(schoolID, gradeID, classID)

	// --- CRITICAL CHANGE STARTS HERE ---
	// 1. Wrap the equipment slice into the structured response object
	response := EquipmentListResponse{
		Items: equipment,
	}

	// 2. Encode the structured response object
	if err := json.NewEncoder(w).Encode(response); err != nil {
		JSONError(w, "Failed to encode equipment response", http.StatusInternalServerError)
		log.Printf("Error encoding response: %v", err)
		return
	}
	log.Printf("Successfully served /api/equipment request")
}

// =====NEW=====
// adding handlers to login page & shopping cart
func authStatusHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("sessionid")
	if err != nil {
		JSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID, exists := sessions[cookie.Value]
	if !exists {
		JSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	for _, user := range MockUsers {
		if user.UserID == userID {
			json.NewEncoder(w).Encode(map[string]string{"userid": user.UserID, "username": user.Username})
			return
		}
	}

	JSONError(w, "Unauthorized", http.StatusUnauthorized)
}

func postLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		JSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		JSONError(w, "Failed to decode request body", http.StatusBadRequest)
		return
	}

	for _, user := range MockUsers {
		if user.Username == credentials.Username && user.Password == credentials.Password {
			// Session generation
			sessionID := generateSessionID()
			sessions[sessionID] = user.UserID

			// Cookie setting
			http.SetCookie(w, &http.Cookie{
				Name:     "sessionid",
				Value:    sessionID,
				Path:     "/",
				HttpOnly: true,
				//Secure:	true, // Uncomment this line if using HTTPS
				//SameSite: http.SameSiteStrictMode,
			})
			json.NewEncoder(w).Encode(map[string]string{"userid": user.UserID, "username": user.Username})
			return
		}
	}

	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

func getPostCartHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userid")
	if userID == "" {
		JSONError(w, "Missing required query parameter: userid", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Return existing cart
		cart, exists := MockCarts[userID]
		if !exists {
			cart = []Equipment{} // Return empty list if no cart exists
		}
		json.NewEncoder(w).Encode(cart)

	case http.MethodPost, http.MethodPut:
		// Update the cart
		var newItems []Equipment
		if err := json.NewDecoder(r.Body).Decode(&newItems); err != nil {
			http.Error(w, "Invalid data", http.StatusBadRequest)
			return
		}
		MockCarts[userID] = newItems
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Cart updated successfully")

	default:
		JSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

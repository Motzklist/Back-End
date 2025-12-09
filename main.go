package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

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
		http.Error(w, "Failed to encode schools response", http.StatusInternalServerError)
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
		http.Error(w, "Missing required query parameter: school_id", http.StatusBadRequest)
		return
	}

	log.Printf("Received request for grades in school ID: %s", schoolID)

	// LATER: The mock data here would be filtered based on schoolID
	// For now, we return the full mock list regardless of the ID.

	// LATER: connect to database, extract corresponding list and parse it

	grades := GetGradesBySchoolID(schoolID)

	// Convert to Json
	if err := json.NewEncoder(w).Encode(grades); err != nil {
		http.Error(w, "Failed to encode grades response", http.StatusInternalServerError)
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
		http.Error(w, "Missing required query parameters: school_id or grade_id", http.StatusBadRequest)
		return
	}

	log.Printf("Received request for classes in school ID: %s, Grade ID: %s", schoolID, gradeID)

	// LATER: The mock data here would be filtered based on schoolID and gradeID
	// For now, we return the full mock list regardless of the IDs.

	// LATER: connect to database, extract corresponding list and parse it
	classes := GetClassesByGradeID(schoolID, gradeID)

	// Convert to Json
	if err := json.NewEncoder(w).Encode(classes); err != nil {
		http.Error(w, "Failed to encode classes response", http.StatusInternalServerError)
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
		http.Error(w, "Missing required query parameters: school_id, grade_id, or class_id", http.StatusBadRequest)
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
		http.Error(w, "Failed to encode equipment response", http.StatusInternalServerError)
		log.Printf("Error encoding response: %v", err)
		return
	}
	log.Printf("Successfully served /api/equipment request")
}

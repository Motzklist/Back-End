package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

func getSchoolsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// LATER: connect to database, extract corresponding list and parse it
	schools := getSchools()

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
	if _, err := strconv.Atoi(schoolID); err != nil {
		JSONError(w, "school_id must be an integer", http.StatusBadRequest)
		return
	}

	log.Printf("Received request for grades in school ID: %s", schoolID)

	// LATER: The mock data here would be filtered based on schoolID
	// For now, we return the full mock list regardless of the ID.

	// LATER: connect to database, extract corresponding list and parse it

	grades := getGrades(schoolID)

	// Convert to Json
	if err := json.NewEncoder(w).Encode(grades); err != nil {
		JSONError(w, "Failed to encode grades response", http.StatusInternalServerError)
		log.Printf("Error encoding response: %v", err)
		return
	}
	log.Printf("Successfully served /api/grades request")
}

func getEquipmentListsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract the required query parameters (updated)
	schoolID := r.URL.Query().Get("school_id")
	gradeID := r.URL.Query().Get("grade_id")

	// 1. Input Validation (updated)
	if schoolID == "" || gradeID == "" {
		JSONError(w, "Missing required query parameters: school_id or grade_id", http.StatusBadRequest)
		return
	}
	if _, err := strconv.Atoi(schoolID); err != nil {
		JSONError(w, "school_id must be an integer", http.StatusBadRequest)
		return
	}
	if _, err := strconv.Atoi(gradeID); err != nil {
		JSONError(w, "grade_id must be an integer", http.StatusBadRequest)
		return
	}

	log.Printf("Received request for equipment list: School=%s, Grade=%s", schoolID, gradeID)

	// LATER: connect to database, extract corresponding list and parse it
	equipment := getEquipment(schoolID, gradeID)

	response := EquipmentListResponse{
		Items: equipment,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		JSONError(w, "Failed to encode equipment response", http.StatusInternalServerError)
		log.Printf("Error encoding response: %v", err)
		return
	}
	log.Printf("Successfully served /api/equipment request")
}

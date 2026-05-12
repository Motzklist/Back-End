package main

import (
	"encoding/json"
	"log"
	"net/http"
)

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

	var username = getUsernameFromUserID(userID)
	if username != "" {
		err := json.NewEncoder(w).Encode(map[string]string{"userid": userID, "username": username})
		if err != nil {
			log.Printf("Failed to encode auth status response: %v", err)
		}
		return
	} else {
		JSONError(w, "Unauthorized", http.StatusUnauthorized)
	}
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

	var userId = getUserIDByCredentials(credentials.Username, credentials.Password)

	if userId == "" {
		JSONError(w, "Incorrect username or password. Please try again.", http.StatusUnauthorized)
	} else {
		// Session generation
		sessionID := generateSessionID()
		sessions[sessionID] = userId

		// Cookie setting
		http.SetCookie(w, &http.Cookie{
			Name:     "sessionid",
			Value:    sessionID,
			Path:     "/",
			HttpOnly: true,
			//Secure: true, // Uncomment this line if using HTTPS
			//SameSite: http.SameSiteStrictMode,
		})
		err := json.NewEncoder(w).Encode(map[string]string{"userid": userId, "username": credentials.Username})
		if err != nil {
			log.Printf("Failed to encode login response: %v", err)
		}
		return
	}
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("sessionid")
	if err == nil {
		delete(sessions, cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "sessionid",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	w.WriteHeader(http.StatusOK)
}

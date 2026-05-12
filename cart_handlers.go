package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func getPostCartHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userid")
	if userID == "" {
		JSONError(w, "Missing required query parameter: userid", http.StatusBadRequest)
		return
	}

	// TODO: make sure the front sends a POST request if items were picked
	switch r.Method {
	case http.MethodGet:
		// Return existing cart (now returns []CartEntry)
		cart := getCartByUserID(userID)
		err := json.NewEncoder(w).Encode(cart)
		if err != nil {
			log.Printf("Failed to encode cart response: %v", err)
		}

	case http.MethodPost, http.MethodPut:
		// Update the cart (expects []CartEntry)
		var newEntries []CartEntry
		if err := json.NewDecoder(r.Body).Decode(&newEntries); err != nil {
			JSONError(w, "Failed to decode request body", http.StatusBadRequest)
			return
		}
		saveCart(userID, newEntries)
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprintf(w, "Cart updated successfully"); err != nil {
			log.Printf("Failed to write cart update response: %v", err)
		}

	default:
		JSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

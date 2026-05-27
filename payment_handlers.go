package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
)

type CheckoutRequest struct {
	ProductName string `json:"productName"`
	Quantity    int64  `json:"quantity"`
	Amount      int64  `json:"amount"`
}

type CheckoutResponse struct {
	URL string `json:"url"`
}

func CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		JSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")

	var req CheckoutRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		JSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ProductName == "" || req.Quantity <= 0 || req.Amount <= 0 {
		JSONError(w, "productName, quantity, and amount must be provided and positive", http.StatusBadRequest)
		return
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		log.Println("FRONTEND_URL is not configured")
		JSONError(w, "Server misconfigured: FRONTEND_URL is not set", http.StatusInternalServerError)
		return
	}

	params := &stripe.CheckoutSessionParams{
		SuccessURL: stripe.String(frontendURL + "/payment/success?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String(frontendURL + "/payment/cancel"),
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Quantity: stripe.Int64(req.Quantity),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("ils"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(req.ProductName),
					},
					UnitAmount: stripe.Int64(req.Amount),
				},
			},
		},
	}

	s, err := session.New(params)
	if err != nil {
		JSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := CheckoutResponse{
		URL: s.URL,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode checkout session response: %v", err)
	}
}

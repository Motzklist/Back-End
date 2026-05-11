package main

import (
	"encoding/json"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"net/http"
	"os"
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")

	var req CheckoutRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	frontendURL := os.Getenv("FRONTEND_URL")

	params := &stripe.CheckoutSessionParams{
		SuccessURL: stripe.String(frontendURL + "/payment/success"),
		CancelURL:  stripe.String(frontendURL + "/payment/cancel"),
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Quantity: stripe.Int64(req.Quantity),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("usd"),
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := CheckoutResponse{
		URL: s.URL,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

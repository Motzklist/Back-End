package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
)

// CheckoutLineItem is one purchasable line in a structured checkout. Amount is
// the unit price in agorot (the ILS minor unit), matching Stripe's convention.
type CheckoutLineItem struct {
	EquipmentID string `json:"equipmentId"`
	Name        string `json:"name"`
	Quantity    int64  `json:"quantity"`
	Amount      int64  `json:"amount"`
}

type CheckoutRequest struct {
	// Legacy single-item fields (kept for backwards compatibility).
	ProductName string `json:"productName"`
	Quantity    int64  `json:"quantity"`
	Amount      int64  `json:"amount"`

	// Structured order context. When present, the webhook uses this metadata to
	// persist the completed purchase into orders/order_item.
	UserID  string             `json:"userId"`
	GradeID string             `json:"gradeId"`
	Items   []CheckoutLineItem `json:"items"`
}

type CheckoutResponse struct {
	URL string `json:"url"`
}

// metaItem is the compact per-line shape stored in Stripe session metadata
// (kept short so the JSON blob stays under Stripe's 500-char metadata limit):
// e = equipment id, q = quantity, a = unit amount in agorot.
type metaItem struct {
	E string `json:"e"`
	Q int64  `json:"q"`
	A int64  `json:"a"`
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

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		log.Println("FRONTEND_URL is not configured")
		JSONError(w, "Server misconfigured: FRONTEND_URL is not set", http.StatusInternalServerError)
		return
	}

	// Build the Stripe line items either from the structured cart (preferred) or
	// from the legacy single-product fields.
	var lineItems []*stripe.CheckoutSessionLineItemParams
	var metaItems []metaItem
	if len(req.Items) > 0 {
		for _, it := range req.Items {
			if it.Quantity <= 0 || it.Amount < 0 {
				JSONError(w, "each item needs a positive quantity and non-negative amount", http.StatusBadRequest)
				return
			}
			lineItems = append(lineItems, &stripe.CheckoutSessionLineItemParams{
				Quantity: stripe.Int64(it.Quantity),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("ils"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(it.Name),
					},
					UnitAmount: stripe.Int64(it.Amount),
				},
			})
			metaItems = append(metaItems, metaItem{E: it.EquipmentID, Q: it.Quantity, A: it.Amount})
		}
	} else {
		if req.ProductName == "" || req.Quantity <= 0 || req.Amount <= 0 {
			JSONError(w, "productName, quantity, and amount must be provided and positive", http.StatusBadRequest)
			return
		}
		lineItems = []*stripe.CheckoutSessionLineItemParams{
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
		}
	}

	params := &stripe.CheckoutSessionParams{
		SuccessURL: stripe.String(frontendURL + "/payment/success?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String(frontendURL + "/payment/cancel"),
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems:  lineItems,
	}

	// Attach order context so the webhook can persist the purchase once paid.
	if req.UserID != "" && req.GradeID != "" && len(metaItems) > 0 {
		if encoded, err := json.Marshal(metaItems); err == nil {
			params.AddMetadata("userId", req.UserID)
			params.AddMetadata("gradeId", req.GradeID)
			params.AddMetadata("items", string(encoded))
		} else {
			log.Printf("Failed to encode checkout item metadata: %v", err)
		}
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

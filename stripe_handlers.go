package main

// Stripe admin + webhook
//
// Three concerns live here:
//   1. POST /api/stripe/webhook        — Stripe -> us. On checkout.session.completed
//      we persist the purchase into orders/order_item (the link that makes the
//      dashboard reflect real payments). No session auth; verified by signature.
//   2. GET  /api/admin/payments        — admin reads live payments from Stripe.
//   3. GET  /api/admin/payments/balance
//      POST /api/admin/payments/{id}/refund — admin control over Stripe.
//
// Everything degrades gracefully when STRIPE_SECRET_KEY / STRIPE_WEBHOOK_SECRET
// are unset: reads return {configured:false}, the webhook no-ops, and refunds
// report that Stripe is not configured. So the app runs fine before keys exist
// and "lights up" once they're added.

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/balance"
	"github.com/stripe/stripe-go/v82/charge"
	"github.com/stripe/stripe-go/v82/refund"
	"github.com/stripe/stripe-go/v82/webhook"
)

// ---------------------------------------------------------------------------
// DTOs
// ---------------------------------------------------------------------------

type StripePayment struct {
	ID             string  `json:"id"`
	PaymentIntent  string  `json:"paymentIntent,omitempty"`
	Amount         float64 `json:"amount"`
	AmountRefunded float64 `json:"amountRefunded"`
	Currency       string  `json:"currency"`
	Status         string  `json:"status"`
	Refunded       bool    `json:"refunded"`
	Email          string  `json:"email,omitempty"`
	Description    string  `json:"description,omitempty"`
	ReceiptURL     string  `json:"receiptUrl,omitempty"`
	Created        string  `json:"created"`
	DashboardURL   string  `json:"dashboardUrl,omitempty"`
}

type PaymentsResponse struct {
	Configured bool            `json:"configured"`
	Payments   []StripePayment `json:"payments"`
}

type BalanceItem struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type BalanceResponse struct {
	Configured bool          `json:"configured"`
	Available  []BalanceItem `json:"available"`
	Pending    []BalanceItem `json:"pending"`
}

type RefundResult struct {
	ID     string  `json:"id"`
	Status string  `json:"status"`
	Amount float64 `json:"amount"`
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// useStripe sets the API key from the environment and reports whether Stripe is
// configured. Call at the start of every Stripe-backed handler.
func useStripe() bool {
	key := os.Getenv("STRIPE_SECRET_KEY")
	if key == "" {
		return false
	}
	stripe.Key = key
	return true
}

// minorToMajor converts an amount in agorot/cents to whole currency units.
func minorToMajor(minor int64) float64 {
	return float64(minor) / 100
}

func dashboardURL(livemode bool, ref string) string {
	if ref == "" {
		return ""
	}
	base := "https://dashboard.stripe.com/"
	if !livemode {
		base += "test/"
	}
	return base + "payments/" + ref
}

// ---------------------------------------------------------------------------
// Webhook: Stripe -> us
// ---------------------------------------------------------------------------

func stripeWebhookHandler(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		JSONError(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	secret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if secret == "" {
		// Without a signing secret we cannot trust the event, so we no-op. This
		// keeps the endpoint healthy before keys are configured.
		log.Println("Stripe webhook received but STRIPE_WEBHOOK_SECRET is not set; ignoring")
		w.WriteHeader(http.StatusOK)
		return
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), secret)
	if err != nil {
		log.Printf("Stripe webhook signature verification failed: %v", err)
		JSONError(w, "Invalid signature", http.StatusBadRequest)
		return
	}

	if event.Type == "checkout.session.completed" {
		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			log.Printf("Stripe webhook: failed to parse session: %v", err)
			JSONError(w, "Bad payload", http.StatusBadRequest)
			return
		}
		if err := persistOrderFromSession(session); err != nil {
			log.Printf("Stripe webhook: failed to persist order: %v", err)
			// 500 so Stripe retries delivery.
			JSONError(w, "Failed to record order", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

// persistOrderFromSession writes a completed checkout into orders/order_item.
// Idempotent on the Stripe session id, so webhook retries don't duplicate.
func persistOrderFromSession(session stripe.CheckoutSession) error {
	userID := session.Metadata["userId"]
	gradeID := session.Metadata["gradeId"]
	itemsRaw := session.Metadata["items"]
	if userID == "" || gradeID == "" || itemsRaw == "" {
		log.Printf("Stripe webhook: session %s has no order metadata; skipping persistence", session.ID)
		return nil
	}

	uid, err := strconv.Atoi(userID)
	if err != nil {
		log.Printf("Stripe webhook: invalid userId %q", userID)
		return nil
	}
	gid, err := strconv.Atoi(gradeID)
	if err != nil {
		log.Printf("Stripe webhook: invalid gradeId %q", gradeID)
		return nil
	}

	var items []metaItem
	if err := json.Unmarshal([]byte(itemsRaw), &items); err != nil {
		log.Printf("Stripe webhook: invalid items metadata: %v", err)
		return nil
	}
	if len(items) == 0 {
		return nil
	}

	// Idempotency guard against webhook retries.
	var exists bool
	if err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM orders WHERE stripe_session_id = $1)", session.ID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		log.Printf("Stripe webhook: order for session %s already recorded; skipping", session.ID)
		return nil
	}

	var paymentIntent string
	if session.PaymentIntent != nil {
		paymentIntent = session.PaymentIntent.ID
	}

	var total float64
	for _, it := range items {
		total += float64(it.Q) * minorToMajor(it.A)
	}

	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	var oid int
	err = tx.QueryRow(
		`INSERT INTO orders (uid, gid, total_amount, stripe_session_id, stripe_payment_intent)
		 VALUES ($1, $2, $3, $4, $5) RETURNING oid`,
		uid, gid, total, session.ID, paymentIntent,
	).Scan(&oid)
	if err != nil {
		tx.Rollback()
		return err
	}

	for _, it := range items {
		eid, convErr := strconv.Atoi(it.E)
		if convErr != nil {
			tx.Rollback()
			log.Printf("Stripe webhook: invalid equipment id %q", it.E)
			return convErr
		}
		if _, err := tx.Exec(
			`INSERT INTO order_item (oid, eid, quantity, price_at_purchase) VALUES ($1, $2, $3, $4)`,
			oid, eid, it.Q, minorToMajor(it.A),
		); err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("Stripe webhook: recorded order %d from session %s", oid, session.ID)
	return nil
}

// ---------------------------------------------------------------------------
// Admin reads
// ---------------------------------------------------------------------------

func listPaymentsHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	if !useStripe() {
		writeJSON(w, http.StatusOK, PaymentsResponse{Configured: false, Payments: []StripePayment{}})
		return
	}

	params := &stripe.ChargeListParams{}
	params.Limit = stripe.Int64(25)
	payments := []StripePayment{}

	iter := charge.List(params)
	for iter.Next() {
		c := iter.Charge()
		p := StripePayment{
			ID:             c.ID,
			Amount:         minorToMajor(c.Amount),
			AmountRefunded: minorToMajor(c.AmountRefunded),
			Currency:       string(c.Currency),
			Status:         string(c.Status),
			Refunded:       c.Refunded,
			Description:    c.Description,
			ReceiptURL:     c.ReceiptURL,
			Created:        time.Unix(c.Created, 0).UTC().Format(time.RFC3339),
		}
		if c.PaymentIntent != nil {
			p.PaymentIntent = c.PaymentIntent.ID
			p.DashboardURL = dashboardURL(c.Livemode, c.PaymentIntent.ID)
		} else {
			p.DashboardURL = dashboardURL(c.Livemode, c.ID)
		}
		if c.BillingDetails != nil {
			p.Email = c.BillingDetails.Email
		}
		payments = append(payments, p)
	}
	if err := iter.Err(); err != nil {
		log.Printf("listPayments: %v", err)
		JSONError(w, "Failed to load payments from Stripe", http.StatusBadGateway)
		return
	}

	writeJSON(w, http.StatusOK, PaymentsResponse{Configured: true, Payments: payments})
}

func getBalanceHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	if !useStripe() {
		writeJSON(w, http.StatusOK, BalanceResponse{Configured: false, Available: []BalanceItem{}, Pending: []BalanceItem{}})
		return
	}

	b, err := balance.Get(nil)
	if err != nil {
		log.Printf("getBalance: %v", err)
		JSONError(w, "Failed to load balance from Stripe", http.StatusBadGateway)
		return
	}

	resp := BalanceResponse{Configured: true, Available: []BalanceItem{}, Pending: []BalanceItem{}}
	for _, a := range b.Available {
		resp.Available = append(resp.Available, BalanceItem{Amount: minorToMajor(a.Amount), Currency: string(a.Currency)})
	}
	for _, p := range b.Pending {
		resp.Pending = append(resp.Pending, BalanceItem{Amount: minorToMajor(p.Amount), Currency: string(p.Currency)})
	}
	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// Admin control: refund
// ---------------------------------------------------------------------------

func refundPaymentHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	chargeID := r.PathValue("id")
	if chargeID == "" {
		JSONError(w, "Missing payment id", http.StatusBadRequest)
		return
	}
	if !useStripe() {
		JSONError(w, "Stripe is not configured", http.StatusServiceUnavailable)
		return
	}

	ref, err := refund.New(&stripe.RefundParams{Charge: stripe.String(chargeID)})
	if err != nil {
		log.Printf("refundPayment(%s): %v", chargeID, err)
		JSONError(w, "Refund failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	writeJSON(w, http.StatusOK, RefundResult{
		ID:     ref.ID,
		Status: string(ref.Status),
		Amount: minorToMajor(ref.Amount),
	})
}

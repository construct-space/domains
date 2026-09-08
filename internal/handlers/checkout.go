package handlers

import (
	"fmt"
	"log"
	"net/http"

	"construct/domains/internal/polar"
)

// CreateCheckout creates a Polar checkout session for cart items.
// Each cart item (domain) needs a Polar product — we create on-the-fly if needed.
func CreateCheckout(w http.ResponseWriter, r *http.Request) {
	body, err := parseBody(r)
	if err != nil {
		WriteJSON(w, 400, map[string]any{"error": "invalid request body"})
		return
	}

	itemsRaw, ok := body["items"]
	if !ok {
		WriteJSON(w, 400, map[string]any{"error": "items array is required"})
		return
	}

	items, ok := itemsRaw.([]any)
	if !ok || len(items) == 0 {
		WriteJSON(w, 400, map[string]any{"error": "items must be a non-empty array"})
		return
	}

	email := getString(body, "email")
	userID := getUserID(r)

	var productIDs []string
	var totalDomains []string

	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		domain := getString(item, "domain")
		priceStr := getString(item, "price")
		if domain == "" || priceStr == "" {
			continue
		}

		// Convert price string (e.g. "9.73") to cents
		var dollars, cents int
		fmt.Sscanf(priceStr, "%d.%d", &dollars, &cents)
		priceInCents := dollars*100 + cents

		// Create a one-time Polar product for this domain
		product, err := Polar.CreateProduct(
			fmt.Sprintf("Domain: %s", domain),
			fmt.Sprintf("1 year domain registration for %s", domain),
			priceInCents,
		)
		if err != nil {
			log.Printf("Polar create product error for %s: %v", domain, err)
			WriteJSON(w, 502, map[string]any{"error": "failed to create product for " + domain})
			return
		}

		productIDs = append(productIDs, product.ID)
		totalDomains = append(totalDomains, domain)
	}

	if len(productIDs) == 0 {
		WriteJSON(w, 400, map[string]any{"error": "no valid items in cart"})
		return
	}

	// Create checkout session
	checkoutReq := &polar.CheckoutRequest{
		Products:   productIDs,
		SuccessURL: Cfg.AppURL + "/dashboard?checkout=success",
		Metadata: map[string]string{
			"source":  "construct-domains",
			"user_id": fmt.Sprintf("%d", userID),
		},
	}
	if email != "" {
		checkoutReq.CustomerEmail = email
	}

	checkout, err := Polar.CreateCheckout(checkoutReq)
	if err != nil {
		log.Printf("Polar checkout error: %v", err)
		WriteJSON(w, 502, map[string]any{"error": "failed to create checkout session"})
		return
	}

	WriteJSON(w, 201, map[string]any{
		"status":       "success",
		"checkout_id":  checkout.ID,
		"checkout_url": checkout.URL,
		"total_amount": checkout.TotalAmount,
		"currency":     checkout.Currency,
		"domains":      totalDomains,
	})
}

// GetCheckoutStatus returns the status of a checkout session
func GetCheckoutStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteJSON(w, 400, map[string]any{"error": "checkout id is required"})
		return
	}

	checkout, err := Polar.GetCheckout(id)
	if err != nil {
		log.Printf("Polar get checkout error: %v", err)
		WriteJSON(w, 502, map[string]any{"error": "failed to get checkout status"})
		return
	}

	WriteJSON(w, 200, map[string]any{
		"status":       checkout.Status,
		"checkout_id":  checkout.ID,
		"total_amount": checkout.TotalAmount,
		"currency":     checkout.Currency,
	})
}

// ListOrders returns Polar orders (authenticated). Soft-fails to an
// empty list when Polar is unreachable / misconfigured — orders are a
// best-effort enrichment, not the canonical source of truth, and 502'ing
// here makes the whole "Orders" tab unrenderable.
func ListOrders(w http.ResponseWriter, r *http.Request) {
	userID := fmt.Sprintf("%d", getUserID(r))
	orders, err := Polar.ListOrders("")
	if err != nil {
		log.Printf("Polar list orders error: %v — returning empty list", err)
		WriteJSON(w, 200, map[string]any{
			"status":                 "success",
			"orders":                 []any{},
			"total":                  0,
			"registrar_unavailable":  true,
		})
		return
	}
	filtered := orders.Items[:0]
	for _, order := range orders.Items {
		if order.Metadata != nil && fmt.Sprintf("%v", order.Metadata["user_id"]) == userID {
			filtered = append(filtered, order)
		}
	}

	WriteJSON(w, 200, map[string]any{
		"status": "success",
		"orders": filtered,
		"total":  len(filtered),
	})
}

// GetOrder returns a single order
func GetOrder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteJSON(w, 400, map[string]any{"error": "order id is required"})
		return
	}

	order, err := Polar.GetOrder(id)
	if err != nil {
		log.Printf("Polar get order error: %v", err)
		WriteJSON(w, 502, map[string]any{"error": "failed to get order"})
		return
	}
	if order.Metadata == nil || fmt.Sprintf("%v", order.Metadata["user_id"]) != fmt.Sprintf("%d", getUserID(r)) {
		WriteJSON(w, 404, map[string]any{"error": "order not found"})
		return
	}

	WriteJSON(w, 200, map[string]any{
		"status": "success",
		"order":  order,
	})
}

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- 1. Price Conversion Tests ---

func TestWcPriceToCents(t *testing.T) {
	tests := []struct {
		input string
		want  int
		err   bool
	}{
		{"19.99", 1999, false},
		{"0.99", 99, false},
		{"100", 10000, false},
		{"19.9", 1990, false},
		{"0.01", 1, false},
		{"0", 0, false},
		{"", 0, true},
		{"abc", 0, true},
		{"19.999", 1999, false}, // truncated to 2 decimals
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := wcPriceToCents(tt.input)
			if tt.err && err == nil {
				t.Fatalf("expected error for %q", tt.input)
			}
			if !tt.err && err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("wcPriceToCents(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestCentsToWcPrice(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{1999, "19.99"},
		{99, "0.99"},
		{10000, "100.00"},
		{0, "0.00"},
		{1, "0.01"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.input), func(t *testing.T) {
			got := centsToWcPrice(tt.input)
			if got != tt.want {
				t.Errorf("centsToWcPrice(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- 2. UCP Type Serialization Tests ---

func TestUCPCheckoutSerialization(t *testing.T) {
	checkout := UCPCheckout{
		ID:     "123",
		Status: "incomplete",
		LineItems: []UCPLineItem{
			{
				ID:       "1",
				Item:     UCPItem{ID: "42", Title: "Test Product", Price: 1999},
				Quantity: 2,
				Totals:   []UCPTotal{{Type: "total", DisplayText: "39.98", Amount: 3998}},
			},
		},
		Currency: "USD",
		Totals:   []UCPTotal{{Type: "total", DisplayText: "39.98", Amount: 3998}},
		Links:    []UCPLink{},
	}
	data, err := json.Marshal(checkout)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded UCPCheckout
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.ID != "123" {
		t.Errorf("ID = %q, want %q", decoded.ID, "123")
	}
	if decoded.Status != "incomplete" {
		t.Errorf("Status = %q, want %q", decoded.Status, "incomplete")
	}
	if len(decoded.LineItems) != 1 {
		t.Fatalf("LineItems count = %d, want 1", len(decoded.LineItems))
	}
	if decoded.LineItems[0].Item.Price != 1999 {
		t.Errorf("LineItem price = %d, want 1999", decoded.LineItems[0].Item.Price)
	}
}

// --- 3. UCP Profile Test ---

func TestNewDefaultUCPProfile(t *testing.T) {
	profile := NewDefaultUCPProfile("https://store.example.com")
	if profile.UCP.Version != "2026-01-11" {
		t.Errorf("Version = %q, want %q", profile.UCP.Version, "2026-01-11")
	}
	if len(profile.UCP.Capabilities) != 4 {
		t.Errorf("Capabilities count = %d, want 4", len(profile.UCP.Capabilities))
	}
	svc, ok := profile.UCP.Services["dev.ucp.shopping"]
	if !ok {
		t.Fatal("missing dev.ucp.shopping service")
	}
	if svc.MCP.Endpoint != "https://store.example.com/ucp/mcp" {
		t.Errorf("MCP endpoint = %q", svc.MCP.Endpoint)
	}
	if len(profile.Payment.Handlers) != 1 {
		t.Errorf("Payment handlers count = %d, want 1", len(profile.Payment.Handlers))
	}
}

// --- 4. Checkout Mapping Tests ---

func TestMapOrderToUCPCheckout(t *testing.T) {
	order := &Order{
		ID:       200,
		Status:   "pending",
		Total:    "39.98",
		Currency: "USD",
		OrderKey: "wc_order_abc",
		LineItems: []OrderLineItem{
			{ID: 1, Name: "Shoe", ProductID: 42, Quantity: 2, Subtotal: "39.98", Total: "39.98", Price: 19.99},
		},
		DiscountTotal: "0.00",
		ShippingTotal: "0.00",
		TotalTax:      "0.00",
	}

	checkout := mapOrderToUCPCheckout(order, "https://store.example.com")
	if checkout.ID != "200" {
		t.Errorf("ID = %q, want %q", checkout.ID, "200")
	}
	if checkout.Status != "incomplete" {
		t.Errorf("Status = %q, want %q", checkout.Status, "incomplete")
	}
	if checkout.ContinueURL != "" {
		t.Errorf("ContinueURL should be empty for incomplete, got %q", checkout.ContinueURL)
	}
}

func TestMapOrderToUCPCheckoutWithBilling(t *testing.T) {
	order := &Order{
		ID:       201,
		Status:   "pending",
		Total:    "19.99",
		Currency: "USD",
		OrderKey: "wc_order_xyz",
		Billing: &OrderAddress{
			Email:     "test@example.com",
			FirstName: "John",
			LastName:  "Doe",
		},
		LineItems: []OrderLineItem{
			{ID: 1, Name: "Shoe", ProductID: 42, Quantity: 1, Subtotal: "19.99", Total: "19.99", Price: 19.99},
		},
		DiscountTotal: "0.00",
		ShippingTotal: "5.00",
		TotalTax:      "1.60",
	}

	checkout := mapOrderToUCPCheckout(order, "https://store.example.com")
	if checkout.Status != "requires_escalation" {
		t.Errorf("Status = %q, want %q", checkout.Status, "requires_escalation")
	}
	if checkout.Buyer == nil {
		t.Fatal("Buyer should not be nil")
	}
	if checkout.Buyer.Email != "test@example.com" {
		t.Errorf("Buyer.Email = %q", checkout.Buyer.Email)
	}
	expected := "https://store.example.com/checkout/order-pay/201/?pay_for_order=true&key=wc_order_xyz"
	if checkout.ContinueURL != expected {
		t.Errorf("ContinueURL = %q, want %q", checkout.ContinueURL, expected)
	}
}

func TestCheckoutStatusMapping(t *testing.T) {
	tests := []struct {
		wcStatus   string
		hasBilling bool
		ucpStatus  string
	}{
		{"pending", false, "incomplete"},
		{"pending", true, "requires_escalation"},
		{"on-hold", false, "complete_in_progress"},
		{"processing", false, "completed"},
		{"completed", false, "completed"},
		{"cancelled", false, "canceled"},
		{"failed", false, "incomplete"},
	}
	for _, tt := range tests {
		t.Run(tt.wcStatus, func(t *testing.T) {
			order := &Order{
				ID:            1,
				Status:        tt.wcStatus,
				Total:         "10.00",
				DiscountTotal: "0.00",
				ShippingTotal: "0.00",
				TotalTax:      "0.00",
			}
			if tt.hasBilling {
				order.Billing = &OrderAddress{Email: "test@test.com"}
			}
			checkout := mapOrderToUCPCheckout(order, "https://store.example.com")
			if checkout.Status != tt.ucpStatus {
				t.Errorf("Status = %q, want %q", checkout.Status, tt.ucpStatus)
			}
		})
	}
}

// --- 5. Order Mapping Tests ---

func TestMapOrderToUCPOrder(t *testing.T) {
	order := &Order{
		ID:       300,
		Status:   "processing",
		Total:    "49.99",
		Currency: "USD",
		LineItems: []OrderLineItem{
			{ID: 1, Name: "Widget", ProductID: 10, Quantity: 1, Subtotal: "49.99", Total: "49.99", Price: 49.99},
		},
		ShippingLines: []ShippingLine{
			{ID: 1, MethodID: "flat_rate", MethodTitle: "Flat Rate", Total: "5.00"},
		},
		DiscountTotal: "0.00",
		ShippingTotal: "5.00",
		TotalTax:      "4.00",
	}
	refunds := []OrderRefund{
		{ID: 50, Reason: "Damaged item", Total: "-10.00", CreatedAt: "2025-01-20T12:00:00"},
	}
	notes := []OrderNote{
		{ID: 10, Note: "Shipped via UPS"},
	}

	ucpOrder := mapOrderToUCPOrder(order, refunds, notes, "https://store.example.com")
	if ucpOrder.ID != "300" {
		t.Errorf("ID = %q", ucpOrder.ID)
	}
	if ucpOrder.CheckoutID != "300" {
		t.Errorf("CheckoutID = %q", ucpOrder.CheckoutID)
	}
	if len(ucpOrder.LineItems) != 1 {
		t.Fatalf("LineItems count = %d", len(ucpOrder.LineItems))
	}
	if ucpOrder.LineItems[0].Status != "processing" {
		t.Errorf("LineItem status = %q", ucpOrder.LineItems[0].Status)
	}
	if ucpOrder.Fulfillment == nil {
		t.Fatal("Fulfillment should not be nil")
	}
	if len(ucpOrder.Fulfillment.Expectations) != 1 {
		t.Errorf("Expectations count = %d", len(ucpOrder.Fulfillment.Expectations))
	}
	if len(ucpOrder.Fulfillment.Events) != 1 {
		t.Errorf("Events count = %d", len(ucpOrder.Fulfillment.Events))
	}
	if len(ucpOrder.Adjustments) != 1 {
		t.Fatalf("Adjustments count = %d", len(ucpOrder.Adjustments))
	}
	if ucpOrder.Adjustments[0].Amount != 1000 {
		t.Errorf("Adjustment amount = %d, want 1000", ucpOrder.Adjustments[0].Amount)
	}
}

func TestOrderLineItemStatusMapping(t *testing.T) {
	tests := []struct {
		wcStatus string
		want     string
	}{
		{"processing", "processing"},
		{"completed", "fulfilled"},
		{"on-hold", "processing"},
		{"pending", "processing"},
	}
	for _, tt := range tests {
		t.Run(tt.wcStatus, func(t *testing.T) {
			got := orderStatusToLineItemStatus(tt.wcStatus)
			if got != tt.want {
				t.Errorf("orderStatusToLineItemStatus(%q) = %q, want %q", tt.wcStatus, got, tt.want)
			}
		})
	}
}

// --- 6. A2UI Card Tests ---

func TestProductCard(t *testing.T) {
	p := Product{
		ID:          1,
		Name:        "Test Shoe",
		Price:       "29.99",
		Description: "A nice shoe",
		StockStatus: "instock",
		Images:      []ProductImage{{ID: 1, Src: "https://example.com/shoe.jpg"}},
	}
	lines := ProductCard(p)
	if len(lines) != 3 {
		t.Fatalf("expected 3 JSONL lines, got %d", len(lines))
	}
	var su map[string]any
	if err := json.Unmarshal(lines[0], &su); err != nil {
		t.Fatalf("unmarshal surfaceUpdate: %v", err)
	}
	if _, ok := su["surfaceUpdate"]; !ok {
		t.Error("first line should be surfaceUpdate")
	}
}

func TestCheckoutCard(t *testing.T) {
	checkout := UCPCheckout{
		ID:     "100",
		Status: "incomplete",
		LineItems: []UCPLineItem{
			{ID: "1", Item: UCPItem{Title: "Widget"}, Quantity: 2},
		},
		Totals: []UCPTotal{{Type: "total", DisplayText: "19.99", Amount: 1999}},
	}
	lines := CheckoutCard(checkout)
	if len(lines) != 3 {
		t.Fatalf("expected 3 JSONL lines, got %d", len(lines))
	}
}

func TestOrderCard(t *testing.T) {
	order := UCPOrder{
		ID:     "200",
		Status: "processing",
		LineItems: []UCPOrderLineItem{
			{UCPLineItem: UCPLineItem{ID: "1", Item: UCPItem{Title: "Thing"}, Quantity: 1}, Status: "processing"},
		},
		Totals: []UCPTotal{{Type: "total", DisplayText: "29.99", Amount: 2999}},
	}
	lines := OrderCard(order)
	if len(lines) != 3 {
		t.Fatalf("expected 3 JSONL lines, got %d", len(lines))
	}
}

// --- 7. REST Endpoint Tests ---

func TestRESTGetUCPProfile(t *testing.T) {
	cfg := &Config{StoreURL: "https://store.example.com", UCPEnabled: true}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/ucp", handleUCPProfile(cfg))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/.well-known/ucp")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var profile UCPProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if profile.UCP.Version != "2026-01-11" {
		t.Errorf("version = %q", profile.UCP.Version)
	}
}

func TestRESTSearchProducts(t *testing.T) {
	wcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]Product{
			{ID: 1, Name: "Shoe", Price: "29.99", StockStatus: "instock"},
		})
	}))
	defer wcServer.Close()

	cfg := &Config{StoreURL: "https://store.example.com", UCPEnabled: true}
	wc := NewWooClient(wcServer.URL, "ck", "cs")
	rs := NewRESTServer(wc, cfg, nil)

	mux := http.NewServeMux()
	rs.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ucp/v1/products?query=shoe")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var results []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}
	if results[0]["title"] != "Shoe" {
		t.Errorf("title = %v", results[0]["title"])
	}
}

// --- 8. Discovery MCP Tool Tests ---

func TestSearchShopCatalogTool(t *testing.T) {
	cs := setupToolTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]Product{
			{ID: 1, Name: "Widget", Price: "14.99", StockStatus: "instock", Permalink: "https://shop.example.com/widget"},
		})
	}), "https://shop.example.com")

	text := callTool(t, cs, "search_shop_catalog", map[string]any{
		"query":   "widget",
		"context": "looking for widgets",
	})

	var results []map[string]any
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results count = %d", len(results))
	}
	if results[0]["title"] != "Widget" {
		t.Errorf("title = %v", results[0]["title"])
	}
	if results[0]["price"].(float64) != 1499 {
		t.Errorf("price = %v, want 1499", results[0]["price"])
	}
}

func TestGetProductCategoriesTool(t *testing.T) {
	cs := setupToolTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]ProductCategory{
			{ID: 1, Name: "Clothing", Slug: "clothing"},
			{ID: 2, Name: "Electronics", Slug: "electronics"},
		})
	}), "https://shop.example.com")

	text := callTool(t, cs, "get_product_categories", nil)

	var result map[string]any
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	cats, ok := result["categories"].([]any)
	if !ok {
		t.Fatal("missing categories array")
	}
	if len(cats) != 2 {
		t.Errorf("categories count = %d", len(cats))
	}
}

// --- API Key Middleware Tests ---

func TestAPIKeyMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	wrapped := apiKeyMiddleware([]string{"key-abc", "key-xyz"})(handler)
	ts := httptest.NewServer(wrapped)
	defer ts.Close()

	t.Run("valid Bearer token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", ts.URL+"/test", nil)
		req.Header.Set("Authorization", "Bearer key-abc")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	})

	t.Run("valid X-API-Key", func(t *testing.T) {
		req, _ := http.NewRequest("GET", ts.URL+"/test", nil)
		req.Header.Set("X-API-Key", "key-xyz")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/test")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("wrong key", func(t *testing.T) {
		req, _ := http.NewRequest("GET", ts.URL+"/test", nil)
		req.Header.Set("Authorization", "Bearer wrong-key")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
	})
}

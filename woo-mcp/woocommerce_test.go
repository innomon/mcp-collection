package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchProducts(t *testing.T) {
	products := []Product{
		{ID: 1, Name: "Blue T-Shirt", Price: "19.99", Description: "A nice shirt", Permalink: "https://example.com/blue-tshirt"},
		{ID: 2, Name: "Red T-Shirt", Price: "24.99", Description: "A red shirt", Permalink: "https://example.com/red-tshirt"},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wp-json/wc/v3/products" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if q := r.URL.Query().Get("search"); q != "shirt" {
			t.Errorf("unexpected search query: %s", q)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(products)
	}))
	defer server.Close()

	client := NewWooClient(server.URL, "ck_test", "cs_test")
	result, err := client.SearchProducts(context.Background(), "shirt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 products, got %d", len(result))
	}
	if result[0].Name != "Blue T-Shirt" {
		t.Errorf("expected 'Blue T-Shirt', got %q", result[0].Name)
	}
	if result[1].Price != "24.99" {
		t.Errorf("expected price '24.99', got %q", result[1].Price)
	}
}

func TestSearchProductsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewWooClient(server.URL, "ck_test", "cs_test")
	result, err := client.SearchProducts(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 products, got %d", len(result))
	}
}

func TestSearchProductsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	client := NewWooClient(server.URL, "ck_test", "cs_test")
	_, err := client.SearchProducts(context.Background(), "shirt")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain '500', got: %v", err)
	}
}

func TestGetOrders(t *testing.T) {
	orders := []Order{
		{ID: 101, Status: "processing", Total: "49.99", Currency: "USD", OrderKey: "wc_order_abc", CreatedAt: "2025-01-15T10:00:00"},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wp-json/wc/v3/orders" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if pp := r.URL.Query().Get("per_page"); pp != "5" {
			t.Errorf("expected per_page=5, got %s", pp)
		}
		if ob := r.URL.Query().Get("orderby"); ob != "date" {
			t.Errorf("expected orderby=date, got %s", ob)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(orders)
	}))
	defer server.Close()

	client := NewWooClient(server.URL, "ck_test", "cs_test")
	result, err := client.GetOrders(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 order, got %d", len(result))
	}
	if result[0].ID != 101 {
		t.Errorf("expected order ID 101, got %d", result[0].ID)
	}
	if result[0].Status != "processing" {
		t.Errorf("expected status 'processing', got %q", result[0].Status)
	}
}

func TestCreateOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/wp-json/wc/v3/orders" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to parse request body: %v", err)
		}
		if payload["status"] != "pending" {
			t.Errorf("expected status 'pending', got %v", payload["status"])
		}
		items, ok := payload["line_items"].([]interface{})
		if !ok || len(items) != 1 {
			t.Fatalf("expected 1 line item, got %v", payload["line_items"])
		}
		item := items[0].(map[string]interface{})
		if int(item["product_id"].(float64)) != 42 {
			t.Errorf("expected product_id 42, got %v", item["product_id"])
		}

		order := Order{ID: 200, Status: "pending", Total: "19.99", Currency: "USD", OrderKey: "wc_order_xyz", CreatedAt: "2025-01-20T12:00:00"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(order)
	}))
	defer server.Close()

	client := NewWooClient(server.URL, "ck_test", "cs_test")
	result, err := client.CreateOrder(context.Background(), []LineItem{{ProductID: 42, Quantity: 2}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != 200 {
		t.Errorf("expected order ID 200, got %d", result.ID)
	}
	if result.Status != "pending" {
		t.Errorf("expected status 'pending', got %q", result.Status)
	}
}

func TestCreateNote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/wp-json/wc/v3/orders/101/notes" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to parse request body: %v", err)
		}
		if payload["note"] != "Order shipped" {
			t.Errorf("expected note 'Order shipped', got %v", payload["note"])
		}

		note := OrderNote{ID: 5, Note: "Order shipped"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(note)
	}))
	defer server.Close()

	client := NewWooClient(server.URL, "ck_test", "cs_test")
	result, err := client.CreateNote(context.Background(), 101, "Order shipped")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != 5 {
		t.Errorf("expected note ID 5, got %d", result.ID)
	}
	if result.Note != "Order shipped" {
		t.Errorf("expected note 'Order shipped', got %q", result.Note)
	}
}

func TestBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			t.Fatal("expected Basic Auth header")
		}
		if user != "ck_test" {
			t.Errorf("expected username 'ck_test', got %q", user)
		}
		if pass != "cs_test" {
			t.Errorf("expected password 'cs_test', got %q", pass)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewWooClient(server.URL, "ck_test", "cs_test")
	_, err := client.SearchProducts(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

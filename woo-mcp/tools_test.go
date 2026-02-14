package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func setupToolTest(t *testing.T, handler http.Handler, storeURL string) *mcp.ClientSession {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	wc := NewWooClient(ts.URL, "ck", "cs")
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.1"}, nil)
	RegisterTools(s, wc, storeURL)

	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := s.Connect(context.Background(), t1, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "testclient", Version: "0.1"}, nil)
	cs, err := client.Connect(context.Background(), t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func callTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatal(err)
	}
	return res.Content[0].(*mcp.TextContent).Text
}

func TestSearchProductsTool(t *testing.T) {
	cs := setupToolTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]Product{
			{ID: 1, Name: "Shoe", Price: "29.99", Permalink: "https://shop.example.com/shoe"},
			{ID: 2, Name: "Boot", Price: "59.99", Permalink: "https://shop.example.com/boot"},
		})
	}), "https://shop.example.com")

	text := callTool(t, cs, "search_products", map[string]any{"query": "shoe"})
	if !strings.Contains(text, "Shoe") {
		t.Errorf("expected Shoe in result, got %s", text)
	}
	if !strings.Contains(text, "$29.99") {
		t.Errorf("expected $29.99 in result, got %s", text)
	}
	if !strings.Contains(text, "Boot") {
		t.Errorf("expected Boot in result, got %s", text)
	}
}

func TestGetOrderHistoryTool(t *testing.T) {
	cs := setupToolTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]Order{
			{ID: 100, Status: "pending", Total: "10.00"},
			{ID: 101, Status: "on-hold", Total: "20.00"},
			{ID: 102, Status: "processing", Total: "30.00"},
			{ID: 103, Status: "completed", Total: "40.00"},
			{ID: 104, Status: "refunded", Total: "50.00"},
		})
	}), "https://shop.example.com")

	text := callTool(t, cs, "get_order_history", nil)
	if !strings.Contains(text, "Order #100: Open - $10.00") {
		t.Errorf("expected pending mapped to Open, got %s", text)
	}
	if !strings.Contains(text, "Order #101: Open - $20.00") {
		t.Errorf("expected on-hold mapped to Open, got %s", text)
	}
	if !strings.Contains(text, "Order #102: In Process - $30.00") {
		t.Errorf("expected processing mapped to In Process, got %s", text)
	}
	if !strings.Contains(text, "Order #103: Delivered - $40.00") {
		t.Errorf("expected completed mapped to Delivered, got %s", text)
	}
	if !strings.Contains(text, "Order #104: refunded - $50.00") {
		t.Errorf("expected refunded as-is, got %s", text)
	}
}

func TestCheckoutTool(t *testing.T) {
	cs := setupToolTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Order{ID: 200, Status: "pending", Total: "29.99", OrderKey: "wc_order_abc123"})
	}), "https://shop.example.com")

	text := callTool(t, cs, "checkout", map[string]any{
		"items": []map[string]any{
			{"product_id": 1, "quantity": 2},
		},
	})
	expected := "https://shop.example.com/checkout/order-pay/200/?pay_for_order=true&key=wc_order_abc123"
	if text != expected {
		t.Errorf("expected %s, got %s", expected, text)
	}
}

func TestRaiseIssueTool(t *testing.T) {
	cs := setupToolTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(OrderNote{ID: 5, Note: "Item arrived damaged"})
	}), "https://shop.example.com")

	text := callTool(t, cs, "raise_issue", map[string]any{
		"order_id": 101,
		"text":     "Item arrived damaged",
	})
	if !strings.Contains(text, "Issue raised on order 101") {
		t.Errorf("expected confirmation, got %s", text)
	}
	if !strings.Contains(text, "note ID: 5") {
		t.Errorf("expected note ID in result, got %s", text)
	}
}

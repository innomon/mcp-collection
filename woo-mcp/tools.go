package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterTools(s *mcp.Server, client *WooClient, cfg *Config) {
	storeURL := cfg.StoreURL

	s.AddTool(&mcp.Tool{
		Name:        "search_products",
		Description: "Search for products in the store",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Search query for products"}},"required":["query"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, err
		}
		products, err := client.SearchProducts(ctx, args.Query)
		if err != nil {
			return nil, err
		}
		var lines []string
		for _, p := range products {
			lines = append(lines, fmt.Sprintf("%s - $%s - %s", p.Name, p.Price, p.Permalink))
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: strings.Join(lines, "\n")}},
		}, nil
	})

	s.AddTool(&mcp.Tool{
		Name:        "get_order_history",
		Description: "Get recent order history",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		orders, err := client.GetOrders(ctx, 10)
		if err != nil {
			return nil, err
		}
		var lines []string
		for _, o := range orders {
			status := mapOrderStatus(o.Status)
			lines = append(lines, fmt.Sprintf("Order #%d: %s - $%s", o.ID, status, o.Total))
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: strings.Join(lines, "\n")}},
		}, nil
	})

	s.AddTool(&mcp.Tool{
		Name:        "checkout",
		Description: "Create an order and get a payment link",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"product_id":{"type":"number"},"quantity":{"type":"number"}},"required":["product_id","quantity"]}}},"required":["items"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Items []LineItem `json:"items"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, err
		}
		order, err := client.CreateOrder(ctx, args.Items)
		if err != nil {
			return nil, err
		}
		paymentURL := fmt.Sprintf("%s/checkout/order-pay/%d/?pay_for_order=true&key=%s", storeURL, order.ID, order.OrderKey)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: paymentURL}},
		}, nil
	})

	s.AddTool(&mcp.Tool{
		Name:        "raise_issue",
		Description: "Raise an issue on an order",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"order_id":{"type":"number","description":"Order ID"},"text":{"type":"string","description":"Issue description"}},"required":["order_id","text"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			OrderID int    `json:"order_id"`
			Text    string `json:"text"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, err
		}
		note, err := client.CreateNote(ctx, args.OrderID, args.Text)
		if err != nil {
			return nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Issue raised on order %d: %s (note ID: %d)", args.OrderID, note.Note, note.ID)}},
		}, nil
	})

	if cfg.UCPEnabled {
		registerUCPDiscoveryTools(s, client, cfg.A2UIEnabled)
		registerUCPCheckoutTools(s, client, storeURL, cfg.A2UIEnabled)
		registerUCPOrderTools(s, client, storeURL, cfg.A2UIEnabled)
	}
}

func registerUCPDiscoveryTools(s *mcp.Server, client *WooClient, a2uiEnabled bool) {
	s.AddTool(&mcp.Tool{
		Name:        "search_shop_catalog",
		Description: "Search for products in the store with context-aware ranking",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Search terms"},"context":{"type":"string","description":"Buyer context for ranking"},"category":{"type":"string","description":"Filter by category slug"},"min_price":{"type":"number","description":"Minimum price filter"},"max_price":{"type":"number","description":"Maximum price filter"},"per_page":{"type":"integer","description":"Results per page (default 10)"}},"required":["query","context"]}`),
	}, handleSearchShopCatalog(client, a2uiEnabled))

	s.AddTool(&mcp.Tool{
		Name:        "get_product",
		Description: "Get detailed product information including variants",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"Product ID"}},"required":["id"]}`),
	}, handleGetProduct(client, a2uiEnabled))

	s.AddTool(&mcp.Tool{
		Name:        "get_product_categories",
		Description: "List all product categories",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}, handleGetProductCategories(client))

	s.AddTool(&mcp.Tool{
		Name:        "search_shop_policies_and_faqs",
		Description: "Search store policies and FAQ pages",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Search query for policies"}},"required":["query"]}`),
	}, handleSearchShopPolicies(client))
}

func registerUCPCheckoutTools(s *mcp.Server, client *WooClient, storeURL string, a2uiEnabled bool) {
	s.AddTool(&mcp.Tool{
		Name:        "create_checkout",
		Description: "Create a new checkout session",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"line_items":{"type":"array","items":{"type":"object","properties":{"item_id":{"type":"string","description":"Product ID"},"quantity":{"type":"integer","description":"Quantity"}},"required":["item_id","quantity"]}},"buyer":{"type":"object","properties":{"email":{"type":"string"},"first_name":{"type":"string"},"last_name":{"type":"string"}}},"currency":{"type":"string","description":"Currency code (default USD)"}},"required":["line_items"]}`),
	}, handleCreateCheckout(client, storeURL, a2uiEnabled))

	s.AddTool(&mcp.Tool{
		Name:        "get_checkout",
		Description: "Get checkout session details",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"Checkout/order ID"}},"required":["id"]}`),
	}, handleGetCheckout(client, storeURL, a2uiEnabled))

	s.AddTool(&mcp.Tool{
		Name:        "update_checkout",
		Description: "Update checkout session (line items, buyer info)",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"Checkout/order ID"},"line_items":{"type":"array","items":{"type":"object","properties":{"item_id":{"type":"string"},"quantity":{"type":"integer"}},"required":["item_id","quantity"]}},"buyer":{"type":"object","properties":{"email":{"type":"string"},"first_name":{"type":"string"},"last_name":{"type":"string"}}}},"required":["id"]}`),
	}, handleUpdateCheckout(client, storeURL, a2uiEnabled))

	s.AddTool(&mcp.Tool{
		Name:        "complete_checkout",
		Description: "Complete checkout — returns payment redirect URL",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"Checkout/order ID"}},"required":["id"]}`),
	}, handleCompleteCheckout(client, storeURL, a2uiEnabled))

	s.AddTool(&mcp.Tool{
		Name:        "cancel_checkout",
		Description: "Cancel a checkout session",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"Checkout/order ID"}},"required":["id"]}`),
	}, handleCancelCheckout(client, storeURL, a2uiEnabled))
}

func registerUCPOrderTools(s *mcp.Server, client *WooClient, storeURL string, a2uiEnabled bool) {
	s.AddTool(&mcp.Tool{
		Name:        "get_order",
		Description: "Get order details with fulfillment tracking and adjustments",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"Order ID"}},"required":["id"]}`),
	}, handleGetOrder(client, storeURL, a2uiEnabled))

	s.AddTool(&mcp.Tool{
		Name:        "list_orders",
		Description: "List recent orders",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"per_page":{"type":"integer","description":"Number of orders to return (default 10)"}}}`),
	}, handleListOrders(client, storeURL, a2uiEnabled))
}

func mapOrderStatus(status string) string {
	switch status {
	case "pending", "on-hold":
		return "Open"
	case "processing":
		return "In Process"
	case "completed":
		return "Delivered"
	default:
		return status
	}
}

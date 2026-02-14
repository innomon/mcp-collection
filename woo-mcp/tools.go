package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterTools(s *mcp.Server, client *WooClient, storeURL string) {
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

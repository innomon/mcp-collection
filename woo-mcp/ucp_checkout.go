package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Input types for UCP checkout tool handlers ---

type CreateCheckoutInput struct {
	LineItems []CreateCheckoutLineItem `json:"line_items"`
	Buyer     *UCPBuyer               `json:"buyer,omitempty"`
	Currency  string                  `json:"currency,omitempty"`
}

type CreateCheckoutLineItem struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
}

type UpdateCheckoutInput struct {
	ID        string                   `json:"id"`
	LineItems []CreateCheckoutLineItem `json:"line_items,omitempty"`
	Buyer     *UCPBuyer               `json:"buyer,omitempty"`
}

// --- Mapping helper ---

func mapOrderToUCPCheckout(order *Order, storeURL string) *UCPCheckout {
	checkout := &UCPCheckout{
		ID:    strconv.Itoa(order.ID),
		Links: []UCPLink{},
	}

	// Status mapping
	switch order.Status {
	case "pending":
		if order.Billing == nil || order.Billing.Email == "" {
			checkout.Status = "incomplete"
		} else {
			checkout.Status = "requires_escalation"
		}
	case "on-hold":
		checkout.Status = "complete_in_progress"
	case "processing":
		checkout.Status = "completed"
	case "completed":
		checkout.Status = "completed"
	case "cancelled":
		checkout.Status = "canceled"
	default:
		checkout.Status = "incomplete"
	}

	// Line items
	for _, li := range order.LineItems {
		totalCents, err := wcPriceToCents(li.Total)
		if err != nil {
			totalCents = 0
		}
		unitPrice := 0
		if li.Quantity > 0 {
			unitPrice = totalCents / li.Quantity
		}

		ucpLI := UCPLineItem{
			ID: strconv.Itoa(li.ID),
			Item: UCPItem{
				ID:    strconv.Itoa(li.ProductID),
				Title: li.Name,
				Price: unitPrice,
			},
			Quantity: li.Quantity,
			Totals: []UCPTotal{
				{Type: "total", DisplayText: li.Total, Amount: totalCents},
			},
		}
		if li.Image != nil {
			ucpLI.Item.ImageURL = li.Image.Src
		}
		checkout.LineItems = append(checkout.LineItems, ucpLI)
	}
	if checkout.LineItems == nil {
		checkout.LineItems = []UCPLineItem{}
	}

	// Buyer
	if order.Billing != nil && order.Billing.Email != "" {
		checkout.Buyer = &UCPBuyer{
			Email:     order.Billing.Email,
			FirstName: order.Billing.FirstName,
			LastName:  order.Billing.LastName,
		}
	}

	// Currency
	checkout.Currency = order.Currency
	if checkout.Currency == "" {
		checkout.Currency = "USD"
	}

	// Totals
	var totals []UCPTotal

	// Subtotal: sum of line item subtotals
	subtotalCents := 0
	for _, li := range order.LineItems {
		c, err := wcPriceToCents(li.Subtotal)
		if err == nil {
			subtotalCents += c
		}
	}
	if subtotalCents != 0 {
		totals = append(totals, UCPTotal{
			Type: "subtotal", DisplayText: centsToWcPrice(subtotalCents), Amount: subtotalCents,
		})
	}

	// Discount (negated)
	if discountCents, err := wcPriceToCents(order.DiscountTotal); err == nil && discountCents != 0 {
		totals = append(totals, UCPTotal{
			Type: "discount", DisplayText: order.DiscountTotal, Amount: -discountCents,
		})
	}

	// Fulfillment (shipping)
	if shippingCents, err := wcPriceToCents(order.ShippingTotal); err == nil && shippingCents != 0 {
		totals = append(totals, UCPTotal{
			Type: "fulfillment", DisplayText: order.ShippingTotal, Amount: shippingCents,
		})
	}

	// Tax
	if taxCents, err := wcPriceToCents(order.TotalTax); err == nil && taxCents != 0 {
		totals = append(totals, UCPTotal{
			Type: "tax", DisplayText: order.TotalTax, Amount: taxCents,
		})
	}

	// Total (always included)
	totalCents, _ := wcPriceToCents(order.Total)
	totals = append(totals, UCPTotal{
		Type: "total", DisplayText: order.Total, Amount: totalCents,
	})

	checkout.Totals = totals

	// Continue URL for requires_escalation
	if checkout.Status == "requires_escalation" {
		checkout.ContinueURL = fmt.Sprintf("%s/checkout/order-pay/%d/?pay_for_order=true&key=%s",
			strings.TrimRight(storeURL, "/"), order.ID, order.OrderKey)
	}

	// Payment handler
	checkout.Payment = &UCPPayment{
		Handlers: []UCPPaymentHandler{{
			ID:      "wc_payment_redirect",
			Name:    "com.woocommerce.payment_redirect",
			Version: "2026-01-11",
			Config:  map[string]string{"type": "REDIRECT"},
		}},
	}

	// Fulfillment expectations from shipping lines
	if len(order.ShippingLines) > 0 {
		var expectations []UCPFulfillmentExpectation
		var lineItemIDs []string
		for _, li := range checkout.LineItems {
			lineItemIDs = append(lineItemIDs, li.ID)
		}
		for _, sl := range order.ShippingLines {
			expectations = append(expectations, UCPFulfillmentExpectation{
				ID:          strconv.Itoa(sl.ID),
				Label:       sl.MethodTitle,
				LineItemIDs: lineItemIDs,
			})
		}
		checkout.Fulfillment = &UCPFulfillment{
			Expectations: expectations,
		}
	}

	return checkout
}

// --- Tool Handlers ---

func handleCreateCheckout(client *WooClient, storeURL string, a2uiEnabled bool) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input CreateCheckoutInput
		if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return nil, fmt.Errorf("parse create checkout input: %w", err)
		}

		// Convert to WC line items
		var lineItems []LineItem
		for _, cli := range input.LineItems {
			pid, err := strconv.Atoi(cli.ItemID)
			if err != nil {
				return nil, fmt.Errorf("invalid item_id %q: %w", cli.ItemID, err)
			}
			lineItems = append(lineItems, LineItem{ProductID: pid, Quantity: cli.Quantity})
		}

		// Create the order
		order, err := client.CreateOrder(ctx, lineItems)
		if err != nil {
			return nil, fmt.Errorf("create order: %w", err)
		}

		// Set buyer info if provided
		if input.Buyer != nil {
			order, err = client.UpdateOrder(ctx, order.ID, map[string]any{
				"billing": map[string]string{
					"first_name": input.Buyer.FirstName,
					"last_name":  input.Buyer.LastName,
					"email":      input.Buyer.Email,
				},
			})
			if err != nil {
				return nil, fmt.Errorf("update order billing: %w", err)
			}
		}

		// Fetch full order to get all computed fields
		order, err = client.GetOrder(ctx, order.ID)
		if err != nil {
			return nil, fmt.Errorf("get order: %w", err)
		}

		checkout := mapOrderToUCPCheckout(order, storeURL)
		data, err := json.Marshal(checkout)
		if err != nil {
			return nil, fmt.Errorf("marshal checkout: %w", err)
		}
		content := []mcp.Content{&mcp.TextContent{Text: string(data)}}
		if a2uiEnabled {
			content = append(content, a2uiToEmbeddedResource(CheckoutCard(*checkout)))
		}
		return &mcp.CallToolResult{Content: content}, nil
	}
}

func handleGetCheckout(client *WooClient, storeURL string, a2uiEnabled bool) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return nil, fmt.Errorf("parse get checkout input: %w", err)
		}

		orderID, err := strconv.Atoi(input.ID)
		if err != nil {
			return nil, fmt.Errorf("invalid checkout id %q: %w", input.ID, err)
		}

		order, err := client.GetOrder(ctx, orderID)
		if err != nil {
			return nil, fmt.Errorf("get order: %w", err)
		}

		checkout := mapOrderToUCPCheckout(order, storeURL)
		data, err := json.Marshal(checkout)
		if err != nil {
			return nil, fmt.Errorf("marshal checkout: %w", err)
		}
		content := []mcp.Content{&mcp.TextContent{Text: string(data)}}
		if a2uiEnabled {
			content = append(content, a2uiToEmbeddedResource(CheckoutCard(*checkout)))
		}
		return &mcp.CallToolResult{Content: content}, nil
	}
}

func handleUpdateCheckout(client *WooClient, storeURL string, a2uiEnabled bool) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input UpdateCheckoutInput
		if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return nil, fmt.Errorf("parse update checkout input: %w", err)
		}

		orderID, err := strconv.Atoi(input.ID)
		if err != nil {
			return nil, fmt.Errorf("invalid checkout id %q: %w", input.ID, err)
		}

		payload := map[string]any{}

		if len(input.LineItems) > 0 {
			var wcItems []map[string]any
			for _, li := range input.LineItems {
				pid, err := strconv.Atoi(li.ItemID)
				if err != nil {
					return nil, fmt.Errorf("invalid item_id %q: %w", li.ItemID, err)
				}
				wcItems = append(wcItems, map[string]any{
					"product_id": pid,
					"quantity":   li.Quantity,
				})
			}
			payload["line_items"] = wcItems
		}

		if input.Buyer != nil {
			payload["billing"] = map[string]string{
				"first_name": input.Buyer.FirstName,
				"last_name":  input.Buyer.LastName,
				"email":      input.Buyer.Email,
			}
		}

		if len(payload) > 0 {
			_, err = client.UpdateOrder(ctx, orderID, payload)
			if err != nil {
				return nil, fmt.Errorf("update order: %w", err)
			}
		}

		order, err := client.GetOrder(ctx, orderID)
		if err != nil {
			return nil, fmt.Errorf("get order: %w", err)
		}

		checkout := mapOrderToUCPCheckout(order, storeURL)
		data, err := json.Marshal(checkout)
		if err != nil {
			return nil, fmt.Errorf("marshal checkout: %w", err)
		}
		content := []mcp.Content{&mcp.TextContent{Text: string(data)}}
		if a2uiEnabled {
			content = append(content, a2uiToEmbeddedResource(CheckoutCard(*checkout)))
		}
		return &mcp.CallToolResult{Content: content}, nil
	}
}

func handleCompleteCheckout(client *WooClient, storeURL string, a2uiEnabled bool) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return nil, fmt.Errorf("parse complete checkout input: %w", err)
		}

		orderID, err := strconv.Atoi(input.ID)
		if err != nil {
			return nil, fmt.Errorf("invalid checkout id %q: %w", input.ID, err)
		}

		order, err := client.GetOrder(ctx, orderID)
		if err != nil {
			return nil, fmt.Errorf("get order: %w", err)
		}

		// WC requires browser redirect for payment; map current state
		checkout := mapOrderToUCPCheckout(order, storeURL)
		data, err := json.Marshal(checkout)
		if err != nil {
			return nil, fmt.Errorf("marshal checkout: %w", err)
		}
		content := []mcp.Content{&mcp.TextContent{Text: string(data)}}
		if a2uiEnabled {
			content = append(content, a2uiToEmbeddedResource(CheckoutCard(*checkout)))
		}
		return &mcp.CallToolResult{Content: content}, nil
	}
}

func handleCancelCheckout(client *WooClient, storeURL string, a2uiEnabled bool) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return nil, fmt.Errorf("parse cancel checkout input: %w", err)
		}

		orderID, err := strconv.Atoi(input.ID)
		if err != nil {
			return nil, fmt.Errorf("invalid checkout id %q: %w", input.ID, err)
		}

		order, err := client.UpdateOrder(ctx, orderID, map[string]string{
			"status": "cancelled",
		})
		if err != nil {
			return nil, fmt.Errorf("cancel order: %w", err)
		}

		checkout := mapOrderToUCPCheckout(order, storeURL)
		data, err := json.Marshal(checkout)
		if err != nil {
			return nil, fmt.Errorf("marshal checkout: %w", err)
		}
		content := []mcp.Content{&mcp.TextContent{Text: string(data)}}
		if a2uiEnabled {
			content = append(content, a2uiToEmbeddedResource(CheckoutCard(*checkout)))
		}
		return &mcp.CallToolResult{Content: content}, nil
	}
}

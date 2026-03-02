package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func mapOrderToUCPOrder(order *Order, refunds []OrderRefund, notes []OrderNote, storeURL string) *UCPOrder {
	id := strconv.Itoa(order.ID)

	// Map line items
	lineItemStatus := orderStatusToLineItemStatus(order.Status)
	lineItems := make([]UCPOrderLineItem, 0, len(order.LineItems))
	for _, li := range order.LineItems {
		imageURL := ""
		if li.Image != nil {
			imageURL = li.Image.Src
		}
		subtotalCents, _ := wcPriceToCents(li.Subtotal)
		totalCents, _ := wcPriceToCents(li.Total)
		priceCents := int(li.Price * 100)

		lineItems = append(lineItems, UCPOrderLineItem{
			UCPLineItem: UCPLineItem{
				ID: strconv.Itoa(li.ID),
				Item: UCPItem{
					ID:       strconv.Itoa(li.ProductID),
					Title:    li.Name,
					Price:    priceCents,
					ImageURL: imageURL,
				},
				Quantity: li.Quantity,
				Totals: []UCPTotal{
					{Type: "subtotal", DisplayText: li.Subtotal, Amount: subtotalCents},
					{Type: "total", DisplayText: li.Total, Amount: totalCents},
				},
			},
			Status: lineItemStatus,
		})
	}

	// Build all line item IDs for fulfillment expectations
	allLineItemIDs := make([]string, 0, len(order.LineItems))
	for _, li := range order.LineItems {
		allLineItemIDs = append(allLineItemIDs, strconv.Itoa(li.ID))
	}

	// Fulfillment expectations from shipping lines
	expectations := make([]UCPFulfillmentExpectation, 0, len(order.ShippingLines))
	for _, sl := range order.ShippingLines {
		expectations = append(expectations, UCPFulfillmentExpectation{
			ID:          strconv.Itoa(sl.ID),
			Label:       sl.MethodTitle,
			LineItemIDs: allLineItemIDs,
		})
	}

	// Fulfillment events from order notes
	var events []UCPFulfillmentEvent
	for _, note := range notes {
		events = append(events, UCPFulfillmentEvent{
			ID:          strconv.Itoa(note.ID),
			Type:        "note",
			OccurredAt:  "",
			Description: note.Note,
		})
	}

	var fulfillment *UCPFulfillment
	if len(expectations) > 0 || len(events) > 0 {
		fulfillment = &UCPFulfillment{
			Expectations: expectations,
			Events:       events,
		}
	}

	// Adjustments from refunds
	var adjustments []UCPAdjustment
	for _, refund := range refunds {
		amountCents, _ := wcPriceToCents(refund.Total)
		if amountCents < 0 {
			amountCents = -amountCents
		}

		var adjLineItems []UCPAdjustmentLineItem
		for _, rli := range refund.LineItems {
			rliAmount, _ := wcPriceToCents(rli.Total)
			if rliAmount < 0 {
				rliAmount = -rliAmount
			}
			adjLineItems = append(adjLineItems, UCPAdjustmentLineItem{
				LineItemID: strconv.Itoa(rli.ID),
				Quantity:   rli.Quantity,
				Amount:     rliAmount,
			})
		}

		adjustments = append(adjustments, UCPAdjustment{
			ID:          strconv.Itoa(refund.ID),
			Type:        "refund",
			OccurredAt:  refund.CreatedAt,
			Status:      "completed",
			Amount:      amountCents,
			Description: refund.Reason,
			LineItems:   adjLineItems,
		})
	}

	// Totals
	subtotalCents := 0
	for _, li := range order.LineItems {
		c, _ := wcPriceToCents(li.Subtotal)
		subtotalCents += c
	}
	discountCents, _ := wcPriceToCents(order.DiscountTotal)
	shippingCents, _ := wcPriceToCents(order.ShippingTotal)
	taxCents, _ := wcPriceToCents(order.TotalTax)
	totalCents, _ := wcPriceToCents(order.Total)

	totals := []UCPTotal{
		{Type: "subtotal", DisplayText: centsToWcPrice(subtotalCents), Amount: subtotalCents},
		{Type: "discount", DisplayText: centsToWcPrice(-discountCents), Amount: -discountCents},
		{Type: "fulfillment", DisplayText: order.ShippingTotal, Amount: shippingCents},
		{Type: "tax", DisplayText: order.TotalTax, Amount: taxCents},
		{Type: "total", DisplayText: order.Total, Amount: totalCents},
	}

	return &UCPOrder{
		ID:           id,
		CheckoutID:   id,
		PermalinkURL: fmt.Sprintf("%s/my-account/view-order/%d/", strings.TrimRight(storeURL, "/"), order.ID),
		Status:       order.Status,
		LineItems:    lineItems,
		Fulfillment:  fulfillment,
		Adjustments:  adjustments,
		Totals:       totals,
	}
}

func orderStatusToLineItemStatus(status string) string {
	switch status {
	case "processing":
		return "processing"
	case "completed":
		return "fulfilled"
	case "on-hold":
		return "processing"
	default:
		return "processing"
	}
}

func handleGetOrder(client *WooClient, storeURL string) func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return nil, fmt.Errorf("parse input: %w", err)
		}
		orderID, err := strconv.Atoi(input.ID)
		if err != nil {
			return nil, fmt.Errorf("invalid order id %q: %w", input.ID, err)
		}

		order, err := client.GetOrder(ctx, orderID)
		if err != nil {
			return nil, fmt.Errorf("get order: %w", err)
		}
		refunds, err := client.GetOrderRefunds(ctx, orderID)
		if err != nil {
			return nil, fmt.Errorf("get order refunds: %w", err)
		}
		notes, err := client.GetOrderNotes(ctx, orderID)
		if err != nil {
			return nil, fmt.Errorf("get order notes: %w", err)
		}

		ucpOrder := mapOrderToUCPOrder(order, refunds, notes, storeURL)
		data, err := json.Marshal(ucpOrder)
		if err != nil {
			return nil, fmt.Errorf("marshal ucp order: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil
	}
}

func handleListOrders(client *WooClient, storeURL string) func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	type ListOrdersInput struct {
		PerPage int `json:"per_page,omitempty"`
	}

	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input ListOrdersInput
		if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return nil, fmt.Errorf("parse input: %w", err)
		}
		if input.PerPage == 0 {
			input.PerPage = 10
		}

		orders, err := client.GetOrders(ctx, input.PerPage)
		if err != nil {
			return nil, fmt.Errorf("list orders: %w", err)
		}

		ucpOrders := make([]*UCPOrder, 0, len(orders))
		for i := range orders {
			ucpOrders = append(ucpOrders, mapOrderToUCPOrder(&orders[i], nil, nil, storeURL))
		}

		data, err := json.Marshal(ucpOrders)
		if err != nil {
			return nil, fmt.Errorf("marshal ucp orders: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil
	}
}

package main

import (
	"encoding/json"
	"fmt"
)

// ProductCard renders an A2UI card for a single product.
func ProductCard(p Product) []json.RawMessage {
	sid := fmt.Sprintf("product-%d", p.ID)
	s := NewSurface(sid)

	imageURL := ""
	if len(p.Images) > 0 {
		imageURL = p.Images[0].Src
	}

	s.SetData("product", map[string]any{
		"id":             p.ID,
		"name":           p.Name,
		"formattedPrice": fmt.Sprintf("$%s", p.Price),
		"imageUrl":       imageURL,
		"description":    p.Description,
		"stockStatus":    p.StockStatus,
	})

	s.AddComponent("product-image", map[string]any{
		"type": "Image",
		"src":  BoundPath("product.imageUrl"),
		"alt":  BoundPath("product.name"),
	})

	s.AddComponent("product-title", map[string]any{
		"type":  "Text",
		"level": "h3",
		"text":  BoundPath("product.name"),
	})

	s.AddComponent("product-price-stock", map[string]any{
		"type":      "Row",
		"direction": "horizontal",
		"children": []map[string]any{
			{"type": "Text", "text": BoundPath("product.formattedPrice")},
			{"type": "Text", "text": BoundPath("product.stockStatus")},
		},
	})

	s.AddComponent("product-description", map[string]any{
		"type": "Text",
		"text": BoundPath("product.description"),
	})

	s.AddComponent("product-actions", map[string]any{
		"type":      "Row",
		"direction": "horizontal",
		"children": []map[string]any{
			{"type": "Button", "label": BoundLiteral("Add to Cart"), "action": "addToCart"},
		},
	})

	s.AddComponent("product-body", map[string]any{
		"type":      "Column",
		"direction": "vertical",
		"children":  []string{"product-image", "product-title", "product-price-stock", "product-description", "product-actions"},
	})

	s.AddComponent("product-card", map[string]any{
		"type": "Card",
		"body": "product-body",
	})

	return s.Render("product-card")
}

// ProductListCard renders an A2UI card for a list of products.
func ProductListCard(products []Product) []json.RawMessage {
	s := NewSurface("product-list")

	productData := make([]map[string]any, len(products))
	for i, p := range products {
		productData[i] = map[string]any{
			"id":             p.ID,
			"name":           p.Name,
			"formattedPrice": fmt.Sprintf("$%s", p.Price),
			"stockStatus":    p.StockStatus,
		}
	}
	s.SetData("products", productData)

	headerText := fmt.Sprintf("Search Results (%d)", len(products))
	s.AddComponent("list-header", map[string]any{
		"type":  "Text",
		"level": "h3",
		"text":  BoundLiteral(headerText),
	})

	rowIDs := []string{"list-header"}
	for i, p := range products {
		rowID := fmt.Sprintf("product-row-%d", i)
		s.AddComponent(rowID, map[string]any{
			"type":      "Row",
			"direction": "horizontal",
			"children": []map[string]any{
				{"type": "Text", "text": BoundLiteral(p.Name)},
				{"type": "Text", "text": BoundLiteral(fmt.Sprintf("$%s", p.Price))},
				{"type": "Text", "text": BoundLiteral(p.StockStatus)},
			},
		})
		rowIDs = append(rowIDs, rowID)
	}

	s.AddComponent("list-body", map[string]any{
		"type":      "Column",
		"direction": "vertical",
		"children":  rowIDs,
	})

	s.AddComponent("list-card", map[string]any{
		"type": "Card",
		"body": "list-body",
	})

	return s.Render("list-card")
}

// CheckoutCard renders an A2UI card for a UCP checkout.
func CheckoutCard(checkout UCPCheckout) []json.RawMessage {
	sid := fmt.Sprintf("checkout-%s", checkout.ID)
	s := NewSurface(sid)

	s.SetData("checkout", map[string]any{
		"id":     checkout.ID,
		"status": checkout.Status,
	})

	s.AddComponent("checkout-title", map[string]any{
		"type":  "Text",
		"level": "h3",
		"text":  BoundLiteral("Checkout"),
	})

	s.AddComponent("checkout-status", map[string]any{
		"type": "Text",
		"text": BoundPath("checkout.status"),
	})

	// Line items section
	itemIDs := []string{}
	for i, li := range checkout.LineItems {
		itemID := fmt.Sprintf("checkout-item-%d", i)
		s.AddComponent(itemID, map[string]any{
			"type":      "Row",
			"direction": "horizontal",
			"children": []map[string]any{
				{"type": "Text", "text": BoundLiteral(li.Item.Title)},
				{"type": "Text", "text": BoundLiteral(fmt.Sprintf("x%d", li.Quantity))},
			},
		})
		itemIDs = append(itemIDs, itemID)
	}

	s.AddComponent("checkout-items", map[string]any{
		"type":      "Column",
		"direction": "vertical",
		"children":  itemIDs,
	})

	// Totals section
	totalIDs := []string{}
	for i, t := range checkout.Totals {
		totalID := fmt.Sprintf("checkout-total-%d", i)
		s.AddComponent(totalID, map[string]any{
			"type":      "Row",
			"direction": "horizontal",
			"children": []map[string]any{
				{"type": "Text", "text": BoundLiteral(t.DisplayText)},
				{"type": "Text", "text": BoundLiteral(centsToWcPrice(t.Amount))},
			},
		})
		totalIDs = append(totalIDs, totalID)
	}

	s.AddComponent("checkout-totals", map[string]any{
		"type":      "Column",
		"direction": "vertical",
		"children":  totalIDs,
	})

	// Action buttons
	s.AddComponent("checkout-actions", map[string]any{
		"type":      "Row",
		"direction": "horizontal",
		"children": []map[string]any{
			{"type": "Button", "label": BoundLiteral("Continue"), "action": "continuePurchase"},
		},
	})

	bodyChildren := []string{"checkout-title", "checkout-status", "checkout-items", "checkout-totals", "checkout-actions"}
	s.AddComponent("checkout-body", map[string]any{
		"type":      "Column",
		"direction": "vertical",
		"children":  bodyChildren,
	})

	s.AddComponent("checkout-card", map[string]any{
		"type": "Card",
		"body": "checkout-body",
	})

	return s.Render("checkout-card")
}

// OrderCard renders an A2UI card for a UCP order.
func OrderCard(order UCPOrder) []json.RawMessage {
	sid := fmt.Sprintf("order-%s", order.ID)
	s := NewSurface(sid)

	s.SetData("order", map[string]any{
		"id":     order.ID,
		"status": order.Status,
	})

	s.AddComponent("order-title", map[string]any{
		"type":  "Text",
		"level": "h3",
		"text":  BoundLiteral(fmt.Sprintf("Order #%s", order.ID)),
	})

	s.AddComponent("order-status", map[string]any{
		"type": "Text",
		"text": BoundPath("order.status"),
	})

	// Line items
	itemIDs := []string{}
	for i, li := range order.LineItems {
		itemID := fmt.Sprintf("order-item-%d", i)
		s.AddComponent(itemID, map[string]any{
			"type":      "Row",
			"direction": "horizontal",
			"children": []map[string]any{
				{"type": "Text", "text": BoundLiteral(li.Item.Title)},
				{"type": "Text", "text": BoundLiteral(fmt.Sprintf("x%d", li.Quantity))},
				{"type": "Text", "text": BoundLiteral(li.Status)},
			},
		})
		itemIDs = append(itemIDs, itemID)
	}

	s.AddComponent("order-items", map[string]any{
		"type":      "Column",
		"direction": "vertical",
		"children":  itemIDs,
	})

	// Fulfillment info
	fulfillmentChildren := []string{}
	if order.Fulfillment != nil {
		for i, ev := range order.Fulfillment.Events {
			evID := fmt.Sprintf("order-fulfillment-event-%d", i)
			s.AddComponent(evID, map[string]any{
				"type":      "Row",
				"direction": "horizontal",
				"children": []map[string]any{
					{"type": "Text", "text": BoundLiteral(ev.Type)},
					{"type": "Text", "text": BoundLiteral(ev.Description)},
					{"type": "Text", "text": BoundLiteral(ev.OccurredAt)},
				},
			})
			fulfillmentChildren = append(fulfillmentChildren, evID)
		}
	}

	s.AddComponent("order-fulfillment", map[string]any{
		"type":      "Column",
		"direction": "vertical",
		"children":  fulfillmentChildren,
	})

	// Adjustments
	adjChildren := []string{}
	for i, adj := range order.Adjustments {
		adjID := fmt.Sprintf("order-adjustment-%d", i)
		s.AddComponent(adjID, map[string]any{
			"type":      "Row",
			"direction": "horizontal",
			"children": []map[string]any{
				{"type": "Text", "text": BoundLiteral(adj.Type)},
				{"type": "Text", "text": BoundLiteral(adj.Description)},
				{"type": "Text", "text": BoundLiteral(centsToWcPrice(adj.Amount))},
			},
		})
		adjChildren = append(adjChildren, adjID)
	}

	s.AddComponent("order-adjustments", map[string]any{
		"type":      "Column",
		"direction": "vertical",
		"children":  adjChildren,
	})

	// Totals
	totalIDs := []string{}
	for i, t := range order.Totals {
		totalID := fmt.Sprintf("order-total-%d", i)
		s.AddComponent(totalID, map[string]any{
			"type":      "Row",
			"direction": "horizontal",
			"children": []map[string]any{
				{"type": "Text", "text": BoundLiteral(t.DisplayText)},
				{"type": "Text", "text": BoundLiteral(centsToWcPrice(t.Amount))},
			},
		})
		totalIDs = append(totalIDs, totalID)
	}

	s.AddComponent("order-totals", map[string]any{
		"type":      "Column",
		"direction": "vertical",
		"children":  totalIDs,
	})

	bodyChildren := []string{"order-title", "order-status", "order-items", "order-fulfillment", "order-adjustments", "order-totals"}
	s.AddComponent("order-body", map[string]any{
		"type":      "Column",
		"direction": "vertical",
		"children":  bodyChildren,
	})

	s.AddComponent("order-card", map[string]any{
		"type": "Card",
		"body": "order-body",
	})

	return s.Render("order-card")
}


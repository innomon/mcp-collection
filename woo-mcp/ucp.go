package main

import (
	"fmt"
	"strconv"
	"strings"
)

// --- UCP Core Types ---

type UCPEnvelope struct {
	Version      string          `json:"version"`
	Capabilities []UCPCapability `json:"capabilities"`
}

type UCPCapability struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Spec    string `json:"spec"`
	Schema  string `json:"schema"`
	Extends string `json:"extends,omitempty"`
}

type UCPCheckout struct {
	ID          string           `json:"id"`
	Status      string           `json:"status"`
	LineItems   []UCPLineItem    `json:"line_items"`
	Buyer       *UCPBuyer        `json:"buyer,omitempty"`
	Currency    string           `json:"currency"`
	Totals      []UCPTotal       `json:"totals"`
	Links       []UCPLink        `json:"links"`
	ContinueURL string           `json:"continue_url,omitempty"`
	Payment     *UCPPayment      `json:"payment,omitempty"`
	Fulfillment *UCPFulfillment  `json:"fulfillment,omitempty"`
	Messages    []UCPMessage     `json:"messages,omitempty"`
}

type UCPLineItem struct {
	ID       string     `json:"id"`
	Item     UCPItem    `json:"item"`
	Quantity int        `json:"quantity"`
	Totals   []UCPTotal `json:"totals"`
	ParentID string     `json:"parent_id,omitempty"`
}

type UCPItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Price    int    `json:"price"`
	ImageURL string `json:"image_url,omitempty"`
	URL      string `json:"url,omitempty"`
}

type UCPTotal struct {
	Type        string `json:"type"`
	DisplayText string `json:"display_text"`
	Amount      int    `json:"amount"`
}

type UCPBuyer struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type UCPOrder struct {
	ID           string              `json:"id"`
	CheckoutID   string              `json:"checkout_id"`
	PermalinkURL string              `json:"permalink_url"`
	Status       string              `json:"status"`
	LineItems    []UCPOrderLineItem  `json:"line_items"`
	Fulfillment  *UCPFulfillment     `json:"fulfillment,omitempty"`
	Adjustments  []UCPAdjustment     `json:"adjustments,omitempty"`
	Totals       []UCPTotal          `json:"totals"`
}

type UCPOrderLineItem struct {
	UCPLineItem
	Status string `json:"status"`
}

type UCPFulfillment struct {
	Methods      []UCPFulfillmentMethod      `json:"methods,omitempty"`
	Expectations []UCPFulfillmentExpectation  `json:"expectations,omitempty"`
	Events       []UCPFulfillmentEvent        `json:"events,omitempty"`
}

type UCPFulfillmentMethod struct {
	Type   string                `json:"type"`
	Groups []UCPFulfillmentGroup `json:"groups"`
}

type UCPFulfillmentGroup struct {
	ID      string                 `json:"id"`
	Title   string                 `json:"title"`
	Options []UCPFulfillmentOption `json:"options"`
}

type UCPFulfillmentOption struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Price    int    `json:"price"`
	Currency string `json:"currency"`
}

type UCPFulfillmentExpectation struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	LineItemIDs []string `json:"line_item_ids"`
}

type UCPFulfillmentEvent struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	OccurredAt  string `json:"occurred_at"`
	Description string `json:"description"`
}

type UCPAdjustment struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	OccurredAt  string                 `json:"occurred_at"`
	Status      string                 `json:"status"`
	LineItems   []UCPAdjustmentLineItem `json:"line_items,omitempty"`
	Amount      int                    `json:"amount"`
	Description string                 `json:"description"`
}

type UCPAdjustmentLineItem struct {
	LineItemID string `json:"line_item_id"`
	Quantity   int    `json:"quantity"`
	Amount     int    `json:"amount"`
}

type UCPPayment struct {
	Handlers    []UCPPaymentHandler    `json:"handlers"`
	Instruments []UCPPaymentInstrument `json:"instruments,omitempty"`
}

type UCPPaymentHandler struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Version string            `json:"version"`
	Spec    string            `json:"spec,omitempty"`
	Config  map[string]string `json:"config,omitempty"`
}

type UCPPaymentInstrument struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type UCPLink struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type UCPMessage struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// --- UCP Profile (section 3.1) ---

type UCPProfile struct {
	UCP     UCPProfileUCP     `json:"ucp"`
	Payment UCPProfilePayment `json:"payment"`
}

type UCPProfileUCP struct {
	Version      string                       `json:"version"`
	Services     map[string]UCPProfileService `json:"services"`
	Capabilities []UCPCapability              `json:"capabilities"`
}

type UCPProfileService struct {
	Version string                `json:"version"`
	Spec    string                `json:"spec"`
	MCP     *UCPProfileEndpoint   `json:"mcp,omitempty"`
	REST    *UCPProfileEndpoint   `json:"rest,omitempty"`
}

type UCPProfileEndpoint struct {
	Schema   string `json:"schema"`
	Endpoint string `json:"endpoint"`
}

type UCPProfilePayment struct {
	Handlers []UCPPaymentHandler `json:"handlers"`
}

func NewDefaultUCPProfile(storeURL string) *UCPProfile {
	base := strings.TrimRight(storeURL, "/")
	return &UCPProfile{
		UCP: UCPProfileUCP{
			Version: "2026-01-11",
			Services: map[string]UCPProfileService{
				"dev.ucp.shopping": {
					Version: "2026-01-11",
					Spec:    "https://ucp.dev/specification/overview",
					MCP: &UCPProfileEndpoint{
						Schema:   "https://ucp.dev/services/shopping/mcp.openrpc.json",
						Endpoint: base + "/ucp/mcp",
					},
					REST: &UCPProfileEndpoint{
						Schema:   "https://ucp.dev/services/shopping/rest.openapi.json",
						Endpoint: base + "/ucp/v1",
					},
				},
			},
			Capabilities: []UCPCapability{
				{
					Name:    "dev.ucp.shopping.checkout",
					Version: "2026-01-11",
					Spec:    "https://ucp.dev/specification/checkout",
					Schema:  "https://ucp.dev/schemas/shopping/checkout.json",
				},
				{
					Name:    "dev.ucp.shopping.fulfillment",
					Version: "2026-01-11",
					Spec:    "https://ucp.dev/specification/fulfillment",
					Schema:  "https://ucp.dev/schemas/shopping/fulfillment.json",
					Extends: "dev.ucp.shopping.checkout",
				},
				{
					Name:    "dev.ucp.shopping.order",
					Version: "2026-01-11",
					Spec:    "https://ucp.dev/specification/order",
					Schema:  "https://ucp.dev/schemas/shopping/order.json",
				},
			},
		},
		Payment: UCPProfilePayment{
			Handlers: []UCPPaymentHandler{
				{
					ID:      "wc_payment_redirect",
					Name:    "com.woocommerce.payment_redirect",
					Version: "2026-01-11",
					Spec:    "https://woocommerce.com/docs/payment",
					Config: map[string]string{
						"type": "REDIRECT",
						"note": "WooCommerce handles payment via browser redirect to store checkout page",
					},
				},
			},
		},
	}
}

// --- Price Conversion Helpers ---

// wcPriceToCents converts a WooCommerce decimal price string (e.g. "19.99")
// to minor units (cents). It uses string manipulation to avoid float precision issues.
func wcPriceToCents(price string) (int, error) {
	if price == "" {
		return 0, fmt.Errorf("wcPriceToCents: empty price string")
	}

	parts := strings.SplitN(price, ".", 2)
	whole := parts[0]
	decimal := "00"
	if len(parts) == 2 {
		d := parts[1]
		switch {
		case len(d) == 0:
			decimal = "00"
		case len(d) == 1:
			decimal = d + "0"
		case len(d) == 2:
			decimal = d
		default:
			decimal = d[:2]
		}
	}

	combined := whole + decimal
	cents, err := strconv.Atoi(combined)
	if err != nil {
		return 0, fmt.Errorf("wcPriceToCents: invalid price %q: %w", price, err)
	}
	return cents, nil
}

// centsToWcPrice converts minor units (e.g. 1999) to a WooCommerce decimal
// price string (e.g. "19.99").
func centsToWcPrice(cents int) string {
	whole := cents / 100
	frac := cents % 100
	if frac < 0 {
		frac = -frac
	}
	return fmt.Sprintf("%d.%02d", whole, frac)
}

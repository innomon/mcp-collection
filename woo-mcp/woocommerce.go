package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type WooClient struct {
	baseURL        string
	consumerKey    string
	consumerSecret string
	httpClient     *http.Client
}

func NewWooClient(baseURL, consumerKey, consumerSecret string) *WooClient {
	return &WooClient{
		baseURL:        strings.TrimRight(baseURL, "/"),
		consumerKey:    consumerKey,
		consumerSecret: consumerSecret,
		httpClient:     &http.Client{},
	}
}

type Product struct {
	ID              int                `json:"id"`
	Name            string             `json:"name"`
	Slug            string             `json:"slug"`
	Price           string             `json:"price"`
	RegularPrice    string             `json:"regular_price"`
	SalePrice       string             `json:"sale_price"`
	Description     string             `json:"short_description"`
	LongDescription string             `json:"description"`
	Permalink       string             `json:"permalink"`
	StockStatus     string             `json:"stock_status"`
	StockQuantity   *int               `json:"stock_quantity"`
	ManageStock     bool               `json:"manage_stock"`
	Images          []ProductImage     `json:"images"`
	Categories      []ProductCategory  `json:"categories"`
	Variations      []int              `json:"variations"`
	Attributes      []ProductAttribute `json:"attributes"`
	Type            string             `json:"type"`
}

type ProductImage struct {
	ID  int    `json:"id"`
	Src string `json:"src"`
	Alt string `json:"alt"`
}

type ProductCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type ProductAttribute struct {
	ID      int      `json:"id"`
	Name    string   `json:"name"`
	Options []string `json:"options"`
}

type ProductVariation struct {
	ID            int                  `json:"id"`
	Price         string               `json:"price"`
	RegularPrice  string               `json:"regular_price"`
	SalePrice     string               `json:"sale_price"`
	StockStatus   string               `json:"stock_status"`
	StockQuantity *int                 `json:"stock_quantity"`
	Attributes    []VariationAttribute `json:"attributes"`
	Image         *ProductImage        `json:"image"`
}

type VariationAttribute struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Option string `json:"option"`
}

type Order struct {
	ID            int              `json:"id"`
	Status        string           `json:"status"`
	Total         string           `json:"total"`
	Currency      string           `json:"currency"`
	OrderKey      string           `json:"order_key"`
	CreatedAt     string           `json:"date_created"`
	Subtotal      string           `json:"subtotal,omitempty"`
	DiscountTotal string           `json:"discount_total"`
	ShippingTotal string           `json:"shipping_total"`
	TotalTax      string           `json:"total_tax"`
	LineItems     []OrderLineItem  `json:"line_items,omitempty"`
	Billing       *OrderAddress    `json:"billing,omitempty"`
	Shipping      *OrderAddress    `json:"shipping,omitempty"`
	ShippingLines []ShippingLine   `json:"shipping_lines,omitempty"`
	MetaData      []OrderMeta      `json:"meta_data,omitempty"`
	Refunds       []OrderRefundRef `json:"refunds,omitempty"`
}

type OrderLineItem struct {
	ID        int           `json:"id"`
	Name      string        `json:"name"`
	ProductID int           `json:"product_id"`
	Quantity  int           `json:"quantity"`
	Subtotal  string        `json:"subtotal"`
	Total     string        `json:"total"`
	Price     float64       `json:"price"`
	Image     *ProductImage `json:"image,omitempty"`
}

type OrderAddress struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Address1  string `json:"address_1"`
	Address2  string `json:"address_2"`
	City      string `json:"city"`
	State     string `json:"state"`
	Postcode  string `json:"postcode"`
	Country   string `json:"country"`
}

type ShippingLine struct {
	ID          int    `json:"id"`
	MethodID    string `json:"method_id"`
	MethodTitle string `json:"method_title"`
	Total       string `json:"total"`
}

type OrderMeta struct {
	ID    int    `json:"id"`
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type OrderRefundRef struct {
	ID     int    `json:"id"`
	Reason string `json:"reason"`
	Total  string `json:"total"`
}

type OrderRefund struct {
	ID        int              `json:"id"`
	Reason    string           `json:"reason"`
	Total     string           `json:"total"`
	CreatedAt string           `json:"date_created"`
	LineItems []RefundLineItem `json:"line_items,omitempty"`
}

type RefundLineItem struct {
	ID        int    `json:"id"`
	ProductID int    `json:"product_id"`
	Quantity  int    `json:"quantity"`
	Total     string `json:"total"`
}

type ShippingZone struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ShippingMethod struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	MethodID string `json:"method_id"`
	Cost     string `json:"cost,omitempty"`
}

type StoreSetting struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

type WPPage struct {
	ID      int        `json:"id"`
	Title   WPRendered `json:"title"`
	Content WPRendered `json:"content"`
	Slug    string     `json:"slug"`
}

type WPRendered struct {
	Rendered string `json:"rendered"`
}

type SearchParams struct {
	Query    string
	Category string
	MinPrice string
	MaxPrice string
	PerPage  int
	Page     int
}

type OrderNote struct {
	ID   int    `json:"id"`
	Note string `json:"note"`
}

type LineItem struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}

func (c *WooClient) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.consumerKey, c.consumerSecret)
	if method == http.MethodPost || method == http.MethodPut {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}

func (c *WooClient) SearchProducts(ctx context.Context, query string) ([]Product, error) {
	path := "/wp-json/wc/v3/products?search=" + url.QueryEscape(query)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search products: unexpected status %d", resp.StatusCode)
	}
	var products []Product
	if err := json.NewDecoder(resp.Body).Decode(&products); err != nil {
		return nil, err
	}
	return products, nil
}

func (c *WooClient) GetOrders(ctx context.Context, perPage int) ([]Order, error) {
	path := "/wp-json/wc/v3/orders?per_page=" + strconv.Itoa(perPage) + "&orderby=date"
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get orders: unexpected status %d", resp.StatusCode)
	}
	var orders []Order
	if err := json.NewDecoder(resp.Body).Decode(&orders); err != nil {
		return nil, err
	}
	return orders, nil
}

func (c *WooClient) CreateOrder(ctx context.Context, lineItems []LineItem) (*Order, error) {
	payload := struct {
		LineItems []LineItem `json:"line_items"`
		Status    string     `json:"status"`
	}{
		LineItems: lineItems,
		Status:    "pending",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	resp, err := c.doRequest(ctx, http.MethodPost, "/wp-json/wc/v3/orders", strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("create order: unexpected status %d", resp.StatusCode)
	}
	var order Order
	if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (c *WooClient) CreateNote(ctx context.Context, orderID int, note string) (*OrderNote, error) {
	payload := struct {
		Note string `json:"note"`
	}{Note: note}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/wp-json/wc/v3/orders/%d/notes", orderID)
	resp, err := c.doRequest(ctx, http.MethodPost, path, strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("create note: unexpected status %d", resp.StatusCode)
	}
	var orderNote OrderNote
	if err := json.NewDecoder(resp.Body).Decode(&orderNote); err != nil {
		return nil, err
	}
	return &orderNote, nil
}

func (c *WooClient) GetProduct(ctx context.Context, id int) (*Product, error) {
	path := fmt.Sprintf("/wp-json/wc/v3/products/%d", id)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get product: unexpected status %d", resp.StatusCode)
	}
	var product Product
	if err := json.NewDecoder(resp.Body).Decode(&product); err != nil {
		return nil, err
	}
	return &product, nil
}

func (c *WooClient) GetProductVariations(ctx context.Context, productID int) ([]ProductVariation, error) {
	path := fmt.Sprintf("/wp-json/wc/v3/products/%d/variations", productID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get product variations: unexpected status %d", resp.StatusCode)
	}
	var variations []ProductVariation
	if err := json.NewDecoder(resp.Body).Decode(&variations); err != nil {
		return nil, err
	}
	return variations, nil
}

func (c *WooClient) GetProductCategories(ctx context.Context) ([]ProductCategory, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/wp-json/wc/v3/products/categories?per_page=100", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get product categories: unexpected status %d", resp.StatusCode)
	}
	var categories []ProductCategory
	if err := json.NewDecoder(resp.Body).Decode(&categories); err != nil {
		return nil, err
	}
	return categories, nil
}

func (c *WooClient) SearchProductsAdvanced(ctx context.Context, params SearchParams) ([]Product, error) {
	q := url.Values{}
	if params.Query != "" {
		q.Set("search", params.Query)
	}
	if params.Category != "" {
		q.Set("category", params.Category)
	}
	if params.MinPrice != "" {
		q.Set("min_price", params.MinPrice)
	}
	if params.MaxPrice != "" {
		q.Set("max_price", params.MaxPrice)
	}
	if params.PerPage > 0 {
		q.Set("per_page", strconv.Itoa(params.PerPage))
	}
	if params.Page > 0 {
		q.Set("page", strconv.Itoa(params.Page))
	}
	path := "/wp-json/wc/v3/products?" + q.Encode()
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search products advanced: unexpected status %d", resp.StatusCode)
	}
	var products []Product
	if err := json.NewDecoder(resp.Body).Decode(&products); err != nil {
		return nil, err
	}
	return products, nil
}

func (c *WooClient) GetOrder(ctx context.Context, id int) (*Order, error) {
	path := fmt.Sprintf("/wp-json/wc/v3/orders/%d", id)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get order: unexpected status %d", resp.StatusCode)
	}
	var order Order
	if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (c *WooClient) GetOrderRefunds(ctx context.Context, orderID int) ([]OrderRefund, error) {
	path := fmt.Sprintf("/wp-json/wc/v3/orders/%d/refunds", orderID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get order refunds: unexpected status %d", resp.StatusCode)
	}
	var refunds []OrderRefund
	if err := json.NewDecoder(resp.Body).Decode(&refunds); err != nil {
		return nil, err
	}
	return refunds, nil
}

func (c *WooClient) GetOrderNotes(ctx context.Context, orderID int) ([]OrderNote, error) {
	path := fmt.Sprintf("/wp-json/wc/v3/orders/%d/notes", orderID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get order notes: unexpected status %d", resp.StatusCode)
	}
	var notes []OrderNote
	if err := json.NewDecoder(resp.Body).Decode(&notes); err != nil {
		return nil, err
	}
	return notes, nil
}

func (c *WooClient) UpdateOrder(ctx context.Context, id int, payload any) (*Order, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal update order payload: %w", err)
	}
	path := fmt.Sprintf("/wp-json/wc/v3/orders/%d", id)
	resp, err := c.doRequest(ctx, http.MethodPut, path, strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update order: unexpected status %d", resp.StatusCode)
	}
	var order Order
	if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (c *WooClient) GetStoreSettings(ctx context.Context) ([]StoreSetting, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/wp-json/wc/v3/settings/general", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get store settings: unexpected status %d", resp.StatusCode)
	}
	var settings []StoreSetting
	if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
		return nil, err
	}
	return settings, nil
}

// WCCustomer represents a WooCommerce customer record.
type WCCustomer struct {
	ID        int    `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

// GetCustomerByEmail looks up a WooCommerce customer by email address.
// Returns nil if no customer is found with that email.
func (c *WooClient) GetCustomerByEmail(ctx context.Context, email string) (*WCCustomer, error) {
	path := "/wp-json/wc/v3/customers?email=" + url.QueryEscape(email)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("get customer by email: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get customer by email: unexpected status %d", resp.StatusCode)
	}
	var customers []WCCustomer
	if err := json.NewDecoder(resp.Body).Decode(&customers); err != nil {
		return nil, fmt.Errorf("get customer by email: %w", err)
	}
	if len(customers) == 0 {
		return nil, nil
	}
	return &customers[0], nil
}

// AuthenticateWPUser validates WordPress user credentials by attempting to
// access the WP REST API with the provided email/password via HTTP Basic Auth.
// This works with WordPress Application Passwords (WP 5.6+) or plugins that
// enable Basic Auth for standard credentials.
// Returns the authenticated user ID on success, or an error if credentials are invalid.
func (c *WooClient) AuthenticateWPUser(ctx context.Context, email, password string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/wp-json/wp/v2/users/me", nil)
	if err != nil {
		return 0, fmt.Errorf("authenticate wp user: %w", err)
	}
	req.SetBasicAuth(email, password)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("authenticate wp user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return 0, fmt.Errorf("invalid credentials")
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("authenticate wp user: unexpected status %d", resp.StatusCode)
	}
	var user struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return 0, fmt.Errorf("authenticate wp user: %w", err)
	}
	return user.ID, nil
}

func (c *WooClient) GetPages(ctx context.Context, search string) ([]WPPage, error) {
	path := "/wp-json/wp/v2/pages?search=" + url.QueryEscape(search) + "&per_page=10"
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get pages: unexpected status %d", resp.StatusCode)
	}
	var pages []WPPage
	if err := json.NewDecoder(resp.Body).Decode(&pages); err != nil {
		return nil, err
	}
	return pages, nil
}

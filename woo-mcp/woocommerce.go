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
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Price       string `json:"price"`
	Description string `json:"short_description"`
	Permalink   string `json:"permalink"`
}

type Order struct {
	ID        int    `json:"id"`
	Status    string `json:"status"`
	Total     string `json:"total"`
	Currency  string `json:"currency"`
	OrderKey  string `json:"order_key"`
	CreatedAt string `json:"date_created"`
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
	if method == http.MethodPost {
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

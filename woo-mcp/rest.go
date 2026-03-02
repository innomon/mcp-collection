package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type RESTServer struct {
	client *WooClient
	cfg    *Config
	oauth  *OAuthServer
}

func NewRESTServer(client *WooClient, cfg *Config, oauth *OAuthServer) *RESTServer {
	return &RESTServer{client: client, cfg: cfg, oauth: oauth}
}

func (rs *RESTServer) RegisterRoutes(mux *http.ServeMux) {
	// UCP Profile
	mux.HandleFunc("GET /.well-known/ucp", handleUCPProfile(rs.cfg))

	// OAuth 2.0 Identity Linking endpoints (not registered in super_user mode)
	if rs.oauth != nil && !rs.isSuperUser() {
		mux.HandleFunc("GET /.well-known/oauth-authorization-server", rs.oauth.HandleMetadata)
		mux.HandleFunc("GET /oauth2/authorize", rs.oauth.HandleAuthorize)
		mux.HandleFunc("POST /oauth2/authorize", rs.oauth.HandleAuthorize)
		mux.HandleFunc("POST /oauth2/token", rs.oauth.HandleToken)
		mux.HandleFunc("POST /oauth2/revoke", rs.oauth.HandleRevoke)
	}

	// Product endpoints
	mux.HandleFunc("GET /ucp/v1/products", rs.handleSearchProducts)
	mux.HandleFunc("GET /ucp/v1/products/{id}", rs.handleGetProduct)

	// Checkout endpoints
	mux.HandleFunc("POST /ucp/v1/checkout-sessions", rs.handleCreateCheckoutREST)
	mux.HandleFunc("GET /ucp/v1/checkout-sessions/{id}", rs.handleGetCheckoutREST)
	mux.HandleFunc("PATCH /ucp/v1/checkout-sessions/{id}", rs.handleUpdateCheckoutREST)
	mux.HandleFunc("POST /ucp/v1/checkout-sessions/{id}/complete", rs.handleCompleteCheckoutREST)
	mux.HandleFunc("POST /ucp/v1/checkout-sessions/{id}/cancel", rs.handleCancelCheckoutREST)

	// Order endpoints
	mux.HandleFunc("GET /ucp/v1/orders/{id}", rs.handleGetOrderREST)
}

func (rs *RESTServer) extractAuthenticatedEmail(r *http.Request) string {
	if rs.oauth == nil {
		return ""
	}
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	rec, err := rs.oauth.ValidateAccessToken(token)
	if err != nil {
		return ""
	}
	return rec.customerEmail
}

// isSuperUser returns true when the server operates in trusted backend mode.
func (rs *RESTServer) isSuperUser() bool {
	return rs.cfg.SuperUser
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func (rs *RESTServer) handleSearchProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	params := SearchParams{
		Query:    q.Get("query"),
		Category: q.Get("category"),
		MinPrice: q.Get("min_price"),
		MaxPrice: q.Get("max_price"),
		PerPage:  10,
	}

	products, err := rs.client.SearchProductsAdvanced(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusBadGateway, "search_failed", fmt.Sprintf("search products: %v", err))
		return
	}

	type catalogResult struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Price       int    `json:"price"`
		Currency    string `json:"currency"`
		ImageURL    string `json:"image_url"`
		URL         string `json:"url"`
		Description string `json:"description"`
		VariantID   string `json:"variant_id"`
		InStock     bool   `json:"in_stock"`
	}

	results := make([]catalogResult, 0, len(products))
	for _, p := range products {
		price, _ := wcPriceToCents(p.Price)
		imageURL := ""
		if len(p.Images) > 0 {
			imageURL = p.Images[0].Src
		}
		inStock := p.StockStatus == "instock" || p.StockStatus == ""
		results = append(results, catalogResult{
			ID:          strconv.Itoa(p.ID),
			Title:       p.Name,
			Price:       price,
			Currency:    "USD",
			ImageURL:    imageURL,
			URL:         p.Permalink,
			Description: p.Description,
			VariantID:   strconv.Itoa(p.ID),
			InStock:     inStock,
		})
	}

	writeJSON(w, http.StatusOK, results)
}

func (rs *RESTServer) handleGetProduct(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", fmt.Sprintf("invalid product id %q", idStr))
		return
	}

	product, err := rs.client.GetProduct(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadGateway, "get_failed", fmt.Sprintf("get product: %v", err))
		return
	}

	price, _ := wcPriceToCents(product.Price)
	imageURL := ""
	if len(product.Images) > 0 {
		imageURL = product.Images[0].Src
	}

	type categoryInfo struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	type variantInfo struct {
		ID         string            `json:"id"`
		Title      string            `json:"title"`
		Price      int               `json:"price"`
		InStock    bool              `json:"in_stock"`
		Attributes map[string]string `json:"attributes"`
	}
	type productResult struct {
		ID          string         `json:"id"`
		Title       string         `json:"title"`
		Price       int            `json:"price"`
		Currency    string         `json:"currency"`
		ImageURL    string         `json:"image_url"`
		URL         string         `json:"url"`
		Description string         `json:"description"`
		Categories  []categoryInfo `json:"categories"`
		Variants    []variantInfo  `json:"variants,omitempty"`
	}

	cats := make([]categoryInfo, 0, len(product.Categories))
	for _, c := range product.Categories {
		cats = append(cats, categoryInfo{ID: c.ID, Name: c.Name})
	}

	var variants []variantInfo
	if product.Type == "variable" && len(product.Variations) > 0 {
		variations, err := rs.client.GetProductVariations(r.Context(), product.ID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "get_variations_failed", fmt.Sprintf("get product variations: %v", err))
			return
		}
		variants = make([]variantInfo, 0, len(variations))
		for _, v := range variations {
			vPrice, _ := wcPriceToCents(v.Price)
			attrs := make(map[string]string, len(v.Attributes))
			titleParts := make([]string, 0, len(v.Attributes))
			for _, a := range v.Attributes {
				attrs[a.Name] = a.Option
				titleParts = append(titleParts, a.Option)
			}
			title := product.Name
			if len(titleParts) > 0 {
				title = product.Name + " - " + strings.Join(titleParts, ", ")
			}
			inStock := v.StockStatus == "instock" || v.StockStatus == ""
			variants = append(variants, variantInfo{
				ID:         strconv.Itoa(v.ID),
				Title:      title,
				Price:      vPrice,
				InStock:    inStock,
				Attributes: attrs,
			})
		}
	}

	result := productResult{
		ID:          strconv.Itoa(product.ID),
		Title:       product.Name,
		Price:       price,
		Currency:    "USD",
		ImageURL:    imageURL,
		URL:         product.Permalink,
		Description: product.Description,
		Categories:  cats,
		Variants:    variants,
	}

	writeJSON(w, http.StatusOK, result)
}

func (rs *RESTServer) handleCreateCheckoutREST(w http.ResponseWriter, r *http.Request) {
	var input CreateCheckoutInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", fmt.Sprintf("parse request body: %v", err))
		return
	}

	// Pre-fill buyer email from OAuth token if not provided
	if input.Buyer == nil {
		if email := rs.extractAuthenticatedEmail(r); email != "" {
			input.Buyer = &UCPBuyer{Email: email}
		}
	}

	var lineItems []LineItem
	for _, cli := range input.LineItems {
		pid, err := strconv.Atoi(cli.ItemID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_item_id", fmt.Sprintf("invalid item_id %q", cli.ItemID))
			return
		}
		lineItems = append(lineItems, LineItem{ProductID: pid, Quantity: cli.Quantity})
	}

	ctx := r.Context()
	order, err := rs.client.CreateOrder(ctx, lineItems)
	if err != nil {
		writeError(w, http.StatusBadGateway, "create_failed", fmt.Sprintf("create order: %v", err))
		return
	}

	if input.Buyer != nil {
		order, err = rs.client.UpdateOrder(ctx, order.ID, map[string]any{
			"billing": map[string]string{
				"first_name": input.Buyer.FirstName,
				"last_name":  input.Buyer.LastName,
				"email":      input.Buyer.Email,
			},
		})
		if err != nil {
			writeError(w, http.StatusBadGateway, "update_failed", fmt.Sprintf("update order billing: %v", err))
			return
		}
	}

	order, err = rs.client.GetOrder(ctx, order.ID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "get_failed", fmt.Sprintf("get order: %v", err))
		return
	}

	checkout := mapOrderToUCPCheckout(order, rs.cfg.StoreURL)
	writeJSON(w, http.StatusCreated, checkout)
}

func (rs *RESTServer) handleGetCheckoutREST(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	orderID, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", fmt.Sprintf("invalid checkout id %q", idStr))
		return
	}

	order, err := rs.client.GetOrder(r.Context(), orderID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "get_failed", fmt.Sprintf("get order: %v", err))
		return
	}

	checkout := mapOrderToUCPCheckout(order, rs.cfg.StoreURL)
	writeJSON(w, http.StatusOK, checkout)
}

func (rs *RESTServer) handleUpdateCheckoutREST(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	orderID, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", fmt.Sprintf("invalid checkout id %q", idStr))
		return
	}

	var input UpdateCheckoutInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", fmt.Sprintf("parse request body: %v", err))
		return
	}
	input.ID = idStr

	ctx := r.Context()
	payload := map[string]any{}

	if len(input.LineItems) > 0 {
		var wcItems []map[string]any
		for _, li := range input.LineItems {
			pid, err := strconv.Atoi(li.ItemID)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_item_id", fmt.Sprintf("invalid item_id %q", li.ItemID))
				return
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
		_, err = rs.client.UpdateOrder(ctx, orderID, payload)
		if err != nil {
			writeError(w, http.StatusBadGateway, "update_failed", fmt.Sprintf("update order: %v", err))
			return
		}
	}

	order, err := rs.client.GetOrder(ctx, orderID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "get_failed", fmt.Sprintf("get order: %v", err))
		return
	}

	checkout := mapOrderToUCPCheckout(order, rs.cfg.StoreURL)
	writeJSON(w, http.StatusOK, checkout)
}

func (rs *RESTServer) handleCompleteCheckoutREST(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	orderID, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", fmt.Sprintf("invalid checkout id %q", idStr))
		return
	}

	order, err := rs.client.GetOrder(r.Context(), orderID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "get_failed", fmt.Sprintf("get order: %v", err))
		return
	}

	checkout := mapOrderToUCPCheckout(order, rs.cfg.StoreURL)
	writeJSON(w, http.StatusOK, checkout)
}

func (rs *RESTServer) handleCancelCheckoutREST(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	orderID, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", fmt.Sprintf("invalid checkout id %q", idStr))
		return
	}

	order, err := rs.client.UpdateOrder(r.Context(), orderID, map[string]string{
		"status": "cancelled",
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "cancel_failed", fmt.Sprintf("cancel order: %v", err))
		return
	}

	checkout := mapOrderToUCPCheckout(order, rs.cfg.StoreURL)
	writeJSON(w, http.StatusOK, checkout)
}

func (rs *RESTServer) handleGetOrderREST(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	orderID, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", fmt.Sprintf("invalid order id %q", idStr))
		return
	}

	ctx := r.Context()
	order, err := rs.client.GetOrder(ctx, orderID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "get_failed", fmt.Sprintf("get order: %v", err))
		return
	}

	refunds, err := rs.client.GetOrderRefunds(ctx, orderID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "get_refunds_failed", fmt.Sprintf("get order refunds: %v", err))
		return
	}

	notes, err := rs.client.GetOrderNotes(ctx, orderID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "get_notes_failed", fmt.Sprintf("get order notes: %v", err))
		return
	}

	ucpOrder := mapOrderToUCPOrder(order, refunds, notes, rs.cfg.StoreURL)
	writeJSON(w, http.StatusOK, ucpOrder)
}

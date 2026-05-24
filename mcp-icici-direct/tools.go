package main

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolHandler struct {
	client *BreezeClient
}

// ----------------------------------------------------
// 1. Account Details & Holdings
// ----------------------------------------------------

type GetDematHoldingsInput struct{}

type GetDematHoldingsOutput struct {
	Holdings []map[string]any `json:"holdings" jsonschema:"List of shares in user's Demat account"`
}

func (h *ToolHandler) GetDematHoldings(ctx context.Context, req *mcp.CallToolRequest, input GetDematHoldingsInput) (*mcp.CallToolResult, GetDematHoldingsOutput, error) {
	log.Printf("MCP Tool Call: GetDematHoldings")
	res, err := h.client.CallAPI(ctx, "GET", "https://api.icicidirect.com/breezeapi/api/v1/dematholdings", nil, true)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to get demat holdings: %v", err)}},
		}, GetDematHoldingsOutput{}, nil
	}

	successData, _ := res["Success"].([]any)
	holdings := make([]map[string]any, 0, len(successData))
	for _, item := range successData {
		if m, ok := item.(map[string]any); ok {
			holdings = append(holdings, m)
		}
	}

	return nil, GetDematHoldingsOutput{Holdings: holdings}, nil
}

// ----------------------------------------------------
// 2. Funds Segment Allocation
// ----------------------------------------------------

type GetFundsInput struct{}

type GetFundsOutput struct {
	Funds map[string]any `json:"funds" jsonschema:" segregate balance limits details per segment"`
}

func (h *ToolHandler) GetFunds(ctx context.Context, req *mcp.CallToolRequest, input GetFundsInput) (*mcp.CallToolResult, GetFundsOutput, error) {
	log.Printf("MCP Tool Call: GetFunds")
	res, err := h.client.CallAPI(ctx, "GET", "https://api.icicidirect.com/breezeapi/api/v1/funds", nil, true)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to get funds: %v", err)}},
		}, GetFundsOutput{}, nil
	}

	successData, _ := res["Success"].(map[string]any)
	return nil, GetFundsOutput{Funds: successData}, nil
}

type SetFundsInput struct {
	Segment   string `json:"segment" jsonschema:"Segment to modify, e.g. Equity, F&O"`
	Action    string `json:"action" jsonschema:"Action type: 'add' or 'reduce'"`
	Amount    string `json:"amount" jsonschema:"The financial amount to allocate or reduce"`
}

type SetFundsOutput struct {
	Result map[string]any `json:"result" jsonschema:"Result confirmation of funds transfer"`
}

func (h *ToolHandler) SetFunds(ctx context.Context, req *mcp.CallToolRequest, input SetFundsInput) (*mcp.CallToolResult, SetFundsOutput, error) {
	log.Printf("MCP Tool Call: SetFunds segment=%s action=%s amount=%s", input.Segment, input.Action, input.Amount)
	payload := map[string]any{
		"segment":           input.Segment,
		"action":            input.Action,
		"allocation_amount": input.Amount,
	}

	res, err := h.client.CallAPI(ctx, "POST", "https://api.icicidirect.com/breezeapi/api/v1/funds", payload, true)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to set funds: %v", err)}},
		}, SetFundsOutput{}, nil
	}

	successData, _ := res["Success"].(map[string]any)
	return nil, SetFundsOutput{Result: successData}, nil
}

// ----------------------------------------------------
// 3. Margin Information
// ----------------------------------------------------

type GetMarginsInput struct{}

type GetMarginsOutput struct {
	Margins map[string]any `json:"margins" jsonschema:"Active trading limits and margins details"`
}

func (h *ToolHandler) GetMargins(ctx context.Context, req *mcp.CallToolRequest, input GetMarginsInput) (*mcp.CallToolResult, GetMarginsOutput, error) {
	log.Printf("MCP Tool Call: GetMargins")
	res, err := h.client.CallAPI(ctx, "GET", "https://api.icicidirect.com/breezeapi/api/v1/margins", nil, true)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to get margins: %v", err)}},
		}, GetMarginsOutput{}, nil
	}

	successData, _ := res["Success"].(map[string]any)
	return nil, GetMarginsOutput{Margins: successData}, nil
}

// ----------------------------------------------------
// 4. Real-time Quotes & Option Chain
// ----------------------------------------------------

type GetQuotesInput struct {
	StockCode    string `json:"stock_code" jsonschema:"Stock symbol, e.g. ITC, RELIANCE, TCS"`
	ExchangeCode string `json:"exchange_code" jsonschema:"Exchange code, e.g. NSE, NFO"`
}

type GetQuotesOutput struct {
	Quote []map[string]any `json:"quote" jsonschema:"Price, bids, asks and day volumes information"`
}

func (h *ToolHandler) GetQuotes(ctx context.Context, req *mcp.CallToolRequest, input GetQuotesInput) (*mcp.CallToolResult, GetQuotesOutput, error) {
	log.Printf("MCP Tool Call: GetQuotes stock=%s exch=%s", input.StockCode, input.ExchangeCode)
	payload := map[string]any{
		"stock_code":    input.StockCode,
		"exchange_code": input.ExchangeCode,
	}

	res, err := h.client.CallAPI(ctx, "GET", "https://api.icicidirect.com/breezeapi/api/v1/quotes", payload, true)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to get quotes: %v", err)}},
		}, GetQuotesOutput{}, nil
	}

	successList, _ := res["Success"].([]any)
	quotes := make([]map[string]any, 0, len(successList))
	for _, item := range successList {
		if m, ok := item.(map[string]any); ok {
			quotes = append(quotes, m)
		}
	}

	return nil, GetQuotesOutput{Quote: quotes}, nil
}

// ----------------------------------------------------
// 5. Historical Candlesticks (OHLCV)
// ----------------------------------------------------

type GetHistoricalChartsInput struct {
	StockCode    string `json:"stock_code" jsonschema:"Stock code, e.g. NIFTY, ITC"`
	ExchangeCode string `json:"exchange_code" jsonschema:"Exchange, e.g. NSE, NFO"`
	FromDate     string `json:"from_date" jsonschema:"ISO 8601 UTC date string, e.g. 2026-05-20T00:00:00.000Z"`
	ToDate       string `json:"to_date" jsonschema:"ISO 8601 UTC date string, e.g. 2026-05-23T23:59:59.000Z"`
	Interval     string `json:"interval" jsonschema:"Interval: '1minute', '5minute', '30minute', '1day'"`
	ProductType  string `json:"product_type" jsonschema:"e.g. futures, options, cash"`
	ExpiryDate   string `json:"expiry_date,omitempty" jsonschema:"Expiry date for derivatives, e.g. 2026-06-25"`
	Right        string `json:"right,omitempty" jsonschema:"Right type: call, put, others"`
	StrikePrice  string `json:"strike_price,omitempty" jsonschema:"Strike price, e.g. 22000"`
}

type GetHistoricalChartsOutput struct {
	Candles []map[string]any `json:"candles" jsonschema:"List of OHLCV historical candle points"`
}

func (h *ToolHandler) GetHistoricalCharts(ctx context.Context, req *mcp.CallToolRequest, input GetHistoricalChartsInput) (*mcp.CallToolResult, GetHistoricalChartsOutput, error) {
	log.Printf("MCP Tool Call: GetHistoricalCharts stock=%s interval=%s", input.StockCode, input.Interval)
	payload := map[string]any{
		"stock_code":    input.StockCode,
		"exchange_code": input.ExchangeCode,
		"from_date":     input.FromDate,
		"to_date":       input.ToDate,
		"interval":      input.Interval,
		"product_type":  input.ProductType,
	}

	if input.ExpiryDate != "" {
		payload["expiry_date"] = input.ExpiryDate
	}
	if input.Right != "" {
		payload["right"] = input.Right
	}
	if input.StrikePrice != "" {
		payload["strike_price"] = input.StrikePrice
	}

	// We use the V2 historical charts endpoint which is standard.
	res, err := h.client.CallAPI(ctx, "GET", "https://breezeapi.icicidirect.com/api/v2/historicalcharts", payload, true)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to get historical charts: %v", err)}},
		}, GetHistoricalChartsOutput{}, nil
	}

	successList, _ := res["Success"].([]any)
	candles := make([]map[string]any, 0, len(successList))
	for _, item := range successList {
		if m, ok := item.(map[string]any); ok {
			candles = append(candles, m)
		}
	}

	return nil, GetHistoricalChartsOutput{Candles: candles}, nil
}

// ----------------------------------------------------
// 6. Portfolio & Open Positions
// ----------------------------------------------------

type GetPortfolioHoldingsInput struct{}

type GetPortfolioHoldingsOutput struct {
	Holdings []map[string]any `json:"holdings" jsonschema:"Active long-term investment equity stock holdings"`
}

func (h *ToolHandler) GetPortfolioHoldings(ctx context.Context, req *mcp.CallToolRequest, input GetPortfolioHoldingsInput) (*mcp.CallToolResult, GetPortfolioHoldingsOutput, error) {
	log.Printf("MCP Tool Call: GetPortfolioHoldings")
	res, err := h.client.CallAPI(ctx, "GET", "https://api.icicidirect.com/breezeapi/api/v1/portfolioholdings", nil, true)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to get portfolio holdings: %v", err)}},
		}, GetPortfolioHoldingsOutput{}, nil
	}

	successList, _ := res["Success"].([]any)
	holdings := make([]map[string]any, 0, len(successList))
	for _, item := range successList {
		if m, ok := item.(map[string]any); ok {
			holdings = append(holdings, m)
		}
	}

	return nil, GetPortfolioHoldingsOutput{Holdings: holdings}, nil
}

type GetPortfolioPositionsInput struct{}

type GetPortfolioPositionsOutput struct {
	Positions []map[string]any `json:"positions" jsonschema:"List of open intraday or derivative trading positions"`
}

func (h *ToolHandler) GetPortfolioPositions(ctx context.Context, req *mcp.CallToolRequest, input GetPortfolioPositionsInput) (*mcp.CallToolResult, GetPortfolioPositionsOutput, error) {
	log.Printf("MCP Tool Call: GetPortfolioPositions")
	res, err := h.client.CallAPI(ctx, "GET", "https://api.icicidirect.com/breezeapi/api/v1/portfoliopositions", nil, true)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to get portfolio positions: %v", err)}},
		}, GetPortfolioPositionsOutput{}, nil
	}

	successList, _ := res["Success"].([]any)
	positions := make([]map[string]any, 0, len(successList))
	for _, item := range successList {
		if m, ok := item.(map[string]any); ok {
			positions = append(positions, m)
		}
	}

	return nil, GetPortfolioPositionsOutput{Positions: positions}, nil
}

// ----------------------------------------------------
// 7. Order Execution & Books
// ----------------------------------------------------

type PlaceOrderInput struct {
	StockCode         string `json:"stock_code" jsonschema:"Stock symbol, e.g. ITC, RELIANCE"`
	ExchangeCode      string `json:"exchange_code" jsonschema:"Exchange, e.g. NSE, NFO"`
	Product           string `json:"product" jsonschema:"Product type: cash, options, futures, margin"`
	Action            string `json:"action" jsonschema:"Action: buy or sell"`
	OrderType         string `json:"order_type" jsonschema:"Order type: limit, stoploss"`
	Quantity          string `json:"quantity" jsonschema:"Number of shares or lots to trade"`
	Price             string `json:"price" jsonschema:"Limit price value. Note: Market orders are NOT permitted by circular, so always place limit."`
	Validity          string `json:"validity" jsonschema:"Validity mode: day, ioc"`
	Stoploss          string `json:"stoploss,omitempty" jsonschema:"Trigger price for Stop Loss orders"`
	ValidityDate      string `json:"validity_date,omitempty" jsonschema:"ISO Date for validity duration"`
	DisclosedQuantity string `json:"disclosed_quantity,omitempty" jsonschema:"Quantity to disclose publicly"`
	ExpiryDate        string `json:"expiry_date,omitempty" jsonschema:"Derivative expiry date"`
	Right             string `json:"right,omitempty" jsonschema:"Call/Put for options"`
	StrikePrice       string `json:"strike_price,omitempty" jsonschema:"Option strike price"`
	UserRemark        string `json:"user_remark,omitempty" jsonschema:"Custom annotation tag for trading order"`
}

type PlaceOrderOutput struct {
	Result map[string]any `json:"result" jsonschema:"Execution details and order_id reference"`
}

func (h *ToolHandler) PlaceOrder(ctx context.Context, req *mcp.CallToolRequest, input PlaceOrderInput) (*mcp.CallToolResult, PlaceOrderOutput, error) {
	log.Printf("MCP Tool Call: PlaceOrder stock=%s quantity=%s price=%s action=%s", input.StockCode, input.Quantity, input.Price, input.Action)

	payload := map[string]any{
		"stock_code":         input.StockCode,
		"exchange_code":      input.ExchangeCode,
		"product":            input.Product,
		"action":             input.Action,
		"order_type":         input.OrderType,
		"quantity":           input.Quantity,
		"price":              input.Price,
		"validity":           input.Validity,
		"stoploss":           input.Stoploss,
		"validity_date":      input.ValidityDate,
		"disclosed_quantity": input.DisclosedQuantity,
		"expiry_date":        input.ExpiryDate,
		"right":              input.Right,
		"strike_price":       input.StrikePrice,
		"user_remark":        input.UserRemark,
	}

	res, err := h.client.CallAPI(ctx, "POST", "https://api.icicidirect.com/breezeapi/api/v1/order", payload, true)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to place order: %v", err)}},
		}, PlaceOrderOutput{}, nil
	}

	successData, _ := res["Success"].(map[string]any)
	return nil, PlaceOrderOutput{Result: successData}, nil
}

type GetOrderDetailsInput struct {
	OrderID      string `json:"order_id" jsonschema:"Order transaction reference ID"`
	ExchangeCode string `json:"exchange_code" jsonschema:"Exchange: NSE or NFO"`
}

type GetOrderDetailsOutput struct {
	OrderDetails []map[string]any `json:"order_details" jsonschema:"List matching order execution state"`
}

func (h *ToolHandler) GetOrderDetails(ctx context.Context, req *mcp.CallToolRequest, input GetOrderDetailsInput) (*mcp.CallToolResult, GetOrderDetailsOutput, error) {
	log.Printf("MCP Tool Call: GetOrderDetails order_id=%s", input.OrderID)
	payload := map[string]any{
		"order_id":      input.OrderID,
		"exchange_code": input.ExchangeCode,
	}

	res, err := h.client.CallAPI(ctx, "GET", "https://api.icicidirect.com/breezeapi/api/v1/order", payload, true)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to get order details: %v", err)}},
		}, GetOrderDetailsOutput{}, nil
	}

	successList, _ := res["Success"].([]any)
	details := make([]map[string]any, 0, len(successList))
	for _, item := range successList {
		if m, ok := item.(map[string]any); ok {
			details = append(details, m)
		}
	}

	return nil, GetOrderDetailsOutput{OrderDetails: details}, nil
}

type GetOrderListInput struct {
	ExchangeCode string `json:"exchange_code" jsonschema:"Exchange code, e.g. NSE or NFO"`
	FromDate     string `json:"from_date" jsonschema:"ISO start date to pull historical list"`
	ToDate       string `json:"to_date" jsonschema:"ISO end date to filter"`
}

type GetOrderListOutput struct {
	Orders []map[string]any `json:"orders" jsonschema:"Day's full active and executed order book"`
}

func (h *ToolHandler) GetOrderList(ctx context.Context, req *mcp.CallToolRequest, input GetOrderListInput) (*mcp.CallToolResult, GetOrderListOutput, error) {
	log.Printf("MCP Tool Call: GetOrderList exch=%s", input.ExchangeCode)
	payload := map[string]any{
		"exchange_code": input.ExchangeCode,
		"from_date":     input.FromDate,
		"to_date":       input.ToDate,
	}

	res, err := h.client.CallAPI(ctx, "GET", "https://api.icicidirect.com/breezeapi/api/v1/order", payload, true)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to get order list: %v", err)}},
		}, GetOrderListOutput{}, nil
	}

	successList, _ := res["Success"].([]any)
	orders := make([]map[string]any, 0, len(successList))
	for _, item := range successList {
		if m, ok := item.(map[string]any); ok {
			orders = append(orders, m)
		}
	}

	return nil, GetOrderListOutput{Orders: orders}, nil
}

type ModifyOrderInput struct {
	OrderID           string `json:"order_id" jsonschema:"Pending order transaction reference ID to alter"`
	ExchangeCode      string `json:"exchange_code" jsonschema:"Exchange, e.g. NSE, NFO"`
	Price             string `json:"price" jsonschema:"New limit price value"`
	Quantity          string `json:"quantity" jsonschema:"Modified trading quantity"`
	OrderType         string `json:"order_type" jsonschema:"Order type: limit, stoploss"`
	Validity          string `json:"validity" jsonschema:"Validity: day, ioc"`
	Stoploss          string `json:"stoploss,omitempty" jsonschema:"New trigger price if modifying stoploss"`
	DisclosedQuantity string `json:"disclosed_quantity,omitempty" jsonschema:"Disclosed quantity limit"`
}

type ModifyOrderOutput struct {
	Result map[string]any `json:"result" jsonschema:"Modification transaction confirmation"`
}

func (h *ToolHandler) ModifyOrder(ctx context.Context, req *mcp.CallToolRequest, input ModifyOrderInput) (*mcp.CallToolResult, ModifyOrderOutput, error) {
	log.Printf("MCP Tool Call: ModifyOrder order_id=%s new_price=%s qty=%s", input.OrderID, input.Price, input.Quantity)

	payload := map[string]any{
		"order_id":           input.OrderID,
		"exchange_code":      input.ExchangeCode,
		"price":              input.Price,
		"quantity":           input.Quantity,
		"order_type":         input.OrderType,
		"validity":           input.Validity,
		"stoploss":           input.Stoploss,
		"disclosed_quantity": input.DisclosedQuantity,
	}

	res, err := h.client.CallAPI(ctx, "PUT", "https://api.icicidirect.com/breezeapi/api/v1/order", payload, true)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to modify order: %v", err)}},
		}, ModifyOrderOutput{}, nil
	}

	successData, _ := res["Success"].(map[string]any)
	return nil, ModifyOrderOutput{Result: successData}, nil
}

type CancelOrderInput struct {
	OrderID      string `json:"order_id" jsonschema:"Order transaction reference ID to cancel"`
	ExchangeCode string `json:"exchange_code" jsonschema:"Exchange code: NSE or NFO"`
}

type CancelOrderOutput struct {
	Result map[string]any `json:"result" jsonschema:"Cancellation transaction details"`
}

func (h *ToolHandler) CancelOrder(ctx context.Context, req *mcp.CallToolRequest, input CancelOrderInput) (*mcp.CallToolResult, CancelOrderOutput, error) {
	log.Printf("MCP Tool Call: CancelOrder order_id=%s", input.OrderID)
	payload := map[string]any{
		"order_id":      input.OrderID,
		"exchange_code": input.ExchangeCode,
	}

	res, err := h.client.CallAPI(ctx, "DELETE", "https://api.icicidirect.com/breezeapi/api/v1/order", payload, true)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to cancel order: %v", err)}},
		}, CancelOrderOutput{}, nil
	}

	successData, _ := res["Success"].(map[string]any)
	return nil, CancelOrderOutput{Result: successData}, nil
}

// ----------------------------------------------------
// 8. Square Off Positions
// ----------------------------------------------------

type SquareOffInput struct {
	StockCode      string `json:"stock_code" jsonschema:"Stock symbol to close, e.g. ITC"`
	ExchangeCode   string `json:"exchange_code" jsonschema:"Exchange code: NSE or NFO"`
	Quantity       string `json:"quantity" jsonschema:"Outstanding quantity to liquidate"`
	ProductType    string `json:"product_type" jsonschema:"Segment product, e.g. cash, margin, futures"`
	Action         string `json:"action" jsonschema:"Opposing order action to cover: buy or sell"`
	OrderType      string `json:"order_type" jsonschema:"Order execution pricing type, e.g. limit"`
	Price          string `json:"price" jsonschema:"Limit price value. Use 0 or appropriate tick limit."`
	Validity       string `json:"validity" jsonschema:"Validity: day, ioc"`
	ExpiryDate     string `json:"expiry_date,omitempty" jsonschema:"Derivative expiry date if closing F&O"`
	Right          string `json:"right,omitempty" jsonschema:"Right type call/put for options"`
	StrikePrice    string `json:"strike_price,omitempty" jsonschema:"Option strike price reference"`
	StoplossPrice  string `json:"stoploss_price,omitempty" jsonschema:"Optional stop loss trigger"`
}

type SquareOffOutput struct {
	Result map[string]any `json:"result" jsonschema:"Liquidating position confirmation message"`
}

func (h *ToolHandler) SquareOff(ctx context.Context, req *mcp.CallToolRequest, input SquareOffInput) (*mcp.CallToolResult, SquareOffOutput, error) {
	log.Printf("MCP Tool Call: SquareOff stock=%s quantity=%s", input.StockCode, input.Quantity)

	payload := map[string]any{
		"stock_code":         input.StockCode,
		"exchange_code":      input.ExchangeCode,
		"quantity":           input.Quantity,
		"product_type":       input.ProductType,
		"action":             input.Action,
		"order_type":         input.OrderType,
		"price":              input.Price,
		"validity":           input.Validity,
		"expiry_date":        input.ExpiryDate,
		"right":              input.Right,
		"strike_price":       input.StrikePrice,
		"stoploss_price":     input.StoplossPrice,
	}

	res, err := h.client.CallAPI(ctx, "POST", "https://api.icicidirect.com/breezeapi/api/v1/squareoff", payload, true)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to square off position: %v", err)}},
		}, SquareOffOutput{}, nil
	}

	successData, _ := res["Success"].(map[string]any)
	return nil, SquareOffOutput{Result: successData}, nil
}

// ----------------------------------------------------
// 9. Good Till Triggered (GTT) Orders
// ----------------------------------------------------

type PlaceGttOrderInput struct {
	StockCode    string `json:"stock_code" jsonschema:"Stock symbol, e.g. TCS, ITC"`
	ExchangeCode string `json:"exchange_code" jsonschema:"Exchange: NSE or NFO"`
	Action       string `json:"action" jsonschema:"Order action: buy or sell"`
	TriggerPrice string `json:"trigger_price" jsonschema:"The trigger price to initiate order"`
	LimitPrice   string `json:"limit_price" jsonschema:"The limit execution price"`
	Quantity     string `json:"quantity" jsonschema:"Quantity limits"`
	ProductType  string `json:"product_type" jsonschema:"Segment product, e.g. cash, margin"`
	ExpiryDate   string `json:"expiry_date,omitempty" jsonschema:"Optional derivative expiry"`
	Right        string `json:"right,omitempty" jsonschema:"Call/Put options right"`
	StrikePrice  string `json:"strike_price,omitempty" jsonschema:"Strike price reference"`
}

type PlaceGttOrderOutput struct {
	Result map[string]any `json:"result" jsonschema:"Placed GTT transaction information"`
}

func (h *ToolHandler) PlaceGttOrder(ctx context.Context, req *mcp.CallToolRequest, input PlaceGttOrderInput) (*mcp.CallToolResult, PlaceGttOrderOutput, error) {
	log.Printf("MCP Tool Call: PlaceGttOrder stock=%s trigger=%s", input.StockCode, input.TriggerPrice)

	payload := map[string]any{
		"stock_code":    input.StockCode,
		"exchange_code": input.ExchangeCode,
		"action":        input.Action,
		"trigger_price": input.TriggerPrice,
		"limit_price":   input.LimitPrice,
		"quantity":      input.Quantity,
		"product_type":  input.ProductType,
	}

	if input.ExpiryDate != "" {
		payload["expiry_date"] = input.ExpiryDate
	}
	if input.Right != "" {
		payload["right"] = input.Right
	}
	if input.StrikePrice != "" {
		payload["strike_price"] = input.StrikePrice
	}

	res, err := h.client.CallAPI(ctx, "POST", "https://api.icicidirect.com/breezeapi/api/v1/gttorder", payload, true)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to place GTT order: %v", err)}},
		}, PlaceGttOrderOutput{}, nil
	}

	successData, _ := res["Success"].(map[string]any)
	return nil, PlaceGttOrderOutput{Result: successData}, nil
}

type GetGttOrderBookInput struct {
	ExchangeCode string `json:"exchange_code" jsonschema:"Exchange: NSE or NFO"`
}

type GetGttOrderBookOutput struct {
	GttOrders []map[string]any `json:"gtt_orders" jsonschema:"Active and triggered GTT orders portfolio"`
}

func (h *ToolHandler) GetGttOrderBook(ctx context.Context, req *mcp.CallToolRequest, input GetGttOrderBookInput) (*mcp.CallToolResult, GetGttOrderBookOutput, error) {
	log.Printf("MCP Tool Call: GetGttOrderBook")
	payload := map[string]any{
		"exchange_code": input.ExchangeCode,
	}

	res, err := h.client.CallAPI(ctx, "GET", "https://api.icicidirect.com/breezeapi/api/v1/gttorder", payload, true)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to get GTT order book: %v", err)}},
		}, GetGttOrderBookOutput{}, nil
	}

	successList, _ := res["Success"].([]any)
	orders := make([]map[string]any, 0, len(successList))
	for _, item := range successList {
		if m, ok := item.(map[string]any); ok {
			orders = append(orders, m)
		}
	}

	return nil, GetGttOrderBookOutput{GttOrders: orders}, nil
}

type CancelGttOrderInput struct {
	GttID        string `json:"gtt_id" jsonschema:"GTT active order transaction ID to cancel"`
	ExchangeCode string `json:"exchange_code" jsonschema:"Exchange: NSE or NFO"`
}

type CancelGttOrderOutput struct {
	Result map[string]any `json:"result" jsonschema:"Cancellation confirmation"`
}

func (h *ToolHandler) CancelGttOrder(ctx context.Context, req *mcp.CallToolRequest, input CancelGttOrderInput) (*mcp.CallToolResult, CancelGttOrderOutput, error) {
	log.Printf("MCP Tool Call: CancelGttOrder gtt_id=%s", input.GttID)
	payload := map[string]any{
		"gtt_id":        input.GttID,
		"exchange_code": input.ExchangeCode,
	}

	res, err := h.client.CallAPI(ctx, "DELETE", "https://api.icicidirect.com/breezeapi/api/v1/gttorder", payload, true)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to cancel GTT order: %v", err)}},
		}, CancelGttOrderOutput{}, nil
	}

	successData, _ := res["Success"].(map[string]any)
	return nil, CancelGttOrderOutput{Result: successData}, nil
}

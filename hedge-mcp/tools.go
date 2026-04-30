package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolHandler struct {
	client *APIClient
}

// --- Market Data Tool ---

type GetPricesInput struct {
	Symbol     string `json:"symbol" jsonschema:"The stock or crypto symbol (e.g., AAPL, BTC/USD)"`
	Resolution string `json:"resolution" jsonschema:"Candle resolution (e.g., 1, 5, 15, 60, D, W, M)"`
}

type GetPricesOutput struct {
	Prices []PricePoint `json:"prices"`
}

func (h *ToolHandler) GetPrices(ctx context.Context, req *mcp.CallToolRequest, input GetPricesInput) (*mcp.CallToolResult, GetPricesOutput, error) {
	log.Printf("MCP Tool: GetPrices symbol=%s", input.Symbol)
	prices, err := h.client.FetchFinnHubPrices(input.Symbol, input.Resolution)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
		}, GetPricesOutput{}, nil
	}
	return nil, GetPricesOutput{Prices: prices}, nil
}

// --- Quantitative Analysis Tool ---

type CalculateIndicatorsInput struct {
	Symbol string       `json:"symbol" jsonschema:"The symbol for the data"`
	Data   []PricePoint `json:"data" jsonschema:"List of price points to analyze"`
}

type CalculateIndicatorsOutput struct {
	RSI            float64 `json:"rsi"`
	MACD           float64 `json:"macd"`
	Signal         float64 `json:"macd_signal"`
	Histogram      float64 `json:"macd_histogram"`
	BollingerUpper float64 `json:"bb_upper"`
	BollingerLower float64 `json:"bb_lower"`
	EMA50          float64 `json:"ema_50"`
	EMA200         float64 `json:"ema_200"`
	ATR            float64 `json:"atr"`
}

type PricePoint struct {
	Timestamp int64   `json:"t"`
	Open      float64 `json:"o"`
	High      float64 `json:"h"`
	Low       float64 `json:"l"`
	Close     float64 `json:"c"`
	Volume    float64 `json:"v"`
}

func (h *ToolHandler) CalculateIndicators(ctx context.Context, req *mcp.CallToolRequest, input CalculateIndicatorsInput) (*mcp.CallToolResult, CalculateIndicatorsOutput, error) {
	if len(input.Data) == 0 {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: "No data provided for analysis"}},
		}, CalculateIndicatorsOutput{}, nil
	}

	closes := make([]float64, len(input.Data))
	for i, p := range input.Data {
		closes[i] = p.Close
	}

	macd, signal, hist := MACD(closes)
	upper, lower := BollingerBands(closes, 20, 2.0)
	ema50 := EMA(closes, 50)
	ema200 := EMA(closes, 200)

	output := CalculateIndicatorsOutput{
		RSI:            RSI(closes, 14),
		MACD:           macd,
		Signal:         signal,
		Histogram:      hist,
		BollingerUpper: upper,
		BollingerLower: lower,
		EMA50:          ema50[len(ema50)-1],
		EMA200:         ema200[len(ema200)-1],
		ATR:            ATR(input.Data, 14),
	}

	return nil, output, nil
}

// --- Fundamental Analysis Tool ---

type GetFinancialsInput struct {
	Symbol string `json:"symbol" jsonschema:"The stock symbol"`
}

type GetFinancialsOutput struct {
	Data map[string]any `json:"data"`
}

func (h *ToolHandler) GetFinancials(ctx context.Context, req *mcp.CallToolRequest, input GetFinancialsInput) (*mcp.CallToolResult, GetFinancialsOutput, error) {
	data, err := h.client.FetchAlphaVantageFinancials(input.Symbol)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
		}, GetFinancialsOutput{}, nil
	}
	return nil, GetFinancialsOutput{Data: data}, nil
}

// --- Sentiment Analysis Tool ---

type GetNewsInput struct {
	Symbol string `json:"symbol" jsonschema:"The stock symbol"`
}

type GetNewsOutput struct {
	News []map[string]any `json:"news"`
}

func (h *ToolHandler) GetNews(ctx context.Context, req *mcp.CallToolRequest, input GetNewsInput) (*mcp.CallToolResult, GetNewsOutput, error) {
	news, err := h.client.FetchFinnHubNews(input.Symbol)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
		}, GetNewsOutput{}, nil
	}
	return nil, GetNewsOutput{News: news}, nil
}

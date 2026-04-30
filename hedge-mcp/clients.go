package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type APIClient struct {
	FinnHubKey      string
	AlphaVantageKey string
	HTTPClient      *http.Client
	Simulation      bool
	SyntheticData   *SyntheticData
}

type SyntheticData struct {
	Prices     map[string][]PricePoint   `json:"prices"`
	Financials map[string]map[string]any `json:"financials"`
	News       map[string][]map[string]any `json:"news"`
}

func NewAPIClient() *APIClient {
	return &APIClient{
		FinnHubKey:      os.Getenv("FINNHUB_API_KEY"),
		AlphaVantageKey: os.Getenv("ALPHAVANTAGE_API_KEY"),
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *APIClient) LoadSyntheticData(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var data SyntheticData
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return err
	}
	c.SyntheticData = &data
	c.Simulation = true
	return nil
}

// FinnHub Candle response
type FinnHubCandles struct {
	C []float64 `json:"c"`
	H []float64 `json:"h"`
	L []float64 `json:"l"`
	O []float64 `json:"o"`
	S string    `json:"s"`
	T []int64   `json:"t"`
	V []float64 `json:"v"`
}

func (c *APIClient) FetchFinnHubPrices(symbol, resolution string) ([]PricePoint, error) {
	if c.Simulation && c.SyntheticData != nil {
		if prices, ok := c.SyntheticData.Prices[symbol]; ok {
			return prices, nil
		}
		return nil, fmt.Errorf("no synthetic price data for symbol %s", symbol)
	}

	if c.FinnHubKey == "" {
		return nil, fmt.Errorf("FINNHUB_API_KEY not set")
	}

	end := time.Now().Unix()
	start := end - (60 * 60 * 24 * 30) // Default to last 30 days

	url := fmt.Sprintf("https://finnhub.io/api/v1/stock/candle?symbol=%s&resolution=%s&from=%d&to=%d&token=%s",
		symbol, resolution, start, end, c.FinnHubKey)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data FinnHubCandles
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if data.S != "ok" {
		return nil, fmt.Errorf("finnhub api error: %s", data.S)
	}

	prices := make([]PricePoint, len(data.T))
	for i := range data.T {
		prices[i] = PricePoint{
			Timestamp: data.T[i],
			Open:      data.O[i],
			High:      data.H[i],
			Low:       data.L[i],
			Close:     data.C[i],
			Volume:    data.V[i],
		}
	}

	return prices, nil
}

func (c *APIClient) FetchAlphaVantageFinancials(symbol string) (map[string]any, error) {
	if c.Simulation && c.SyntheticData != nil {
		if financials, ok := c.SyntheticData.Financials[symbol]; ok {
			return financials, nil
		}
		return nil, fmt.Errorf("no synthetic financial data for symbol %s", symbol)
	}

	if c.AlphaVantageKey == "" {
		return nil, fmt.Errorf("ALPHAVANTAGE_API_KEY not set")
	}

	url := fmt.Sprintf("https://www.alphavantage.co/query?function=OVERVIEW&symbol=%s&apikey=%s",
		symbol, c.AlphaVantageKey)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return data, nil
}

func (c *APIClient) FetchFinnHubNews(symbol string) ([]map[string]any, error) {
	if c.Simulation && c.SyntheticData != nil {
		if news, ok := c.SyntheticData.News[symbol]; ok {
			return news, nil
		}
		return nil, fmt.Errorf("no synthetic news data for symbol %s", symbol)
	}

	if c.FinnHubKey == "" {
		return nil, fmt.Errorf("FINNHUB_API_KEY not set")
	}

	to := time.Now().Format("2006-01-02")
	from := time.Now().AddDate(0, 0, -7).Format("2006-01-02")

	url := fmt.Sprintf("https://finnhub.io/api/v1/company-news?symbol=%s&from=%s&to=%s&token=%s",
		symbol, from, to, c.FinnHubKey)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return data, nil
}

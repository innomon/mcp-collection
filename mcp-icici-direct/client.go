package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

// BreezeClient handles authenticating and calling Breeze REST endpoints.
type BreezeClient struct {
	AppKey       string
	SecretKey    string
	SessionToken string // Session token returned from CustomerDetails or set via login
	APIKeyToken  string // The long-lived session_token exchanged from CustomerDetails API
	Simulate     bool
	DataFile     string

	// Mock Storage for simulation
	mu        sync.RWMutex
	mockData  *SyntheticData
	httpClient *http.Client
}

// SyntheticData represents the schema of synthetic_data.json
type SyntheticData struct {
	CustomerDetails    map[string]any   `json:"customer_details"`
	DematHoldings      []map[string]any `json:"demat_holdings"`
	Funds              map[string]any   `json:"funds"`
	Margins            map[string]any   `json:"margins"`
	Quotes             map[string]any   `json:"quotes"`
	PortfolioHoldings  []map[string]any `json:"portfolio_holdings"`
	PortfolioPositions []map[string]any `json:"portfolio_positions"`
	Orders             []map[string]any `json:"orders"`
	Trades             []map[string]any `json:"trades"`
	GttOrders          []map[string]any `json:"gtt_orders"`
	HistoricalCharts   map[string]any   `json:"historical_charts"`
}

// NewBreezeClient constructs a new BreezeClient
func NewBreezeClient(appKey, secretKey, sessionToken string, simulate bool, dataFile string) *BreezeClient {
	if dataFile == "" {
		dataFile = "synthetic_data.json"
	}
	return &BreezeClient{
		AppKey:       appKey,
		SecretKey:    secretKey,
		SessionToken: sessionToken,
		Simulate:     simulate,
		DataFile:     dataFile,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// LoadSyntheticData parses mock data for simulation mode.
func (c *BreezeClient) LoadSyntheticData(filename string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("read synthetic file error: %w", err)
	}

	var sd SyntheticData
	if err := json.Unmarshal(data, &sd); err != nil {
		return fmt.Errorf("unmarshal synthetic data error: %w", err)
	}

	c.mockData = &sd
	c.DataFile = filename
	return nil
}

// SaveSyntheticData serializes simulation data back to the file system (to persist order edits/placements).
func (c *BreezeClient) SaveSyntheticData() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.mockData == nil {
		return nil
	}

	data, err := json.MarshalIndent(c.mockData, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mock data error: %w", err)
	}

	if err := os.WriteFile(c.DataFile, data, 0644); err != nil {
		return fmt.Errorf("write mock file error: %w", err)
	}
	return nil
}

// GetISO8601Timestamp returns ISO8601 UTC DateTime Format with 0 milliseconds
func GetISO8601Timestamp() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05") + ".000Z"
}

// ComputeChecksum generates SHA256(Timestamp + JSONPayloadString + SecretKey)
func ComputeChecksum(timestamp, payload, secretKey string) string {
	hasher := sha256.New()
	hasher.Write([]byte(timestamp + payload + secretKey))
	return hex.EncodeToString(hasher.Sum(nil))
}

// CallAPI conducts an authenticated HTTP call or intercepts it for simulation.
func (c *BreezeClient) CallAPI(ctx context.Context, method, endpointURL string, payload map[string]any, requiresAuthHeaders bool) (map[string]any, error) {
	if c.Simulate {
		return c.mockCall(ctx, method, endpointURL, payload)
	}

	// Prepare payload bytes
	var bodyBytes []byte
	var err error
	if payload == nil {
		payload = make(map[string]any)
	}

	// Breeze requires compact separators without extra spacing to match exact signature hashes.
	bodyBytes, err = json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpointURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if requiresAuthHeaders {
		timestamp := GetISO8601Timestamp()
		checksum := ComputeChecksum(timestamp, string(bodyBytes), c.SecretKey)

		req.Header.Set("X-Timestamp", timestamp)
		req.Header.Set("X-Checksum", "token "+checksum)
		req.Header.Set("X-AppKey", c.AppKey)
		req.Header.Set("X-SessionToken", c.APIKeyToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("api error status %d: %s", resp.StatusCode, string(respBytes))
	}

	var result map[string]any
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result, nil
}

// ExchangedSessionToken authenticates with the CustomerDetails API to get the long-lived APIKeyToken (session_token).
func (c *BreezeClient) Authenticate(ctx context.Context) error {
	if c.Simulate {
		c.mu.RLock()
		defer c.mu.RUnlock()
		if c.mockData != nil && c.mockData.CustomerDetails != nil {
			if token, ok := c.mockData.CustomerDetails["exchanged_token"].(string); ok {
				c.APIKeyToken = token
				log.Printf("Breeze (Simulation): successfully authenticated session_token=%s", c.APIKeyToken)
				return nil
			}
		}
		c.APIKeyToken = "mock_session_token_123"
		return nil
	}

	if c.SessionToken == "" {
		return fmt.Errorf("SessionToken is empty. Please log in at https://api.icicidirect.com/apiuser/login?api_key=%s and set the BREEZE_SESSION_TOKEN environment variable", c.AppKey)
	}

	log.Printf("Breeze Client: Authenticating using AppKey=%s and SessionToken=%s...", c.AppKey, c.SessionToken)
	payload := map[string]any{
		"SessionToken": c.SessionToken,
		"AppKey":       c.AppKey,
	}

	// CustomerDetails does NOT require checksum signature headers, only body parameters.
	res, err := c.CallAPI(ctx, "GET", "https://api.icicidirect.com/breezeapi/api/v1/customerdetails", payload, false)
	if err != nil {
		return fmt.Errorf("customerdetails authentication request failed: %w", err)
	}

	successMap, ok := res["Success"].(map[string]any)
	if !ok {
		if errorMsg, ok := res["Status"].(string); ok {
			return fmt.Errorf("authentication rejected by server: status=%s, msg=%v", errorMsg, res["Error"])
		}
		return fmt.Errorf("invalid authentication response structure: %v", res)
	}

	token, ok := successMap["session_token"].(string)
	if !ok || token == "" {
		return fmt.Errorf("session_token missing in customer details response: %v", successMap)
	}

	c.APIKeyToken = token
	log.Printf("Breeze Client: successfully exchanged SessionToken for session_token: %s", token)
	return nil
}

// mockCall intercepts API calls in simulation mode and serves mock responses
func (c *BreezeClient) mockCall(ctx context.Context, method, endpointURL string, payload map[string]any) (map[string]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.mockData == nil {
		return nil, fmt.Errorf("simulation mode active but mock data not loaded")
	}

	u, err := url.Parse(endpointURL)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint url: %w", err)
	}

	path := u.Path
	log.Printf("Breeze (Simulation): intercepting %s %s", method, path)

	switch {
	case path == "/breezeapi/api/v1/customerdetails":
		return map[string]any{
			"Success": map[string]any{
				"session_token": c.mockData.CustomerDetails["exchanged_token"],
				"client_id":     c.mockData.CustomerDetails["client_id"],
				"user_name":     c.mockData.CustomerDetails["user_name"],
			},
		}, nil

	case path == "/breezeapi/api/v1/dematholdings":
		return map[string]any{
			"Success": c.mockData.DematHoldings,
			"Status":  200,
		}, nil

	case path == "/breezeapi/api/v1/funds":
		if method == "GET" {
			return map[string]any{
				"Success": c.mockData.Funds,
				"Status":  200,
			}, nil
		} else if method == "POST" {
			// Allocate funds
			allocation, _ := payload["allocation_amount"].(string)
			segment, _ := payload["segment"].(string)
			action, _ := payload["action"].(string) // "add" or "reduce"

			log.Printf("Breeze (Simulation): modified funds segment=%s amount=%s action=%s", segment, allocation, action)
			return map[string]any{
				"Success": map[string]any{
					"message": fmt.Sprintf("funds allocation of %s for segment %s successful", allocation, segment),
				},
				"Status": 200,
			}, nil
		}

	case path == "/breezeapi/api/v1/margins":
		return map[string]any{
			"Success": c.mockData.Margins,
			"Status":  200,
		}, nil

	case path == "/breezeapi/api/v1/quotes":
		symbol, _ := payload["stock_code"].(string)
		quote, exists := c.mockData.Quotes[symbol]
		if !exists {
			return map[string]any{
				"Error":  fmt.Sprintf("symbol %s not found in mock database", symbol),
				"Status": 404,
			}, nil
		}
		return map[string]any{
			"Success": []any{quote},
			"Status":  200,
		}, nil

	case path == "/api/v2/historicalcharts" || path == "/breezeapi/api/v1/historicalcharts":
		symbol, _ := payload["stock_code"].(string)
		exch, _ := payload["exchange_code"].(string)
		key := fmt.Sprintf("%s_%s", symbol, exch)
		chart, exists := c.mockData.HistoricalCharts[key]
		if !exists {
			// fallback
			chart = c.mockData.HistoricalCharts["ITC_NSE"]
		}
		return map[string]any{
			"Success": chart,
			"Status":  200,
		}, nil

	case path == "/breezeapi/api/v1/portfolioholdings":
		return map[string]any{
			"Success": c.mockData.PortfolioHoldings,
			"Status":  200,
		}, nil

	case path == "/breezeapi/api/v1/portfoliopositions":
		return map[string]any{
			"Success": c.mockData.PortfolioPositions,
			"Status":  200,
		}, nil

	case path == "/breezeapi/api/v1/order":
		if method == "GET" {
			// Get order list or status
			orderID, _ := payload["order_id"].(string)
			if orderID != "" {
				for _, o := range c.mockData.Orders {
					if o["order_id"] == orderID {
						return map[string]any{
							"Success": []any{o},
							"Status":  200,
						}, nil
					}
				}
				return map[string]any{
					"Error":  "order not found",
					"Status": 404,
				}, nil
			}
			return map[string]any{
				"Success": c.mockData.Orders,
				"Status":  200,
			}, nil

		} else if method == "POST" {
			// Order placement
			stock, _ := payload["stock_code"].(string)
			exch, _ := payload["exchange_code"].(string)
			product, _ := payload["product"].(string)
			action, _ := payload["action"].(string)
			orderType, _ := payload["order_type"].(string)
			qty, _ := payload["quantity"].(string)
			price, _ := payload["price"].(string)
			validity, _ := payload["validity"].(string)

			newOrderID := fmt.Sprintf("MOCK_ORD_%d", time.Now().UnixNano()%1000000)
			newOrder := map[string]any{
				"order_id":      newOrderID,
				"stock_code":    stock,
				"exchange_code": exch,
				"product":       product,
				"action":        action,
				"order_type":    orderType,
				"quantity":      qty,
				"price":         price,
				"status":        "executed", // execute mock order instantly
				"validity":      validity,
				"order_date":    time.Now().Format("2006-01-02 15:04:05"),
			}

			c.mockData.Orders = append(c.mockData.Orders, newOrder)
			go c.SaveSyntheticData()

			return map[string]any{
				"Success": map[string]any{
					"message":  "Order Placed Successfully",
					"order_id": newOrderID,
				},
				"Status": 200,
			}, nil

		} else if method == "PUT" {
			// Order modification
			orderID, _ := payload["order_id"].(string)
			price, _ := payload["price"].(string)
			qty, _ := payload["quantity"].(string)

			for i, o := range c.mockData.Orders {
				if o["order_id"] == orderID {
					c.mockData.Orders[i]["price"] = price
					c.mockData.Orders[i]["quantity"] = qty
					c.mockData.Orders[i]["status"] = "modified"
					go c.SaveSyntheticData()
					return map[string]any{
						"Success": map[string]any{
							"message": "Order Modified Successfully",
						},
						"Status": 200,
					}, nil
				}
			}
			return map[string]any{
				"Error":  "order not found to modify",
				"Status": 404,
			}, nil

		} else if method == "DELETE" {
			// Order cancellation
			orderID, _ := payload["order_id"].(string)
			for i, o := range c.mockData.Orders {
				if o["order_id"] == orderID {
					c.mockData.Orders[i]["status"] = "cancelled"
					go c.SaveSyntheticData()
					return map[string]any{
						"Success": map[string]any{
							"message": "Order Cancelled Successfully",
						},
						"Status": 200,
					}, nil
				}
			}
			return map[string]any{
				"Error":  "order not found to cancel",
				"Status": 404,
			}, nil
		}

	case path == "/breezeapi/api/v1/squareoff":
		stock, _ := payload["stock_code"].(string)
		qty, _ := payload["quantity"].(string)
		return map[string]any{
			"Success": map[string]any{
				"message": fmt.Sprintf("Intraday positions of %s for quantity %s squared off successfully", stock, qty),
			},
			"Status": 200,
		}, nil

	case path == "/breezeapi/api/v1/gttorder":
		if method == "POST" {
			stock, _ := payload["stock_code"].(string)
			exch, _ := payload["exchange_code"].(string)
			triggerPrice, _ := payload["trigger_price"].(string)
			qty, _ := payload["quantity"].(string)

			gttID := fmt.Sprintf("MOCK_GTT_%d", time.Now().UnixNano()%1000000)
			newGtt := map[string]any{
				"gtt_id":        gttID,
				"stock_code":    stock,
				"exchange_code": exch,
				"product_type":  "cash",
				"action":        "buy",
				"trigger_price": triggerPrice,
				"limit_price":   triggerPrice,
				"quantity":      qty,
				"status":        "active",
				"validity_date": time.Now().AddDate(1, 0, 0).Format(time.RFC3339),
			}

			c.mockData.GttOrders = append(c.mockData.GttOrders, newGtt)
			go c.SaveSyntheticData()

			return map[string]any{
				"Success": map[string]any{
					"message": "GTT Order Placed Successfully",
					"gtt_id":  gttID,
				},
				"Status": 200,
			}, nil
		} else if method == "GET" {
			return map[string]any{
				"Success": c.mockData.GttOrders,
				"Status":  200,
			}, nil
		} else if method == "DELETE" {
			gttID, _ := payload["gtt_id"].(string)
			for i, g := range c.mockData.GttOrders {
				if g["gtt_id"] == gttID {
					c.mockData.GttOrders[i]["status"] = "cancelled"
					go c.SaveSyntheticData()
					return map[string]any{
						"Success": map[string]any{
							"message": "GTT Order Cancelled Successfully",
						},
						"Status": 200,
					}, nil
				}
			}
			return map[string]any{
				"Error":  "GTT order not found to cancel",
				"Status": 404,
			}, nil
		}
	}

	return map[string]any{
		"Error":  fmt.Sprintf("Mock endpoint not implemented for %s %s", method, path),
		"Status": 501,
	}, nil
}

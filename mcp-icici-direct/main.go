package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	// 1. Handcrafted CLI parameters (No Cobra / spf13 allowed)
	simulate := flag.Bool("simulate", false, "Run in simulation mode with dummy data")
	dataFile := flag.String("data", "synthetic_data.json", "Path to synthetic data file for simulation")
	flag.Parse()

	log.Println("Initializing mcp-icici-direct server...")

	// 2. Load settings from environment variables
	appKey := os.Getenv("BREEZE_APP_KEY")
	secretKey := os.Getenv("BREEZE_SECRET_KEY")
	sessionToken := os.Getenv("BREEZE_SESSION_TOKEN") // User's initial login token

	if !*simulate {
		if appKey == "" || secretKey == "" {
			fmt.Fprintln(os.Stderr, "Error: BREEZE_APP_KEY and BREEZE_SECRET_KEY environment variables must be set in live mode.")
			fmt.Fprintln(os.Stderr, "Use the '-simulate' flag to run the server in sandbox simulation mode without credentials.")
			os.Exit(1)
		}
	}

	// 3. Construct and authenticate Client
	client := NewBreezeClient(appKey, secretKey, sessionToken, *simulate, *dataFile)
	if *simulate {
		if err := client.LoadSyntheticData(*dataFile); err != nil {
			log.Fatalf("Failed to load mock simulation data: %v", err)
		}
		log.Printf("Breeze Server: running in SIMULATION mode using %s", *dataFile)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Authenticate session keys before serving tools
	if err := client.Authenticate(ctx); err != nil {
		if !*simulate {
			log.Fatalf("Authentication failed: %v", err)
		} else {
			log.Printf("Warning: Simulation authentication failed: %v", err)
		}
	}

	// 4. Initialize MCP Server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "mcp-icici-direct",
		Version: "0.1.0",
	}, nil)

	handler := &ToolHandler{
		client: client,
	}

	// 5. Register all Breeze API tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_demat_holdings",
		Description: "Fetch detailed shares and allocated quantity list from the user's Demat account.",
	}, handler.GetDematHoldings)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_funds",
		Description: "Retrieve segregated balance limits details allocated for Equity and F&O segments.",
	}, handler.GetFunds)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_funds",
		Description: "Dynamically allocate or reduce limit funds for a segment (e.g. Equity, F&O).",
	}, handler.SetFunds)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_margins",
		Description: "Query active limits, utilized limits, and blocked margin balances.",
	}, handler.GetMargins)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_quotes",
		Description: "Fetch real-time L1 quote data including LTP, bid-ask spreads, and daily volumes for a given stock code on NSE.",
	}, handler.GetQuotes)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_historical_charts",
		Description: "Fetch historical OHLCV chart candles for a stock or index at specified intervals (1min, 5min, 30min, 1day).",
	}, handler.GetHistoricalCharts)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_portfolio_holdings",
		Description: "Retrieve the list of active long-term investment stock holdings in the user's portfolio.",
	}, handler.GetPortfolioHoldings)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_portfolio_positions",
		Description: "List currently open or closed intraday / derivative positions.",
	}, handler.GetPortfolioPositions)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "place_order",
		Description: "Place a new Limit or Stop-Loss order for NSE cash or F&O derivative segments. Market orders are prohibited.",
	}, handler.PlaceOrder)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_order_details",
		Description: "Fetch transaction execution status and log history for a specific order ID.",
	}, handler.GetOrderDetails)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_order_list",
		Description: "Retrieve the full daily order book (active, pending, and closed orders).",
	}, handler.GetOrderList)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "modify_order",
		Description: "Amend price or quantity limits of a pending order in the exchange books.",
	}, handler.ModifyOrder)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "cancel_order",
		Description: "Cancel a pending order before it gets filled.",
	}, handler.CancelOrder)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "square_off",
		Description: "Instantly cover and liquidate open intraday or derivative trading positions.",
	}, handler.SquareOff)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "place_gtt_order",
		Description: "Place a long-standing Good-Till-Triggered (GTT) order valid for up to 365 days.",
	}, handler.PlaceGttOrder)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_gtt_order_book",
		Description: "Fetch active and triggered GTT orders.",
	}, handler.GetGttOrderBook)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "cancel_gtt_order",
		Description: "Cancel a pending active GTT order.",
	}, handler.CancelGttOrder)

	// 6. Start Stdio or SSE transport based on environment switch
	transport := os.Getenv("ICICI_MCP_TRANSPORT")
	if transport == "" {
		transport = "stdio"
	}

	log.Printf("mcp-icici-direct: starting transport=%s", transport)

	switch transport {
	case "stdio":
		if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
			log.Fatalf("Stdio server failed: %v", err)
		}
	case "sse":
		if err := runSSEServer(ctx, server); err != nil {
			log.Fatalf("SSE server failed: %v", err)
		}
	default:
		log.Fatalf("Unsupported transport: %s", transport)
	}
}

func runSSEServer(ctx context.Context, server *mcp.Server) error {
	host := os.Getenv("ICICI_MCP_SSE_HOST")
	if host == "" {
		host = "0.0.0.0"
	}
	port := os.Getenv("ICICI_MCP_SSE_PORT")
	if port == "" {
		port = "8086"
	}
	path := os.Getenv("ICICI_MCP_SSE_PATH")
	if path == "" {
		path = "/mcp"
	}

	handler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server { return server }, nil)
	mux := http.NewServeMux()
	mux.Handle(path, handler)

	address := net.JoinHostPort(host, port)
	httpServer := &http.Server{
		Addr:    address,
		Handler: mux,
	}

	errChan := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	log.Printf("mcp-icici-direct SSE listening on http://%s%s", address, path)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-errChan:
		return err
	}
}

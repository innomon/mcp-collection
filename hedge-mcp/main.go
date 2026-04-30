package main

import (
	"context"
	"flag"
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
	simulate := flag.Bool("simulate", false, "Run in simulation mode with dummy data")
	dataFile := flag.String("data", "synthetic_data.json", "Path to synthetic data file")
	flag.Parse()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "hedge-mcp",
		Version: "0.1.0",
	}, nil)

	client := NewAPIClient()
	if *simulate {
		if err := client.LoadSyntheticData(*dataFile); err != nil {
			log.Fatalf("failed to load synthetic data: %v", err)
		}
		log.Printf("running in SIMULATION mode using %s", *dataFile)
	}

	handler := &ToolHandler{
		client: client,
	}

	// Register Market Data tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_prices",
		Description: "Fetch historical price data (OHLCV) for a given symbol and resolution.",
	}, handler.GetPrices)

	// Register Quant tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "calculate_indicators",
		Description: "Calculate technical indicators (RSI, MACD, BB, EMA, ATR) from price data.",
	}, handler.CalculateIndicators)

	// Register Fundamental tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_financials",
		Description: "Fetch key fundamental data and financial ratios for a given stock symbol.",
	}, handler.GetFinancials)

	// Register Sentiment tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_news",
		Description: "Fetch recent news headlines and summaries for a given stock symbol.",
	}, handler.GetNews)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	transport := os.Getenv("HEDGE_MCP_TRANSPORT")
	if transport == "" {
		transport = "stdio"
	}

	log.Printf("hedge-mcp starting transport=%s", transport)

	switch transport {
	case "stdio":
		if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
			log.Fatalf("server failed: %v", err)
		}
	case "sse":
		if err := runSSEServer(ctx, server); err != nil {
			log.Fatalf("server failed: %v", err)
		}
	default:
		log.Fatalf("unsupported transport: %s", transport)
	}
}

func runSSEServer(ctx context.Context, server *mcp.Server) error {
	host := os.Getenv("HEDGE_MCP_SSE_HOST")
	if host == "" {
		host = "0.0.0.0"
	}
	port := os.Getenv("HEDGE_MCP_SSE_PORT")
	if port == "" {
		port = "8082"
	}
	path := os.Getenv("HEDGE_MCP_SSE_PATH")
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

	log.Printf("hedge-mcp SSE listening on http://%s%s", address, path)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-errChan:
		return err
	}
}

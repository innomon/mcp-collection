package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	cfg, err := loadConfigFromEnv()
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	client := &FrappeClient{
		Config: cfg,
		HTTPClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "frappe-mcp",
		Version: "1.1.0",
	}, nil)
	registerTools(server, client, cfg)
	registerResources(server, client, cfg)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Printf("frappe-mcp starting transport=%s doctype_gen_enabled=%t delete_enabled=%t a2ui_pipeline_enabled=%t", cfg.Transport, cfg.EnableDocTypeGen, cfg.EnableDelete, cfg.EnableA2UIPipeline)

	if err := runServer(ctx, server, cfg); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func runServer(ctx context.Context, server *mcp.Server, cfg Config) error {
	switch cfg.Transport {
	case "stdio":
		return server.Run(ctx, &mcp.StdioTransport{})
	case "sse":
		return runSSEServer(ctx, server, cfg)
	default:
		return fmt.Errorf("unsupported FRAPPE_MCP_TRANSPORT value %q", cfg.Transport)
	}
}

func runSSEServer(ctx context.Context, server *mcp.Server, cfg Config) error {
	handler := mcp.NewSSEHandler(func(_ *http.Request) *mcp.Server { return server }, nil)
	mux := http.NewServeMux()
	mux.Handle(cfg.SSEPath, handler)

	address := net.JoinHostPort(cfg.SSEHost, cfg.SSEPort)
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

	log.Printf("frappe-mcp SSE listening on http://%s%s", address, cfg.SSEPath)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-errChan:
		return err
	}
}

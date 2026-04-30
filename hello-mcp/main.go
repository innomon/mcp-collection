package main

import (
	"context"
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

type HelloInput struct {
	Name string `json:"name" jsonschema:"The name to say hello to"`
}

type HelloOutput struct {
	Message string `json:"message"`
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "hello-mcp",
		Version: "0.1.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "hello",
		Description: "Say hello to someone",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input HelloInput) (*mcp.CallToolResult, HelloOutput, error) {
		name := input.Name
		if name == "" {
			name = "World"
		}
		greeting := fmt.Sprintf("Hello, %s!", name)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: greeting},
			},
		}, HelloOutput{Message: greeting}, nil
	})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	transport := os.Getenv("HELLO_MCP_TRANSPORT")
	if transport == "" {
		transport = "stdio"
	}

	log.Printf("hello-mcp starting transport=%s", transport)

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
	host := os.Getenv("HELLO_MCP_SSE_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := os.Getenv("HELLO_MCP_SSE_PORT")
	if port == "" {
		port = "8083"
	}
	path := os.Getenv("HELLO_MCP_SSE_PATH")
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

	log.Printf("hello-mcp SSE listening on http://%s%s", address, path)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-errChan:
		return err
	}
}

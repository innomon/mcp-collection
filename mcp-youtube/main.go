package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ServerName    = "mcp-youtube"
	ServerVersion = "1.0.0"
)

func main() {
	// Disable log timestamp in stdio mode to avoid breaking MCP protocol JSON output
	log.SetFlags(0)
	log.SetOutput(os.Stderr)

	log.Printf("Starting %s %s...", ServerName, ServerVersion)

	// 1. Load config and parse CLI flags
	cfg, err := LoadConfig(os.Args[1:])
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// 2. Initialize YouTube client wrapper (live or simulation)
	client, err := NewYouTubeClient(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize YouTube client: %v", err)
	}

	// 3. Initialize MCP Server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    ServerName,
		Version: ServerVersion,
	}, nil)

	// 4. Register all tools
	RegisterTools(server, client)

	// 5. Connect to stdio transport
	transport, err := server.Connect(ctx, &mcp.StdioTransport{}, nil)
	if err != nil {
		log.Fatalf("Failed to establish stdio transport: %v", err)
	}
	defer transport.Close()

	log.Printf("%s %s successfully running over stdio.", ServerName, ServerVersion)

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutting down YouTube MCP server...")
}

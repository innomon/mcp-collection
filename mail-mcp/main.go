package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	cfg, err := LoadConfig(os.Args[1:])
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	s := mcp.NewServer(&mcp.Implementation{
		Name:    cfg.Server.Name,
		Version: cfg.Server.Version,
	}, nil)

	RegisterTools(s, cfg)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Connect to stdio transport
	ss, err := s.Connect(ctx, &mcp.StdioTransport{}, nil)
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
	defer ss.Close()

	log.Printf("%s %s started", cfg.Server.Name, cfg.Server.Version)

	<-ctx.Done()
	log.Println("server shut down")
}

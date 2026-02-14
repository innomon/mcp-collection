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

	if cfg.PublicKeyPath != "" {
		if _, err := LoadPublicKey(cfg.PublicKeyPath); err != nil {
			log.Fatalf("failed to load public key: %v", err)
		}
	}

	s := mcp.NewServer(&mcp.Implementation{
		Name:    cfg.ServerName,
		Version: cfg.ServerVersion,
	}, nil)

	wc := NewWooClient(cfg.StoreURL, cfg.ConsumerKey, cfg.ConsumerSecret)
	RegisterTools(s, wc, cfg.StoreURL)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	ss, err := s.Connect(ctx, &mcp.StdioTransport{}, nil)
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
	}

	log.Printf("%s %s started", cfg.ServerName, cfg.ServerVersion)

	<-ctx.Done()
	ss.Close()
	log.Println("server shut down")
}

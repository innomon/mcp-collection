package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
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
	RegisterTools(s, wc, cfg)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if cfg.Transport == "http" || cfg.Transport == "both" {
		httpMux := http.NewServeMux()

		var oauth *OAuthServer
		if len(cfg.OAuthClients) > 0 {
			oauth = NewOAuthServer(cfg, cfg.OAuthClients, wc)
		}

		rest := NewRESTServer(wc, cfg, oauth)
		rest.RegisterRoutes(httpMux)

		httpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
			return s
		}, nil)
		httpMux.Handle("/ucp/mcp", httpHandler)

		addr := fmt.Sprintf(":%d", cfg.HTTPPort)
		httpServer := &http.Server{Addr: addr, Handler: httpMux}

		go func() {
			log.Printf("HTTP server listening on %s", addr)
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTP server error: %v", err)
			}
		}()

		defer httpServer.Close()
	}

	if cfg.Transport == "stdio" || cfg.Transport == "both" {
		ss, err := s.Connect(ctx, &mcp.StdioTransport{}, nil)
		if err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
		defer ss.Close()
	}

	log.Printf("%s %s started (transport=%s)", cfg.ServerName, cfg.ServerVersion, cfg.Transport)

	<-ctx.Done()
	log.Println("server shut down")
}

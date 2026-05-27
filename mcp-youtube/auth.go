package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Global lock to prevent concurrent OAuth server starts
var oauthMu sync.Mutex

func getOauthConfig(cfg *Config) *oauth2.Config {
	redirectURL := fmt.Sprintf("http://localhost:%d/oauth2/callback", cfg.OAuthPort)
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/youtube.readonly",
			"https://www.googleapis.com/auth/yt-analytics.readonly",
			"https://www.googleapis.com/auth/yt-analytics-monetization.readonly",
		},
		Endpoint: google.Endpoint,
	}
}

func loadToken(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) error {
	// Create directory if not exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("opening token file: %w", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

func getOAuthClient(ctx context.Context, cfg *Config) (*http.Client, error) {
	oauthConfig := getOauthConfig(cfg)

	// Try loading cached token
	tok, err := loadToken(cfg.TokenCachePath)
	if err == nil && tok != nil {
		tokenSource := oauthConfig.TokenSource(ctx, tok)
		savingTokenSource := &savingTokenSource{
			source: tokenSource,
			path:   cfg.TokenCachePath,
		}
		return oauth2.NewClient(ctx, savingTokenSource), nil
	}

	// No valid cached token, need to authenticate
	oauthMu.Lock()
	defer oauthMu.Unlock()

	// Double-check after lock
	tok, err = loadToken(cfg.TokenCachePath)
	if err == nil && tok != nil {
		tokenSource := oauthConfig.TokenSource(ctx, tok)
		savingTokenSource := &savingTokenSource{
			source: tokenSource,
			path:   cfg.TokenCachePath,
		}
		return oauth2.NewClient(ctx, savingTokenSource), nil
	}

	log.Printf("No cached OAuth token found. Initiating Google OAuth 2.0 flow...")

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("generating state: %w", err)
	}
	state := hex.EncodeToString(stateBytes)

	// Generate Auth URL
	authURL := oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	// Print link to Stderr clearly so client / agent logs it
	fmt.Fprintf(os.Stderr, "\n==================================================\n")
	fmt.Fprintf(os.Stderr, "YOUTUBE MCP AUTHENTICATION REQUIRED\n")
	fmt.Fprintf(os.Stderr, "Please open the following URL in your browser to authenticate:\n\n%s\n\n", authURL)
	fmt.Fprintf(os.Stderr, "==================================================\n\n")

	// Start local HTTP server to receive redirect
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.OAuthPort),
		Handler: mux,
	}

	mux.HandleFunc("/oauth2/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		retState := q.Get("state")
		if retState != state {
			http.Error(w, "State mismatch", http.StatusBadRequest)
			errChan <- fmt.Errorf("state mismatch: expected %s, got %s", state, retState)
			return
		}

		code := q.Get("code")
		if code == "" {
			http.Error(w, "Code missing", http.StatusBadRequest)
			errChan <- fmt.Errorf("oauth callback was missing the authorization code")
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
			<html>
			<body style="font-family: Arial, sans-serif; text-align: center; padding-top: 50px; background-color: #f9f9f9;">
				<div style="display: inline-block; padding: 30px; border-radius: 8px; background: white; box-shadow: 0 4px 6px rgba(0,0,0,0.1);">
					<h1 style="color: #FF0000; margin-bottom: 10px;">YouTube MCP Server</h1>
					<p style="color: #333; font-size: 16px;">Authentication successful! You can close this window now.</p>
				</div>
			</body>
			</html>
		`)

		codeChan <- code
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("callback server: %w", err)
		}
	}()

	// Wait for code, error or timeout (2 minutes)
	var code string
	select {
	case code = <-codeChan:
		// Success!
	case err := <-errChan:
		server.Shutdown(context.Background())
		return nil, fmt.Errorf("authentication error: %w", err)
	case <-time.After(2 * time.Minute):
		server.Shutdown(context.Background())
		return nil, fmt.Errorf("authentication timed out after 2 minutes")
	case <-ctx.Done():
		server.Shutdown(context.Background())
		return nil, ctx.Err()
	}

	// Graceful shutdown of callback server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	server.Shutdown(shutdownCtx)

	// Exchange authorization code for token
	token, err := oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchanging code: %w", err)
	}

	// Cache the token
	if err := saveToken(cfg.TokenCachePath, token); err != nil {
		log.Printf("Warning: failed to save token to cache: %v", err)
	} else {
		log.Printf("Successfully cached OAuth token to %s", cfg.TokenCachePath)
	}

	tokenSource := oauthConfig.TokenSource(ctx, token)
	savingTokenSource := &savingTokenSource{
		source: tokenSource,
		path:   cfg.TokenCachePath,
	}

	return oauth2.NewClient(ctx, savingTokenSource), nil
}

// savingTokenSource wraps an oauth2.TokenSource to write refreshed tokens to disk automatically.
type savingTokenSource struct {
	source oauth2.TokenSource
	path   string
	mu     sync.Mutex
	last   *oauth2.Token
}

func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tok, err := s.source.Token()
	if err != nil {
		return nil, err
	}

	// If the token changed (e.g. was refreshed), save it.
	if s.last == nil || s.last.AccessToken != tok.AccessToken {
		s.last = tok
		if err := saveToken(s.path, tok); err != nil {
			log.Printf("Warning: failed to save refreshed token: %v", err)
		} else {
			log.Printf("Saved refreshed OAuth token to %s", s.path)
		}
	}

	return tok, nil
}

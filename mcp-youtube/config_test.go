package main

import (
	"os"
	"testing"
)

func TestConfigDefaults(t *testing.T) {
	// Set required env vars to prevent exit in live mode
	os.Setenv("YOUTUBE_OAUTH_CLIENT_ID", "dummy_client")
	os.Setenv("YOUTUBE_OAUTH_CLIENT_SECRET", "dummy_secret")
	defer func() {
		os.Unsetenv("YOUTUBE_OAUTH_CLIENT_ID")
		os.Unsetenv("YOUTUBE_OAUTH_CLIENT_SECRET")
	}()

	cfg, err := LoadConfig([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Simulate {
		t.Errorf("expected Simulate to be false by default")
	}

	if cfg.OAuthPort != 6050 {
		t.Errorf("expected default OAuthPort to be 6050, got %d", cfg.OAuthPort)
	}

	if cfg.DataFile != "synthetic_data.json" {
		t.Errorf("expected default DataFile to be synthetic_data.json, got %q", cfg.DataFile)
	}
}

func TestConfigEnvOverrides(t *testing.T) {
	os.Setenv("YOUTUBE_OAUTH_PORT", "9999")
	os.Setenv("YOUTUBE_TOKEN_CACHE_PATH", "/tmp/token.json")
	os.Setenv("YOUTUBE_OAUTH_CLIENT_ID", "dummy_client")
	os.Setenv("YOUTUBE_OAUTH_CLIENT_SECRET", "dummy_secret")

	defer func() {
		os.Unsetenv("YOUTUBE_OAUTH_PORT")
		os.Unsetenv("YOUTUBE_TOKEN_CACHE_PATH")
		os.Unsetenv("YOUTUBE_OAUTH_CLIENT_ID")
		os.Unsetenv("YOUTUBE_OAUTH_CLIENT_SECRET")
	}()

	cfg, err := LoadConfig([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.OAuthPort != 9999 {
		t.Errorf("expected OAuthPort to be 9999, got %d", cfg.OAuthPort)
	}

	if cfg.TokenCachePath != "/tmp/token.json" {
		t.Errorf("expected TokenCachePath to be /tmp/token.json, got %q", cfg.TokenCachePath)
	}
}

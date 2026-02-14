package main

import (
	"os"
	"path/filepath"
	"testing"
)

const testYAML = `store_url: "https://example.com"
consumer_key: "ck_test123"
consumer_secret: "cs_test456"
public_key_path: "/path/to/key.pem"
server_name: "TestServer"
server_version: "0.1.0"
`

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfigFromArgs(t *testing.T) {
	path := writeTestConfig(t, testYAML)

	cfg, err := LoadConfig([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.StoreURL != "https://example.com" {
		t.Errorf("StoreURL = %q, want %q", cfg.StoreURL, "https://example.com")
	}
	if cfg.ConsumerKey != "ck_test123" {
		t.Errorf("ConsumerKey = %q, want %q", cfg.ConsumerKey, "ck_test123")
	}
	if cfg.ConsumerSecret != "cs_test456" {
		t.Errorf("ConsumerSecret = %q, want %q", cfg.ConsumerSecret, "cs_test456")
	}
	if cfg.PublicKeyPath != "/path/to/key.pem" {
		t.Errorf("PublicKeyPath = %q, want %q", cfg.PublicKeyPath, "/path/to/key.pem")
	}
	if cfg.ServerName != "TestServer" {
		t.Errorf("ServerName = %q, want %q", cfg.ServerName, "TestServer")
	}
	if cfg.ServerVersion != "0.1.0" {
		t.Errorf("ServerVersion = %q, want %q", cfg.ServerVersion, "0.1.0")
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	path := writeTestConfig(t, testYAML)
	t.Setenv("CONFIG_FILE", path)

	cfg, err := LoadConfig(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.StoreURL != "https://example.com" {
		t.Errorf("StoreURL = %q, want %q", cfg.StoreURL, "https://example.com")
	}
	if cfg.ConsumerKey != "ck_test123" {
		t.Errorf("ConsumerKey = %q, want %q", cfg.ConsumerKey, "ck_test123")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	path := writeTestConfig(t, `store_url: "https://example.com"`)

	cfg, err := LoadConfig([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ServerName != "WooCommerce-MCP" {
		t.Errorf("ServerName = %q, want %q", cfg.ServerName, "WooCommerce-MCP")
	}
	if cfg.ServerVersion != "1.0.0" {
		t.Errorf("ServerVersion = %q, want %q", cfg.ServerVersion, "1.0.0")
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := LoadConfig([]string{"/nonexistent/config.yaml"})
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	path := writeTestConfig(t, `{{{invalid yaml:::`)

	_, err := LoadConfig([]string{path})
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadConfigNoFileFound(t *testing.T) {
	t.Setenv("CONFIG_FILE", "")

	_, err := LoadConfig(nil)
	if err == nil {
		t.Fatal("expected error when no config file found, got nil")
	}
}

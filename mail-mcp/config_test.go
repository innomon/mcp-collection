package main

import (
	"os"
	"path/filepath"
	"testing"
)

const testYAML = `server:
  name: "test-mail-mcp"
  version: "0.1.0"
accounts:
  - id: "primary"
    email: "user@example.com"
    imap:
      host: "imap.example.com"
      port: 993
      tls: true
    smtp:
      host: "smtp.example.com"
      port: 587
      starttls: true
    auth:
      type: "app_password"
      user: "user@example.com"
      password: "secret-password"
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

func TestLoadConfig(t *testing.T) {
	path := writeTestConfig(t, testYAML)

	cfg, err := LoadConfig([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Name != "test-mail-mcp" {
		t.Errorf("Server.Name = %q, want %q", cfg.Server.Name, "test-mail-mcp")
	}
	if len(cfg.Accounts) != 1 {
		t.Fatalf("len(Accounts) = %d, want 1", len(cfg.Accounts))
	}
	acc := cfg.Accounts[0]
	if acc.ID != "primary" {
		t.Errorf("Account.ID = %q, want %q", acc.ID, "primary")
	}
	if acc.IMAP.Host != "imap.example.com" {
		t.Errorf("Account.IMAP.Host = %q, want %q", acc.IMAP.Host, "imap.example.com")
	}
	if acc.Auth.Password != "secret-password" {
		t.Errorf("Account.Auth.Password = %q, want %q", acc.Auth.Password, "secret-password")
	}
}

func TestGetAccount(t *testing.T) {
	path := writeTestConfig(t, testYAML)
	cfg, _ := LoadConfig([]string{path})

	// Test default account
	acc, err := cfg.GetAccount("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acc.ID != "primary" {
		t.Errorf("default Account.ID = %q, want %q", acc.ID, "primary")
	}

	// Test specific account
	acc, err = cfg.GetAccount("primary")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acc.ID != "primary" {
		t.Errorf("primary Account.ID = %q, want %q", acc.ID, "primary")
	}

	// Test non-existent account
	_, err = cfg.GetAccount("missing")
	if err == nil {
		t.Fatal("expected error for missing account, got nil")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	path := writeTestConfig(t, `server: {}`)

	cfg, err := LoadConfig([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Name != "mail-mcp" {
		t.Errorf("ServerName = %q, want %q", cfg.Server.Name, "mail-mcp")
	}
	if cfg.Server.Version != "1.0.0" {
		t.Errorf("ServerVersion = %q, want %q", cfg.Server.Version, "1.0.0")
	}
}

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	StoreURL       string `yaml:"store_url"`
	ConsumerKey    string `yaml:"consumer_key"`
	ConsumerSecret string `yaml:"consumer_secret"`
	PublicKeyPath  string `yaml:"public_key_path"`
	ServerName           string `yaml:"server_name"`
	ServerVersion        string `yaml:"server_version"`
	Transport            string `yaml:"transport"`               // "stdio" | "http" | "both"
	HTTPPort             int    `yaml:"http_port"`               // port for HTTP transport
	UCPEnabled           bool   `yaml:"ucp_enabled"`             // enable UCP profile + REST endpoints
	A2UIEnabled          bool   `yaml:"a2ui_enabled"`            // enable A2UI card generation
	StorePoliciesPageIDs []int  `yaml:"store_policies_page_ids"` // WordPress page IDs for policies
}

func LoadConfig(args []string) (*Config, error) {
	path, err := resolveConfigPath(args)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if cfg.ServerName == "" {
		cfg.ServerName = "WooCommerce-MCP"
	}
	if cfg.ServerVersion == "" {
		cfg.ServerVersion = "1.0.0"
	}
	if cfg.Transport == "" {
		cfg.Transport = "stdio"
	}
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 8080
	}

	return &cfg, nil
}

func resolveConfigPath(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	if envPath := os.Getenv("CONFIG_FILE"); envPath != "" {
		return envPath, nil
	}

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("determining executable path: %w", err)
	}
	binDir := filepath.Dir(exe)
	candidate := filepath.Join(binDir, "config.yaml")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}

	return "", fmt.Errorf("no config file found: provide a path as argument, set CONFIG_FILE, or place config.yaml next to the binary")
}

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig  `yaml:"server"`
	Accounts []AccountConfig `yaml:"accounts"`
}

type ServerConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type AccountConfig struct {
	ID    string     `yaml:"id"`
	Email string     `yaml:"email"`
	IMAP  IMAPConfig `yaml:"imap"`
	SMTP  SMTPConfig `yaml:"smtp"`
	Auth  AuthConfig `yaml:"auth"`
}

type IMAPConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	TLS  bool   `yaml:"tls"`
}

type SMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	StartTLS bool   `yaml:"starttls"`
}

type AuthConfig struct {
	Type     string `yaml:"type"` // "app_password" | "oauth2"
	User     string `yaml:"user"`
	Password string `yaml:"password"`
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

	if cfg.Server.Name == "" {
		cfg.Server.Name = "mail-mcp"
	}
	if cfg.Server.Version == "" {
		cfg.Server.Version = "1.0.0"
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

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Simulate       bool
	DataFile       string
	ClientID       string
	ClientSecret   string
	OAuthPort      int
	TokenCachePath string
}

func LoadConfig(args []string) (*Config, error) {
	// Handcrafted CLI parameters using standard flag package (No spf13 Cobra/Pflag allowed)
	fs := flag.NewFlagSet("mcp-youtube", flag.ContinueOnError)
	
	simulate := fs.Bool("simulate", false, "Run in simulation mode with dummy data")
	dataFile := fs.String("data", "synthetic_data.json", "Path to synthetic data file for simulation")
	portFlag := fs.Int("port", 0, "OAuth callback server port (overrides env YOUTUBE_OAUTH_PORT)")
	
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	// 2. Load from Env
	clientID := os.Getenv("YOUTUBE_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("YOUTUBE_OAUTH_CLIENT_SECRET")

	// Determine Port (portFlag > env > fallback to 6050)
	port := 6050
	if *portFlag > 0 {
		port = *portFlag
	} else if envPort := os.Getenv("YOUTUBE_OAUTH_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil && p > 0 {
			port = p
		}
	}

	// Determine cache path
	cachePath := os.Getenv("YOUTUBE_TOKEN_CACHE_PATH")
	if cachePath == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			cachePath = filepath.Join(home, ".config", "mcp-youtube", "token.json")
		} else {
			cachePath = "token.json" // fallback to current dir
		}
	}

	// Validate live configuration
	if !*simulate {
		if clientID == "" || clientSecret == "" {
			return nil, fmt.Errorf("YOUTUBE_OAUTH_CLIENT_ID and YOUTUBE_OAUTH_CLIENT_SECRET environment variables must be set in live mode.\nTo run without credentials in dry-run mode, please use the '-simulate' flag.")
		}
	}

	return &Config{
		Simulate:       *simulate,
		DataFile:       *dataFile,
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		OAuthPort:      port,
		TokenCachePath: cachePath,
	}, nil
}

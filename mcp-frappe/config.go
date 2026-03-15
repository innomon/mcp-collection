package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	BaseURL             string
	APIKey              string
	APISecret           string
	Timeout             time.Duration
	Transport           string
	SSEHost             string
	SSEPort             string
	SSEPath             string
	EnableDelete        bool
	EnableDocTypeGen    bool
	AllowedDocTypes     map[string]struct{}
	Environment         string
	DeleteApprovalToken string
	EnableA2UIPipeline  bool
	ConfidenceThreshold float64
	MaxCandidates       int
	FallbackSchema      string
	CacheTTLSec         int
}

func loadConfigFromEnv() (Config, error) {
	baseURL := strings.TrimSpace(os.Getenv("FRAPPE_BASE_URL"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("FRAPPE_URL"))
	}

	timeoutMS := parseIntWithDefault(os.Getenv("FRAPPE_TIMEOUT_MS"), 15000)
	transport := strings.ToLower(strings.TrimSpace(defaultValue(os.Getenv("FRAPPE_MCP_TRANSPORT"), "stdio")))

	ssePath := strings.TrimSpace(defaultValue(os.Getenv("FRAPPE_MCP_SSE_PATH"), "/sse"))
	if !strings.HasPrefix(ssePath, "/") {
		ssePath = "/" + ssePath
	}

	cfg := Config{
		BaseURL:             baseURL,
		APIKey:              strings.TrimSpace(os.Getenv("FRAPPE_API_KEY")),
		APISecret:           strings.TrimSpace(os.Getenv("FRAPPE_API_SECRET")),
		Timeout:             time.Duration(timeoutMS) * time.Millisecond,
		Transport:           transport,
		SSEHost:             strings.TrimSpace(os.Getenv("FRAPPE_MCP_SSE_HOST")),
		SSEPort:             strings.TrimSpace(os.Getenv("FRAPPE_MCP_SSE_PORT")),
		SSEPath:             ssePath,
		EnableDelete:        parseBool(os.Getenv("FRAPPE_MCP_ENABLE_DELETE")),
		EnableDocTypeGen:    parseBool(os.Getenv("FRAPPE_MCP_ENABLE_DOCTYPE_GEN")),
		AllowedDocTypes:     parseAllowlist(os.Getenv("FRAPPE_ALLOWED_DOCTYPES")),
		Environment:         strings.ToLower(strings.TrimSpace(defaultValue(os.Getenv("FRAPPE_ENV"), os.Getenv("APP_ENV")))),
		DeleteApprovalToken: strings.TrimSpace(os.Getenv("FRAPPE_MCP_DELETE_APPROVAL_TOKEN")),
		EnableA2UIPipeline:  parseBool(os.Getenv("FRAPPE_MCP_ENABLE_A2UI_PIPELINE")),
		ConfidenceThreshold: parseFloatWithDefault(os.Getenv("FRAPPE_MCP_CONFIDENCE_THRESHOLD"), 0.5),
		MaxCandidates:       parseIntWithDefault(os.Getenv("FRAPPE_MCP_MAX_CANDIDATES"), 20),
		FallbackSchema:      defaultValue(os.Getenv("FRAPPE_MCP_FALLBACK_SCHEMA"), "markdown"),
		CacheTTLSec:         parseIntWithDefault(os.Getenv("FRAPPE_MCP_CACHE_TTL_SEC"), 300),
	}

	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.APISecret == "" {
		return Config{}, fmt.Errorf("FRAPPE_BASE_URL (or FRAPPE_URL), FRAPPE_API_KEY, and FRAPPE_API_SECRET are required")
	}

	if cfg.Transport != "stdio" && cfg.Transport != "sse" {
		return Config{}, fmt.Errorf("FRAPPE_MCP_TRANSPORT must be either 'stdio' or 'sse'")
	}

	if cfg.Transport == "sse" {
		if cfg.SSEHost == "" || cfg.SSEPort == "" {
			return Config{}, fmt.Errorf("FRAPPE_MCP_SSE_HOST and FRAPPE_MCP_SSE_PORT are required when FRAPPE_MCP_TRANSPORT=sse")
		}
	}

	return cfg, nil
}

func parseBool(raw string) bool {
	v, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return v
}

func parseFloatWithDefault(raw string, def float64) float64 {
	v := strings.TrimSpace(raw)
	if v == "" {
		return def
	}
	parsed, err := strconv.ParseFloat(v, 64)
	if err != nil || parsed < 0 {
		return def
	}
	return parsed
}

func parseIntWithDefault(raw string, defaultValue int) int {
	v := strings.TrimSpace(raw)
	if v == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(v)
	if err != nil || parsed <= 0 {
		return defaultValue
	}
	return parsed
}

func parseAllowlist(raw string) map[string]struct{} {
	allowed := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		allowed[strings.ToLower(name)] = struct{}{}
	}
	return allowed
}

func isDocTypeAllowed(allowlist map[string]struct{}, docType string) bool {
	if len(allowlist) == 0 {
		return true
	}
	_, ok := allowlist[strings.ToLower(strings.TrimSpace(docType))]
	return ok
}

func defaultValue(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}

func isProductionEnvironment(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}

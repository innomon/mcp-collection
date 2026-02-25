package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Configuration & Client ---

const doctypeContractVersion = "0.1.0"

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
}

type FrappeClient struct {
	Config     Config
	HTTPClient *http.Client
}

func (c *FrappeClient) Do(ctx context.Context, method, path string, body []byte) (string, error) {
	resourceURL := fmt.Sprintf("%s/api/resource/%s", strings.TrimRight(c.Config.BaseURL, "/"), path)
	req, err := http.NewRequestWithContext(ctx, method, resourceURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", c.Config.APIKey, c.Config.APISecret))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("frappe error (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return string(respBody), nil
}

// --- Tool Argument Structs ---
// The official SDK uses these tags to generate the tool's JSON schema automatically.

type ToolResponse struct {
	Data any `json:"data"`
}

type DeleteResponse struct {
	DocType string `json:"doctype"`
	Name    string `json:"name"`
	Deleted bool   `json:"deleted"`
	DryRun  bool   `json:"dry_run"`
}

type DeleteRecordArgs struct {
	DocType       string `json:"doctype" jsonschema:"description=The Frappe DocType"`
	Name          string `json:"name" jsonschema:"description=The unique record name"`
	DryRun        bool   `json:"dry_run" jsonschema:"description=If true, validates policy and returns delete intent without deleting"`
	ApprovalToken string `json:"approval_token,omitempty" jsonschema:"description=Required for live delete in production"`
}

type DocTypeMetaArgs struct {
	DocType string `json:"doctype" jsonschema:"description=The Frappe DocType to fetch metadata for"`
}

type GenerateDocTypeJSONArgs struct {
	DocType        string         `json:"doctype" jsonschema:"description=The Frappe DocType to normalize"`
	SourceMetadata map[string]any `json:"source_metadata,omitempty" jsonschema:"description=Optional raw metadata payload. If omitted, metadata is fetched from Frappe"`
}

type ValidateDocTypeJSONArgs struct {
	DocTypeJSON map[string]any `json:"doctype_json" jsonschema:"description=DocType JSON payload generated from the canonical contract"`
}

type DocTypeContractEnvelope struct {
	ContractVersion string               `json:"contract_version"`
	DocType         CanonicalDocTypeJSON `json:"doctype"`
}

type CanonicalDocTypeJSON struct {
	Name          string               `json:"name"`
	Module        string               `json:"module"`
	IsSubmittable bool                 `json:"is_submittable"`
	Modified      string               `json:"modified"`
	Fields        []CanonicalFieldJSON `json:"fields"`
	Permissions   []PermissionJSON     `json:"permissions"`
}

type CanonicalFieldJSON struct {
	FieldName string  `json:"fieldname"`
	Label     string  `json:"label"`
	FieldType string  `json:"fieldtype"`
	Reqd      bool    `json:"reqd"`
	Options   *string `json:"options"`
}

type PermissionJSON struct {
	Role   string `json:"role"`
	Read   bool   `json:"read"`
	Write  bool   `json:"write"`
	Create bool   `json:"create"`
	Delete bool   `json:"delete"`
}

type ValidationViolation struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ValidationResult struct {
	ContractVersion string                `json:"contract_version"`
	Valid           bool                  `json:"valid"`
	Violations      []ValidationViolation `json:"violations"`
}

type SearchArgs struct {
	DocType string `json:"doctype" jsonschema:"description=The Frappe DocType to search (e.g. Customer)"`
	Filters string `json:"filters" jsonschema:"description=JSON list of filters, e.g. [['name', 'like', '%John%']]"`
	Fields  string `json:"fields" jsonschema:"description=JSON list of fields to return, e.g. ['name', 'email_id']"`
}

type GetRecordArgs struct {
	DocType string `json:"doctype" jsonschema:"description=The Frappe DocType"`
	Name    string `json:"name" jsonschema:"description=The unique record name"`
}

type CreateRecordArgs struct {
	DocType string         `json:"doctype" jsonschema:"description=The Frappe DocType"`
	DocJSON map[string]any `json:"doc_json" jsonschema:"description=The document fields to create"`
}

type UpdateRecordArgs struct {
	DocType    string         `json:"doctype" jsonschema:"description=The Frappe DocType"`
	Name       string         `json:"name" jsonschema:"description=The unique record name"`
	UpdateJSON map[string]any `json:"update_json" jsonschema:"description=The fields to update"`
}

func main() {
	cfg, err := loadConfigFromEnv()
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	client := &FrappeClient{
		Config: cfg,
		HTTPClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "frappe-mcp",
		Version: "1.1.0",
	}, nil)
	registerTools(server, client, cfg)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Printf("frappe-mcp starting transport=%s doctype_gen_enabled=%t delete_enabled=%t", cfg.Transport, cfg.EnableDocTypeGen, cfg.EnableDelete)

	if err := runServer(ctx, server, cfg); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func runServer(ctx context.Context, server *mcp.Server, cfg Config) error {
	switch cfg.Transport {
	case "stdio":
		return server.Run(ctx, &mcp.StdioTransport{})
	case "sse":
		return runSSEServer(ctx, server, cfg)
	default:
		return fmt.Errorf("unsupported FRAPPE_MCP_TRANSPORT value %q", cfg.Transport)
	}
}

func runSSEServer(ctx context.Context, server *mcp.Server, cfg Config) error {
	handler := mcp.NewSSEHandler(func(_ *http.Request) *mcp.Server { return server }, nil)
	mux := http.NewServeMux()
	mux.Handle(cfg.SSEPath, handler)

	address := net.JoinHostPort(cfg.SSEHost, cfg.SSEPort)
	httpServer := &http.Server{
		Addr:    address,
		Handler: mux,
	}

	errChan := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	log.Printf("frappe-mcp SSE listening on http://%s%s", address, cfg.SSEPath)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-errChan:
		return err
	}
}

func registerTools(server *mcp.Server, client *FrappeClient, cfg Config) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "frappe_search",
		Description: "Search Frappe records with URL-safe filters and fields",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args SearchArgs) (*mcp.CallToolResult, ToolResponse, error) {
		docType, ok := validateName(args.DocType)
		if !ok {
			return toolError("invalid_argument", "doctype is required"), ToolResponse{}, nil
		}

		params := url.Values{}
		if strings.TrimSpace(args.Filters) != "" {
			params.Set("filters", args.Filters)
		}
		if strings.TrimSpace(args.Fields) != "" {
			params.Set("fields", args.Fields)
		}

		path := escapePathSegment(docType)
		if encoded := params.Encode(); encoded != "" {
			path = path + "?" + encoded
		}

		res, err := client.Do(ctx, http.MethodGet, path, nil)
		if err != nil {
			return toolError("frappe_request_failed", err.Error()), ToolResponse{}, nil
		}
		return nil, ToolResponse{Data: decodeJSONOrString(res)}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "frappe_get_record",
		Description: "Get a specific record's full details",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GetRecordArgs) (*mcp.CallToolResult, ToolResponse, error) {
		docType, ok := validateName(args.DocType)
		if !ok {
			return toolError("invalid_argument", "doctype is required"), ToolResponse{}, nil
		}
		name, ok := validateName(args.Name)
		if !ok {
			return toolError("invalid_argument", "name is required"), ToolResponse{}, nil
		}

		path := fmt.Sprintf("%s/%s", escapePathSegment(docType), escapePathSegment(name))
		res, err := client.Do(ctx, http.MethodGet, path, nil)
		if err != nil {
			return toolError("frappe_request_failed", err.Error()), ToolResponse{}, nil
		}
		return nil, ToolResponse{Data: decodeJSONOrString(res)}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "frappe_create_record",
		Description: "Create a new record in Frappe",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args CreateRecordArgs) (*mcp.CallToolResult, ToolResponse, error) {
		docType, ok := validateName(args.DocType)
		if !ok {
			return toolError("invalid_argument", "doctype is required"), ToolResponse{}, nil
		}
		body, err := json.Marshal(args.DocJSON)
		if err != nil {
			return toolError("invalid_argument", fmt.Sprintf("invalid doc_json: %v", err)), ToolResponse{}, nil
		}

		res, err := client.Do(ctx, http.MethodPost, escapePathSegment(docType), body)
		if err != nil {
			return toolError("frappe_request_failed", err.Error()), ToolResponse{}, nil
		}
		return nil, ToolResponse{Data: decodeJSONOrString(res)}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "frappe_update_record",
		Description: "Update fields on an existing record",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args UpdateRecordArgs) (*mcp.CallToolResult, ToolResponse, error) {
		docType, ok := validateName(args.DocType)
		if !ok {
			return toolError("invalid_argument", "doctype is required"), ToolResponse{}, nil
		}
		name, ok := validateName(args.Name)
		if !ok {
			return toolError("invalid_argument", "name is required"), ToolResponse{}, nil
		}
		body, err := json.Marshal(args.UpdateJSON)
		if err != nil {
			return toolError("invalid_argument", fmt.Sprintf("invalid update_json: %v", err)), ToolResponse{}, nil
		}

		path := fmt.Sprintf("%s/%s", escapePathSegment(docType), escapePathSegment(name))
		res, err := client.Do(ctx, http.MethodPut, path, body)
		if err != nil {
			return toolError("frappe_request_failed", err.Error()), ToolResponse{}, nil
		}
		return nil, ToolResponse{Data: decodeJSONOrString(res)}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "frappe_delete_record",
		Description: "Delete an existing record from Frappe with policy checks",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DeleteRecordArgs) (*mcp.CallToolResult, DeleteResponse, error) {
		if !cfg.EnableDelete {
			return toolError("delete_disabled", "delete tool is disabled; set FRAPPE_MCP_ENABLE_DELETE=true"), DeleteResponse{}, nil
		}

		docType, ok := validateName(args.DocType)
		if !ok {
			return toolError("invalid_argument", "doctype is required"), DeleteResponse{}, nil
		}
		name, ok := validateName(args.Name)
		if !ok {
			return toolError("invalid_argument", "name is required"), DeleteResponse{}, nil
		}
		if !isDocTypeAllowed(cfg.AllowedDocTypes, docType) {
			return toolError("doctype_not_allowed", "doctype is not allowed by FRAPPE_ALLOWED_DOCTYPES"), DeleteResponse{}, nil
		}

		if args.DryRun {
			return nil, DeleteResponse{DocType: docType, Name: name, Deleted: false, DryRun: true}, nil
		}

		if isProductionEnvironment(cfg.Environment) {
			if cfg.DeleteApprovalToken == "" {
				return toolError("delete_policy_violation", "production delete requires FRAPPE_MCP_DELETE_APPROVAL_TOKEN config"), DeleteResponse{}, nil
			}
			if args.ApprovalToken == "" || args.ApprovalToken != cfg.DeleteApprovalToken {
				return toolError("delete_approval_required", "production delete requires a valid approval_token"), DeleteResponse{}, nil
			}
		}

		path := fmt.Sprintf("%s/%s", escapePathSegment(docType), escapePathSegment(name))
		if _, err := client.Do(ctx, http.MethodDelete, path, nil); err != nil {
			return toolError("frappe_request_failed", err.Error()), DeleteResponse{}, nil
		}

		return nil, DeleteResponse{DocType: docType, Name: name, Deleted: true, DryRun: false}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "frappe_get_doctype_meta",
		Description: "Fetch canonical DocType metadata from Frappe",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DocTypeMetaArgs) (*mcp.CallToolResult, ToolResponse, error) {
		if !cfg.EnableDocTypeGen {
			return toolError("doctype_generation_disabled", "doctype generation tools are disabled; set FRAPPE_MCP_ENABLE_DOCTYPE_GEN=true"), ToolResponse{}, nil
		}
		docType, ok := validateName(args.DocType)
		if !ok {
			return toolError("invalid_argument", "doctype is required"), ToolResponse{}, nil
		}
		if !isDocTypeAllowed(cfg.AllowedDocTypes, docType) {
			return toolError("doctype_not_allowed", "doctype is not allowed by FRAPPE_ALLOWED_DOCTYPES"), ToolResponse{}, nil
		}

		meta, err := getDocTypeMeta(ctx, client, docType)
		if err != nil {
			return toolError("frappe_request_failed", err.Error()), ToolResponse{}, nil
		}
		return nil, ToolResponse{Data: meta}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "frappe_generate_doctype_json",
		Description: "Generate canonical DocType JSON for contract version 0.1.0",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GenerateDocTypeJSONArgs) (*mcp.CallToolResult, DocTypeContractEnvelope, error) {
		if !cfg.EnableDocTypeGen {
			return toolError("doctype_generation_disabled", "doctype generation tools are disabled; set FRAPPE_MCP_ENABLE_DOCTYPE_GEN=true"), DocTypeContractEnvelope{}, nil
		}

		docType, ok := validateName(args.DocType)
		if !ok {
			return toolError("invalid_argument", "doctype is required"), DocTypeContractEnvelope{}, nil
		}
		if !isDocTypeAllowed(cfg.AllowedDocTypes, docType) {
			return toolError("doctype_not_allowed", "doctype is not allowed by FRAPPE_ALLOWED_DOCTYPES"), DocTypeContractEnvelope{}, nil
		}

		rawMeta := args.SourceMetadata
		if len(rawMeta) == 0 {
			var err error
			rawMeta, err = getDocTypeMeta(ctx, client, docType)
			if err != nil {
				return toolError("frappe_request_failed", err.Error()), DocTypeContractEnvelope{}, nil
			}
		}

		normalized, err := normalizeDocTypeMeta(docType, rawMeta)
		if err != nil {
			return toolError("normalize_failed", err.Error()), DocTypeContractEnvelope{}, nil
		}

		return nil, normalized, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "frappe_validate_doctype_json",
		Description: "Validate generated DocType JSON against contract version 0.1.0",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args ValidateDocTypeJSONArgs) (*mcp.CallToolResult, ValidationResult, error) {
		if !cfg.EnableDocTypeGen {
			return toolError("doctype_generation_disabled", "doctype generation tools are disabled; set FRAPPE_MCP_ENABLE_DOCTYPE_GEN=true"), ValidationResult{}, nil
		}
		return nil, validateDocTypeJSON(args.DocTypeJSON), nil
	})
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

func getDocTypeMeta(ctx context.Context, client *FrappeClient, docType string) (map[string]any, error) {
	path := fmt.Sprintf("DocType/%s", escapePathSegment(docType))
	res, err := client.Do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	decoded := decodeJSONOrString(res)
	meta, ok := decoded.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected metadata shape from Frappe")
	}
	return meta, nil
}

func normalizeDocTypeMeta(docType string, source map[string]any) (DocTypeContractEnvelope, error) {
	docNode := source
	if nested, ok := source["data"].(map[string]any); ok {
		docNode = nested
	}

	name := asString(docNode["name"])
	if name == "" {
		name = docType
	}

	fields := make([]CanonicalFieldJSON, 0)
	if rawFields, ok := docNode["fields"].([]any); ok {
		for idx, raw := range rawFields {
			entry, ok := raw.(map[string]any)
			if !ok {
				return DocTypeContractEnvelope{}, fmt.Errorf("field index %d is not an object", idx)
			}
			field := CanonicalFieldJSON{
				FieldName: asString(entry["fieldname"]),
				Label:     asString(entry["label"]),
				FieldType: asString(entry["fieldtype"]),
				Reqd:      asBool(entry["reqd"]),
			}
			options := strings.TrimSpace(asString(entry["options"]))
			if options != "" {
				field.Options = &options
			}
			fields = append(fields, field)
		}
	}

	permissions := make([]PermissionJSON, 0)
	if rawPerms, ok := docNode["permissions"].([]any); ok {
		for idx, raw := range rawPerms {
			entry, ok := raw.(map[string]any)
			if !ok {
				return DocTypeContractEnvelope{}, fmt.Errorf("permission index %d is not an object", idx)
			}
			permissions = append(permissions, PermissionJSON{
				Role:   asString(entry["role"]),
				Read:   asBool(entry["read"]),
				Write:  asBool(entry["write"]),
				Create: asBool(entry["create"]),
				Delete: asBool(entry["delete"]),
			})
		}
	}

	return DocTypeContractEnvelope{
		ContractVersion: doctypeContractVersion,
		DocType: CanonicalDocTypeJSON{
			Name:          name,
			Module:        asString(docNode["module"]),
			IsSubmittable: asBool(docNode["is_submittable"]),
			Modified:      asString(docNode["modified"]),
			Fields:        fields,
			Permissions:   permissions,
		},
	}, nil
}

func validateDocTypeJSON(payload map[string]any) ValidationResult {
	result := ValidationResult{
		ContractVersion: doctypeContractVersion,
		Valid:           true,
		Violations:      make([]ValidationViolation, 0),
	}

	addViolation := func(path, code, message string) {
		result.Valid = false
		result.Violations = append(result.Violations, ValidationViolation{Path: path, Code: code, Message: message})
	}

	if asString(payload["contract_version"]) != doctypeContractVersion {
		addViolation("contract_version", "unsupported_contract_version", fmt.Sprintf("expected %s", doctypeContractVersion))
	}

	doc, ok := payload["doctype"].(map[string]any)
	if !ok {
		addViolation("doctype", "missing_or_invalid", "doctype must be an object")
		return result
	}

	if strings.TrimSpace(asString(doc["name"])) == "" {
		addViolation("doctype.name", "required", "doctype.name is required")
	}
	if strings.TrimSpace(asString(doc["module"])) == "" {
		addViolation("doctype.module", "required", "doctype.module is required")
	}

	fields, ok := doc["fields"].([]any)
	if !ok {
		addViolation("doctype.fields", "missing_or_invalid", "doctype.fields must be an array")
	} else {
		for idx, raw := range fields {
			field, ok := raw.(map[string]any)
			if !ok {
				addViolation(fmt.Sprintf("doctype.fields[%d]", idx), "invalid_type", "field entry must be an object")
				continue
			}
			if strings.TrimSpace(asString(field["fieldname"])) == "" {
				addViolation(fmt.Sprintf("doctype.fields[%d].fieldname", idx), "required", "fieldname is required")
			}
			if strings.TrimSpace(asString(field["label"])) == "" {
				addViolation(fmt.Sprintf("doctype.fields[%d].label", idx), "required", "label is required")
			}
			if strings.TrimSpace(asString(field["fieldtype"])) == "" {
				addViolation(fmt.Sprintf("doctype.fields[%d].fieldtype", idx), "required", "fieldtype is required")
			}
			if !isBooleanLike(field["reqd"]) {
				addViolation(fmt.Sprintf("doctype.fields[%d].reqd", idx), "invalid_type", "reqd must be boolean")
			}
			if _, exists := field["options"]; exists {
				if field["options"] != nil {
					if _, ok := field["options"].(string); !ok {
						addViolation(fmt.Sprintf("doctype.fields[%d].options", idx), "invalid_type", "options must be string or null")
					}
				}
			}
		}
	}

	if rawPerms, exists := doc["permissions"]; exists {
		permissions, ok := rawPerms.([]any)
		if !ok {
			addViolation("doctype.permissions", "invalid_type", "permissions must be an array")
		} else {
			for idx, raw := range permissions {
				perm, ok := raw.(map[string]any)
				if !ok {
					addViolation(fmt.Sprintf("doctype.permissions[%d]", idx), "invalid_type", "permission entry must be an object")
					continue
				}
				if strings.TrimSpace(asString(perm["role"])) == "" {
					addViolation(fmt.Sprintf("doctype.permissions[%d].role", idx), "required", "role is required")
				}
				for _, key := range []string{"read", "write", "create", "delete"} {
					if !isBooleanLike(perm[key]) {
						addViolation(fmt.Sprintf("doctype.permissions[%d].%s", idx, key), "invalid_type", fmt.Sprintf("%s must be boolean", key))
					}
				}
			}
		}
	}

	return result
}

func decodeJSONOrString(raw string) any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}
	}

	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		return parsed
	}
	return raw
}

func toolError(code, message string) *mcp.CallToolResult {
	errorEnvelope := map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	}
	payload, _ := json.Marshal(errorEnvelope)
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: string(payload)}},
	}
}

func validateName(value string) (string, bool) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", false
	}
	if !utf8.ValidString(v) {
		return "", false
	}
	return v, true
}

func escapePathSegment(value string) string {
	return url.PathEscape(strings.TrimSpace(value))
}

func asString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func asBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case int:
		return v != 0
	case int64:
		return v != 0
	case string:
		lower := strings.ToLower(strings.TrimSpace(v))
		return lower == "1" || lower == "true" || lower == "yes"
	default:
		return false
	}
}

func isBooleanLike(value any) bool {
	if value == nil {
		return false
	}
	switch value.(type) {
	case bool, float64, int, int64:
		return true
	default:
		return false
	}
}

func parseBool(raw string) bool {
	v, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return v
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

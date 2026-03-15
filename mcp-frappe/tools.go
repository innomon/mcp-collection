package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Tool Argument Structs ---

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

type MapDocTypeToCandidatesArgs struct {
	DocType string `json:"doctype" jsonschema:"description=The Frappe DocType to map to schema candidates"`
}

type SelectSchemaArgs struct {
	DocType          string            `json:"doctype" jsonschema:"description=The Frappe DocType to use for candidate generation"`
	Content          string            `json:"content" jsonschema:"description=The agent response content to select a schema for"`
	StaticCandidates []SchemaCandidate `json:"static_candidates,omitempty" jsonschema:"description=Optional static schema candidates to merge with DocType-derived candidates"`
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

	if !cfg.EnableA2UIPipeline {
		return
	}

	dtCache := NewDocTypeCache(time.Duration(cfg.CacheTTLSec) * time.Second)
	mc := NewMetricsCollector()

	fetchDocType := func(ctx context.Context, docType string) (*FrappeDocType, error) {
		if dt, ok := dtCache.Get(docType); ok {
			mc.IncCounter(metricDocTypeCacheHitTotal, nil)
			return dt, nil
		}
		mc.IncCounter(metricDocTypeCacheMissTotal, nil)
		mc.IncCounter(metricDocTypeFetchTotal, nil)
		meta, err := getDocTypeMeta(ctx, client, docType)
		if err != nil {
			return nil, err
		}
		dt := metaToFrappeDocType(docType, meta)
		dtCache.Set(docType, dt)
		return dt, nil
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "frappe_map_doctype_to_candidates",
		Description: "Fetch DocType metadata and return derived A2UI schema candidates",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args MapDocTypeToCandidatesArgs) (*mcp.CallToolResult, ToolResponse, error) {
		docType, ok := validateName(args.DocType)
		if !ok {
			return toolError("invalid_argument", "doctype is required"), ToolResponse{}, nil
		}
		if !isDocTypeAllowed(cfg.AllowedDocTypes, docType) {
			return toolError("doctype_not_allowed", "doctype is not allowed by FRAPPE_ALLOWED_DOCTYPES"), ToolResponse{}, nil
		}

		dt, err := fetchDocType(ctx, docType)
		if err != nil {
			return toolError("frappe_request_failed", err.Error()), ToolResponse{}, nil
		}

		candidates := DocTypeToCandidates([]FrappeDocType{*dt})
		return nil, ToolResponse{Data: map[string]any{"candidates": candidates}}, nil
	})

	pipelineCfg := PipelineConfig{
		ConfidenceThreshold: cfg.ConfidenceThreshold,
		FallbackSchema:      cfg.FallbackSchema,
		MaxCandidates:       cfg.MaxCandidates,
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "frappe_select_schema",
		Description: "Run the full A2UI schema selection pipeline: fetch DocType, map candidates, merge with static, select best schema",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args SelectSchemaArgs) (*mcp.CallToolResult, ToolResponse, error) {
		docType, ok := validateName(args.DocType)
		if !ok {
			return toolError("invalid_argument", "doctype is required"), ToolResponse{}, nil
		}
		if !isDocTypeAllowed(cfg.AllowedDocTypes, docType) {
			return toolError("doctype_not_allowed", "doctype is not allowed by FRAPPE_ALLOWED_DOCTYPES"), ToolResponse{}, nil
		}

		dt, err := fetchDocType(ctx, docType)
		if err != nil {
			return toolError("frappe_request_failed", err.Error()), ToolResponse{}, nil
		}

		dtCandidates := DocTypeToCandidates([]FrappeDocType{*dt})
		maxC := pipelineCfg.MaxCandidates
		if maxC <= 0 {
			maxC = 20 // Default max candidates
		}
		merged := mergeCandidates(args.StaticCandidates, dtCandidates, maxC)

		selector := &DefaultSelector{}
		result, err := runPipeline(ctx, merged, args.Content, selector, pipelineCfg, mc)
		if err != nil {
			return toolError("pipeline_failed", err.Error()), ToolResponse{}, nil
		}

		return nil, ToolResponse{Data: result}, nil
	})
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

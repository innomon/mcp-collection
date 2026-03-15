package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerResources(server *mcp.Server, client *FrappeClient, cfg Config) {
	// Resource Handler
	handler := func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uri := req.Params.URI
		if uri == "frappe://doctypes" {
			data, err := client.GetDocTypes(ctx)
			if err != nil {
				return nil, err
			}
			return resourceResponse(uri, data)
		}
		if uri == "frappe://modules" {
			data, err := client.GetModules(ctx)
			if err != nil {
				return nil, err
			}
			return resourceResponse(uri, data)
		}
		if uri == "frappe://apps" {
			data, err := client.GetApps(ctx)
			if err != nil {
				return nil, err
			}
			return resourceResponse(uri, data)
		}
		if uri == "frappe://system/info" {
			data, err := client.GetSystemInfo(ctx)
			if err != nil {
				return nil, err
			}
			return resourceResponse(uri, data)
		}

		// Template Matching
		if strings.HasPrefix(uri, "frappe://doctype/") {
			trimmed := strings.TrimPrefix(uri, "frappe://doctype/")
			parts := strings.Split(trimmed, "/")
			if len(parts) == 0 || parts[0] == "" {
				return nil, fmt.Errorf("invalid doctype resource URI")
			}
			docType := parts[0]

			meta, err := getDocTypeMeta(ctx, client, docType)
			if err != nil {
				return nil, err
			}

			if len(parts) == 1 {
				return resourceResponse(uri, meta)
			}

			sub := parts[1]
			if sub == "schema" {
				normalized, err := normalizeDocTypeMeta(docType, meta)
				if err != nil {
					return nil, err
				}
				return resourceResponse(uri, normalized)
			}

			if sub == "ui-candidates" {
				dt := metaToFrappeDocType(docType, meta)
				candidates := DocTypeToCandidates([]FrappeDocType{*dt})
				return resourceResponse(uri, map[string]any{"candidates": candidates})
			}
		}

		return nil, fmt.Errorf("resource not found: %s", uri)
	}

	// Static Resources
	server.AddResource(&mcp.Resource{
		URI:      "frappe://doctypes",
		Name:     "Frappe DocTypes",
		MIMEType: "application/json",
	}, handler)
	server.AddResource(&mcp.Resource{
		URI:      "frappe://modules",
		Name:     "Frappe Modules",
		MIMEType: "application/json",
	}, handler)
	server.AddResource(&mcp.Resource{
		URI:      "frappe://apps",
		Name:     "Frappe Apps",
		MIMEType: "application/json",
	}, handler)
	server.AddResource(&mcp.Resource{
		URI:      "frappe://system/info",
		Name:     "Frappe System Info",
		MIMEType: "application/json",
	}, handler)

	// Resource Templates
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "frappe://doctype/{name}",
		Name:        "Frappe DocType Metadata",
		MIMEType:    "application/json",
	}, handler)
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "frappe://doctype/{name}/schema",
		Name:        "Frappe DocType Canonical Schema",
		MIMEType:    "application/json",
	}, handler)
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "frappe://doctype/{name}/ui-candidates",
		Name:        "Frappe DocType A2UI Candidates",
		MIMEType:    "application/json",
	}, handler)
}

func resourceResponse(uri string, data any) (*mcp.ReadResourceResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      uri,
				MIMEType: "application/json",
				Text:     string(jsonBytes),
			},
		},
	}, nil
}

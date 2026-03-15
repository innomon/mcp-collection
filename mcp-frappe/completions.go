package main

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newCompletionHandler(client *FrappeClient, cfg Config) func(context.Context, *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	return func(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
		argName := req.Params.Argument.Name
		prefix := strings.ToLower(req.Params.Argument.Value)

		// Completion for "doctype" argument
		if argName == "doctype" || (req.Params.Ref.Type == "ref/resource" && req.Params.Ref.Name == "frappe://doctype/{name}") {
			data, err := client.GetDocTypes(ctx)
			if err != nil {
				return nil, err
			}

			var matches []string
			if m, ok := data.(map[string]any); ok {
				if items, ok := m["data"].([]any); ok {
					for _, item := range items {
						if doc, ok := item.(map[string]any); ok {
							if name, ok := doc["name"].(string); ok {
								if strings.HasPrefix(strings.ToLower(name), prefix) {
									if isDocTypeAllowed(cfg.AllowedDocTypes, name) {
										matches = append(matches, name)
									}
								}
							}
						}
					}
				}
			}

			// Cap at 50 results
			if len(matches) > 50 {
				matches = matches[:50]
			}

			return &mcp.CompleteResult{
				Completion: mcp.CompletionResultDetails{
					Values:  matches,
					Total:   len(matches),
					HasMore: len(matches) == 50,
				},
			}, nil
		}

		// Completion for "name" (record ID) when "doctype" is known
		if argName == "name" && req.Params.Ref.Type == "ref/tool" {
			docType := ""
			// Try to find doctype in other arguments via Context
			if req.Params.Context != nil && req.Params.Context.Arguments != nil {
				if val, ok := req.Params.Context.Arguments["doctype"]; ok {
					docType = val
				}
			}

			if docType == "" {
				return emptyCompletion(), nil
			}

			data, err := client.GetRecords(ctx, docType, req.Params.Argument.Value)
			if err != nil {
				return nil, err
			}

			var matches []string
			if m, ok := data.(map[string]any); ok {
				if items, ok := m["data"].([]any); ok {
					for _, item := range items {
						if doc, ok := item.(map[string]any); ok {
							if name, ok := doc["name"].(string); ok {
								matches = append(matches, name)
							}
						}
					}
				}
			}

			return &mcp.CompleteResult{
				Completion: mcp.CompletionResultDetails{
					Values:  matches,
					Total:   len(matches),
					HasMore: false,
				},
			}, nil
		}

		return emptyCompletion(), nil
	}
}

func emptyCompletion() *mcp.CompleteResult {
	return &mcp.CompleteResult{
		Completion: mcp.CompletionResultDetails{
			Values: []string{},
		},
	}
}

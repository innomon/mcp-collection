package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterTools(s *mcp.Server, cfg *Config) {
	// IMAP Tools
	s.AddTool(&mcp.Tool{
		Name:        "list_folders",
		Description: "List all mail folders (INBOX, Sent, etc.)",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"account_id":{"type":"string","description":"Optional account ID (defaults to first account)"}}}`),
	}, handleListFolders(cfg))

	s.AddTool(&mcp.Tool{
		Name:        "list_messages",
		Description: "List recent messages in a folder",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"folder":{"type":"string","description":"Folder name (default: INBOX)"},"limit":{"type":"integer","description":"Number of messages to fetch (default 10)"},"account_id":{"type":"string"}},"required":["folder"]}`),
	}, handleListMessages(cfg))

	s.AddTool(&mcp.Tool{
		Name:        "get_message",
		Description: "Fetch full message content",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"message_id":{"type":"string","description":"Internal message ID"},"folder":{"type":"string"},"account_id":{"type":"string"}},"required":["message_id","folder"]}`),
	}, handleGetMessage(cfg))

	// SMTP Tools
	s.AddTool(&mcp.Tool{
		Name:        "send_email",
		Description: "Send a new email",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"to":{"type":"array","items":{"type":"string"}},"subject":{"type":"string"},"body":{"type":"string"},"is_html":{"type":"boolean"},"account_id":{"type":"string"}},"required":["to","subject","body"]}`),
	}, handleSendEmail(cfg))
}

func handleListFolders(cfg *Config) mcp.ToolHandlerFunc {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Not implemented yet"}},
		}, nil
	}
}

func handleListMessages(cfg *Config) mcp.ToolHandlerFunc {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Not implemented yet"}},
		}, nil
	}
}

func handleGetMessage(cfg *Config) mcp.ToolHandlerFunc {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Not implemented yet"}},
		}, nil
	}
}

func handleSendEmail(cfg *Config) mcp.ToolHandlerFunc {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Not implemented yet"}},
		}, nil
	}
}

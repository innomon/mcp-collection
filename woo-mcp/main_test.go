package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func mockHandler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{}, nil
}

func TestServerInitialization(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "TestServer", Version: "1.0.0"}, nil)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}

	s.AddTool(&mcp.Tool{
		Name:        "mock_tool",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}, mockHandler)
}

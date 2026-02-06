package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Configuration & Client ---

type Config struct {
	BaseURL   string
	APIKey    string
	APISecret string
}

type FrappeClient struct {
	Config Config
}

func (c *FrappeClient) Do(method, path string, body []byte) (string, error) {
	url := fmt.Sprintf("%s/api/resource/%s", c.Config.BaseURL, path)
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", c.Config.APIKey, c.Config.APISecret))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("frappe error (%d): %s", resp.StatusCode, string(respBody))
	}
	return string(respBody), nil
}

// --- Tool Argument Structs ---
// The official SDK uses these tags to generate the tool's JSON schema automatically.

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
	config := Config{
		BaseURL:   os.Getenv("FRAPPE_URL"),
		APIKey:    os.Getenv("FRAPPE_API_KEY"),
		APISecret: os.Getenv("FRAPPE_API_SECRET"),
	}

	if config.BaseURL == "" || config.APIKey == "" {
		log.Fatal("Environment variables FRAPPE_URL, FRAPPE_API_KEY, and FRAPPE_API_SECRET are required.")
	}

	client := &FrappeClient{Config: config}

	// 1. Initialize official Server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "frappe-mcp",
		Version: "1.0.0",
	}, nil)

	// 2. Register Search Tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "frappe_search",
		Description: "Search Frappe records with filters",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args SearchArgs) (*mcp.CallToolResult, error) {
		path := fmt.Sprintf("%s?filters=%s&fields=%s", args.DocType, args.Filters, args.Fields)
		res, err := client.Do("GET", path, nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(res), nil
	})

	// 3. Register Read Tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "frappe_get_record",
		Description: "Get a specific record's full details",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetRecordArgs) (*mcp.CallToolResult, error) {
		path := fmt.Sprintf("%s/%s", args.DocType, args.Name)
		res, err := client.Do("GET", path, nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(res), nil
	})

	// 4. Register Create Tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "frappe_create_record",
		Description: "Create a new record in Frappe",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args CreateRecordArgs) (*mcp.CallToolResult, error) {
		body, _ := json.Marshal(args.DocJSON)
		res, err := client.Do("POST", args.DocType, body)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(res), nil
	})

	// 5. Register Update Tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "frappe_update_record",
		Description: "Update fields on an existing record",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args UpdateRecordArgs) (*mcp.CallToolResult, error) {
		body, _ := json.Marshal(args.UpdateJSON)
		path := fmt.Sprintf("%s/%s", args.DocType, args.Name)
		res, err := client.Do("PUT", path, body)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(res), nil
	})

	// 6. Run using Stdio Transport
	log.Println("Frappe MCP Server starting...")
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

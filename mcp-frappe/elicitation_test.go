package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestElicitationBuilder_BuildFormSchema(t *testing.T) {
	builder := &ElicitationBuilder{}
	options := "Option 1\nOption 2\nOption 3"
	fields := []FrappeField{
		{
			Fieldname: "test_data",
			Label:     "Test Data",
			Fieldtype: "Data",
			Reqd:      true,
		},
		{
			Fieldname: "test_select",
			Label:     "Test Select",
			Fieldtype: "Select",
			Options:   &options,
			Reqd:      false,
		},
	}

	schema := builder.BuildFormSchema(fields)
	
	if schema["type"] != "object" {
		t.Errorf("expected type object, got %v", schema["type"])
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map")
	}

	if _, ok := properties["test_data"]; !ok {
		t.Errorf("expected test_data property")
	}

	testSelect, ok := properties["test_select"].(map[string]any)
	if !ok {
		t.Fatalf("expected test_select property map")
	}

	enum, ok := testSelect["enum"].([]string)
	if !ok {
		t.Errorf("expected enum for Select field")
	}
	if len(enum) != 3 {
		t.Errorf("expected 3 enum options, got %d", len(enum))
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("expected required slice")
	}
	if len(required) != 1 || required[0] != "test_data" {
		t.Errorf("expected test_data to be required")
	}
}

func TestElicitation_DeleteProduction(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		EnableDelete:         true,
		Environment:          "production",
		DeleteApprovalToken: "secret123",
		AllowedDocTypes:      map[string]struct{}{"customer": {}},
	}
	client := &FrappeClient{Config: cfg, HTTPClient: http.DefaultClient}
	
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{
			Experimental: map[string]any{"elicitation": map[string]any{}},
		},
	})
	registerTools(server, client, cfg)

	// Setup In-Memory Transport
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	
	go server.Connect(ctx, serverTransport, nil)

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "client"}, &mcp.ClientOptions{
		Capabilities: &mcp.ClientCapabilities{
			Elicitation: &mcp.ElicitationCapabilities{
				Form: &mcp.FormElicitationCapabilities{},
			},
		},
	})
	
	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer session.Close()

	t.Run("Triggers Elicitation on Missing Token", func(t *testing.T) {
		params := &mcp.CallToolParams{
			Name: "frappe_delete_record",
			Arguments: map[string]any{
				"doctype": "Customer",
				"name":    "CUST-001",
				"dry_run": false,
			},
		}

		res, err := session.CallTool(ctx, params)
		if err != nil {
			t.Fatalf("CallTool failed: %v", err)
		}

		if res == nil {
			t.Fatalf("expected result, got nil")
		}

		if !res.IsError {
			content, _ := json.Marshal(res.Content)
			t.Errorf("expected IsError true for elicitation, got false. Content: %s", string(content))
		}

		elicitRaw, ok := res.Meta["elicitation"]
		if !ok {
			content, _ := json.Marshal(res.Content)
			t.Fatalf("expected elicitation in Meta. Content: %s", string(content))
		}

		elicit, ok := elicitRaw.(map[string]any)
		if !ok {
			t.Fatalf("expected elicitation to be a map")
		}

		if elicit["mode"] != "form" {
			t.Errorf("expected mode form, got %v", elicit["mode"])
		}
	})
}

func TestElicitation_CreateMandatory(t *testing.T) {
	ctx := context.Background()
	
	// Mock Frappe Server
	mux := http.NewServeMux()
	mux.HandleFunc("/api/resource/Customer", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{
				"exc_type": "MandatoryError",
				"exception": "Mandatory fields required: customer_name",
			})
		}
	})
	mux.HandleFunc("/api/resource/DocType/Customer", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": map[string]any{
				"name": "Customer",
				"fields": []map[string]any{
					{"fieldname": "customer_name", "label": "Customer Name", "fieldtype": "Data", "reqd": 1},
					{"fieldname": "email", "label": "Email", "fieldtype": "Data", "reqd": 0},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	cfg := Config{
		BaseURL:         ts.URL,
		AllowedDocTypes: map[string]struct{}{"customer": {}},
	}
	client := &FrappeClient{
		Config:     cfg,
		HTTPClient: ts.Client(),
	}
	
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{
			Experimental: map[string]any{"elicitation": map[string]any{}},
		},
	})
	registerTools(server, client, cfg)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	go server.Connect(ctx, serverTransport, nil)

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "client"}, &mcp.ClientOptions{
		Capabilities: &mcp.ClientCapabilities{
			Elicitation: &mcp.ElicitationCapabilities{
				Form: &mcp.FormElicitationCapabilities{},
			},
		},
	})
	
	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer session.Close()

	t.Run("Triggers Elicitation on MandatoryError", func(t *testing.T) {
		params := &mcp.CallToolParams{
			Name: "frappe_create_record",
			Arguments: map[string]any{
				"doctype": "Customer",
				"doc_json": map[string]any{
					"email": "test@example.com",
				},
			},
		}

		res, err := session.CallTool(ctx, params)
		if err != nil {
			t.Fatalf("CallTool failed: %v", err)
		}

		if !res.IsError {
			t.Errorf("expected IsError true")
		}

		elicitRaw := res.Meta["elicitation"]
		elicit := elicitRaw.(map[string]any)
		schema := elicit["requestedSchema"].(map[string]any)
		props := schema["properties"].(map[string]any)

		if _, ok := props["customer_name"]; !ok {
			t.Errorf("expected customer_name in elicited schema")
		}
		
		// Ensure email is NOT in elicited schema because it was already provided
		if _, ok := props["email"]; ok {
			t.Errorf("did not expect email in elicited schema")
		}
	})
}

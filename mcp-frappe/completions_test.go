package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCompletionHandler(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/resource/DocType", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"name": "Customer"},
				{"name": "Contact"},
				{"name": "Supplier"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/api/resource/Customer", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"name": "CUST-001"},
				{"name": "CUST-002"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := &FrappeClient{
		Config: Config{
			BaseURL: ts.URL,
		},
		HTTPClient: ts.Client(),
	}
	cfg := Config{
		AllowedDocTypes: map[string]struct{}{},
	}

	handler := newCompletionHandler(client, cfg)

	t.Run("Doctype Completion", func(t *testing.T) {
		req := &mcp.CompleteRequest{
			Params: &mcp.CompleteParams{
				Argument: mcp.CompleteParamsArgument{
					Name:  "doctype",
					Value: "Cu",
				},
				Ref: &mcp.CompleteReference{
					Type: "ref/tool",
					Name: "frappe_search",
				},
			},
		}

		res, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("handler failed: %v", err)
		}

		if len(res.Completion.Values) != 1 || res.Completion.Values[0] != "Customer" {
			t.Errorf("expected [Customer], got %v", res.Completion.Values)
		}
	})

	t.Run("Name Completion with Context", func(t *testing.T) {
		req := &mcp.CompleteRequest{
			Params: &mcp.CompleteParams{
				Argument: mcp.CompleteParamsArgument{
					Name:  "name",
					Value: "CUST",
				},
				Ref: &mcp.CompleteReference{
					Type: "ref/tool",
					Name: "frappe_get_record",
				},
				Context: &mcp.CompleteContext{
					Arguments: map[string]string{
						"doctype": "Customer",
					},
				},
			},
		}

		res, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("handler failed: %v", err)
		}

		if len(res.Completion.Values) != 2 {
			t.Errorf("expected 2 values, got %d", len(res.Completion.Values))
		}
	})
}

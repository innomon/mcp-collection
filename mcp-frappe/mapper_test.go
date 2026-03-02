package main

import (
	"encoding/json"
	"testing"
)

func strPtr(s string) *string { return &s }

func TestDocTypeSchemaName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Sales Order", "sales_order"},
		{"Customer", "customer"},
		{"Purchase Invoice", "purchase_invoice"},
		{"Item", "item"},
	}
	for _, tt := range tests {
		if got := docTypeSchemaName(tt.input); got != tt.want {
			t.Errorf("docTypeSchemaName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFilterDataFields(t *testing.T) {
	fields := []FrappeField{
		{Fieldname: "customer_name", Label: "Customer Name", Fieldtype: "Data"},
		{Fieldname: "sb1", Label: "", Fieldtype: "Section Break"},
		{Fieldname: "email", Label: "Email", Fieldtype: "Data"},
		{Fieldname: "cb1", Label: "", Fieldtype: "Column Break"},
		{Fieldname: "phone", Label: "Phone", Fieldtype: "Data"},
		{Fieldname: "tb1", Label: "", Fieldtype: "Tab Break"},
	}

	got := filterDataFields(fields)
	if len(got) != 3 {
		t.Fatalf("filterDataFields: got %d fields, want 3", len(got))
	}
	names := []string{got[0].Fieldname, got[1].Fieldname, got[2].Fieldname}
	want := []string{"customer_name", "email", "phone"}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("field[%d] = %q, want %q", i, n, want[i])
		}
	}
}

func TestDocTypeToCandidates(t *testing.T) {
	dt := FrappeDocType{
		Name:   "Customer",
		Module: "Selling",
		Fields: []FrappeField{
			{Fieldname: "customer_name", Label: "Customer Name", Fieldtype: "Data", Reqd: true},
			{Fieldname: "email_id", Label: "Email", Fieldtype: "Data"},
		},
		Modified: "2026-01-01T00:00:00Z",
	}

	candidates := DocTypeToCandidates([]FrappeDocType{dt})
	if len(candidates) != 1 {
		t.Fatalf("got %d candidates, want 1", len(candidates))
	}

	c := candidates[0]
	if c.Schema != "customer" {
		t.Errorf("schema = %q, want %q", c.Schema, "customer")
	}
	if c.Source == nil || c.Source.Source != "doctype" || c.Source.DocType != "Customer" {
		t.Errorf("unexpected source: %+v", c.Source)
	}

	var example map[string]any
	if err := json.Unmarshal(c.Example, &example); err != nil {
		t.Fatalf("unmarshal example: %v", err)
	}
	if example["doctype"] != "Customer" {
		t.Errorf("example doctype = %v, want Customer", example["doctype"])
	}
}

func TestDocTypeToCandidatesEmpty(t *testing.T) {
	candidates := DocTypeToCandidates(nil)
	if len(candidates) != 0 {
		t.Errorf("got %d candidates for nil input, want 0", len(candidates))
	}
}

func TestFieldPlaceholder(t *testing.T) {
	tests := []struct {
		fieldtype string
		want      any
	}{
		{"Int", 0},
		{"Float", 0.0},
		{"Currency", 0.0},
		{"Check", false},
		{"Data", ""},
		{"Link", ""},
		{"Table", []any{}},
	}
	for _, tt := range tests {
		f := FrappeField{Fieldtype: tt.fieldtype}
		got := fieldPlaceholder(f)
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(tt.want)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("fieldPlaceholder(%s) = %s, want %s", tt.fieldtype, gotJSON, wantJSON)
		}
	}
}

func TestDocTypeDescription(t *testing.T) {
	dt := &FrappeDocType{
		Name:   "Item",
		Module: "Stock",
		Fields: []FrappeField{
			{Fieldname: "item_name", Label: "Item Name", Fieldtype: "Data", Reqd: true},
			{Fieldname: "sb1", Label: "", Fieldtype: "Section Break"},
			{Fieldname: "description", Label: "Description", Fieldtype: "Text"},
		},
	}

	desc := docTypeDescription(dt)
	if desc == "" {
		t.Fatal("description is empty")
	}
	if !contains(desc, "Item") || !contains(desc, "Stock") {
		t.Errorf("description missing DocType/module info: %s", desc)
	}
	if !contains(desc, "Item Name") {
		t.Errorf("description missing field label: %s", desc)
	}
	if !contains(desc, "required") {
		t.Errorf("description missing required marker: %s", desc)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstr(s, substr)
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

package main

import (
	"encoding/json"
	"testing"
)

func TestMergeCandidatesStaticOnly(t *testing.T) {
	static := []SchemaCandidate{
		{Schema: "info_card", Description: "static info_card", Example: json.RawMessage(`{}`)},
		{Schema: "data_table", Description: "static data_table", Example: json.RawMessage(`{}`)},
	}

	merged := mergeCandidates(static, nil, 20)
	if len(merged) != 2 {
		t.Fatalf("got %d, want 2", len(merged))
	}
}

func TestMergeCandidatesDocTypeOnly(t *testing.T) {
	dt := []SchemaCandidate{
		{Schema: "customer", Description: "doctype customer", Example: json.RawMessage(`{}`)},
	}

	merged := mergeCandidates(nil, dt, 20)
	if len(merged) != 1 {
		t.Fatalf("got %d, want 1", len(merged))
	}
	if merged[0].Schema != "customer" {
		t.Errorf("schema = %q, want customer", merged[0].Schema)
	}
}

func TestMergeCandidatesDedup(t *testing.T) {
	static := []SchemaCandidate{
		{Schema: "info_card", Description: "static", Example: json.RawMessage(`{}`)},
	}
	dt := []SchemaCandidate{
		{Schema: "info_card", Description: "doctype", Example: json.RawMessage(`{}`)},
		{Schema: "customer", Description: "doctype customer", Example: json.RawMessage(`{}`)},
	}

	merged := mergeCandidates(static, dt, 20)
	if len(merged) != 2 {
		t.Fatalf("got %d, want 2", len(merged))
	}
	if merged[0].Description != "static" {
		t.Errorf("first candidate should be static, got %q", merged[0].Description)
	}
}

func TestMergeCandidatesCap(t *testing.T) {
	static := []SchemaCandidate{
		{Schema: "a", Example: json.RawMessage(`{}`)},
		{Schema: "b", Example: json.RawMessage(`{}`)},
		{Schema: "c", Example: json.RawMessage(`{}`)},
	}
	dt := []SchemaCandidate{
		{Schema: "d", Example: json.RawMessage(`{}`)},
		{Schema: "e", Example: json.RawMessage(`{}`)},
	}

	merged := mergeCandidates(static, dt, 3)
	if len(merged) != 3 {
		t.Fatalf("got %d, want 3 (capped)", len(merged))
	}
}

func TestMergeCandidatesEmpty(t *testing.T) {
	merged := mergeCandidates(nil, nil, 20)
	if len(merged) != 0 {
		t.Fatalf("got %d, want 0", len(merged))
	}
}

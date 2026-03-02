package main

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRunPipelineNormalSelection(t *testing.T) {
	candidates := []SchemaCandidate{
		{Schema: "info_card", Description: "test", Example: json.RawMessage(`{}`),
			Source: &CandidateSource{Source: "doctype", DocType: "Customer", Version: "v1"}},
	}

	cfg := PipelineConfig{ConfidenceThreshold: 0.5, FallbackSchema: "markdown"}
	result, err := runPipeline(context.Background(), candidates, "test content", &DefaultSelector{}, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Schema != "info_card" {
		t.Errorf("schema = %q, want info_card", result.Schema)
	}
	if result.IsFallback {
		t.Error("expected is_fallback=false")
	}
	if result.SourceDocType == nil || result.SourceDocType.Name != "Customer" {
		t.Error("expected SourceDocType.Name=Customer")
	}
}

func TestRunPipelineFallbackNoCandidates(t *testing.T) {
	cfg := PipelineConfig{ConfidenceThreshold: 0.5, FallbackSchema: "markdown"}
	result, err := runPipeline(context.Background(), nil, "", &DefaultSelector{}, cfg, NewMetricsCollector())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Schema != "markdown" {
		t.Errorf("schema = %q, want markdown", result.Schema)
	}
	if !result.IsFallback {
		t.Error("expected is_fallback=true")
	}
}

type lowConfidenceSelector struct{}

func (s *lowConfidenceSelector) Select(_ context.Context, _ []SchemaCandidate, _ string) (*SelectionResult, error) {
	return &SelectionResult{Schema: "info_card", Confidence: 0.1, IsFallback: false}, nil
}

func TestRunPipelineFallbackLowConfidence(t *testing.T) {
	candidates := []SchemaCandidate{
		{Schema: "info_card", Example: json.RawMessage(`{}`)},
	}

	cfg := PipelineConfig{ConfidenceThreshold: 0.5, FallbackSchema: "markdown"}
	result, err := runPipeline(context.Background(), candidates, "", &lowConfidenceSelector{}, cfg, NewMetricsCollector())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Schema != "markdown" {
		t.Errorf("schema = %q, want markdown (fallback)", result.Schema)
	}
	if !result.IsFallback {
		t.Error("expected is_fallback=true")
	}
}

func TestResolveSourceDocType(t *testing.T) {
	candidates := []SchemaCandidate{
		{Schema: "customer", Source: &CandidateSource{Source: "doctype", DocType: "Customer", Version: "v1"}},
		{Schema: "info_card", Source: &CandidateSource{Source: "static"}},
	}

	dt := resolveSourceDocType(candidates, "customer")
	if dt == nil || dt.Name != "Customer" {
		t.Error("expected DocType=Customer")
	}

	dt = resolveSourceDocType(candidates, "info_card")
	if dt != nil {
		t.Error("expected nil for static source")
	}

	dt = resolveSourceDocType(candidates, "nonexistent")
	if dt != nil {
		t.Error("expected nil for nonexistent")
	}
}

func TestFallbackSchemaCandidate(t *testing.T) {
	c := FallbackSchemaCandidate()
	if c.Schema != "markdown" {
		t.Errorf("schema = %q, want markdown", c.Schema)
	}
	if c.Source == nil || c.Source.Source != "fallback" {
		t.Error("expected source=fallback")
	}
}

func TestConfigurableThresholdAndFallback(t *testing.T) {
	cfg := PipelineConfig{ConfidenceThreshold: 0.9, FallbackSchema: "custom_fallback"}
	candidates := []SchemaCandidate{
		{Schema: "info_card", Example: json.RawMessage(`{}`)},
	}
	// DefaultSelector returns confidence 1.0 which is above 0.9
	result, err := runPipeline(context.Background(), candidates, "", &DefaultSelector{}, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Schema != "info_card" {
		t.Errorf("schema = %q, want info_card", result.Schema)
	}

	// Low confidence selector should trigger custom fallback
	result, err = runPipeline(context.Background(), candidates, "", &lowConfidenceSelector{}, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Schema != "custom_fallback" {
		t.Errorf("schema = %q, want custom_fallback", result.Schema)
	}
}

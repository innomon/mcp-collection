package main

import "context"

// Selector chooses the best A2UI schema for a given agent response.
type Selector interface {
	Select(ctx context.Context, candidates []SchemaCandidate, content string) (*SelectionResult, error)
}

// DefaultSelector returns the first candidate with confidence 1.0.
// It serves as a simple stub when no LLM-backed selector is available.
type DefaultSelector struct{}

func (d *DefaultSelector) Select(_ context.Context, candidates []SchemaCandidate, _ string) (*SelectionResult, error) {
	if len(candidates) == 0 {
		return &SelectionResult{
			Schema:     "",
			Confidence: 0,
			IsFallback: true,
		}, nil
	}
	return &SelectionResult{
		Schema:     candidates[0].Schema,
		Confidence: 1.0,
		IsFallback: false,
	}, nil
}

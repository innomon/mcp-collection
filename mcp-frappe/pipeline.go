package main

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

const defaultConfidenceThreshold = 0.5
const fallbackSchemaName = "markdown"

// runPipeline orchestrates: assemble → select → confidence threshold → fallback.
func runPipeline(ctx context.Context, candidates []SchemaCandidate, content string, selector Selector, cfg PipelineConfig, mc *MetricsCollector) (*PipelineResult, error) {
	start := time.Now()

	confidence := cfg.ConfidenceThreshold
	if confidence <= 0 {
		confidence = defaultConfidenceThreshold
	}
	fallback := cfg.FallbackSchema
	if fallback == "" {
		fallback = fallbackSchemaName
	}

	// No candidates → immediate fallback.
	if len(candidates) == 0 {
		result := fallbackResult(fallback)
		if mc != nil {
			mc.IncCounter(metricSchemaFallbackTotal, map[string]string{"reason": "no_candidates"})
		}
		log.Printf("pipeline_fallback_no_candidates schema=%s duration_ms=%d", result.Schema, time.Since(start).Milliseconds())
		return result, nil
	}

	// Run selector.
	sel, err := selector.Select(ctx, candidates, content)
	if err != nil {
		log.Printf("pipeline_selector_failed error=%v duration_ms=%d", err, time.Since(start).Milliseconds())
		return nil, err
	}

	// Confidence threshold check.
	if sel.Confidence < confidence {
		result := fallbackResult(fallback)
		if mc != nil {
			mc.IncCounter(metricSchemaFallbackTotal, map[string]string{"reason": "low_confidence"})
		}
		log.Printf("pipeline_fallback_low_confidence attempted=%s confidence=%.2f threshold=%.2f fallback=%s duration_ms=%d",
			sel.Schema, sel.Confidence, confidence, result.Schema, time.Since(start).Milliseconds())
		return result, nil
	}

	// Build result and resolve source DocType if applicable.
	pResult := &PipelineResult{
		SelectionResult: *sel,
	}
	pResult.SourceDocType = resolveSourceDocType(candidates, sel.Schema)

	durationMs := time.Since(start).Milliseconds()
	if mc != nil {
		mc.ObserveLatency(metricSchemaPipelineLatencyMs, nil, durationMs)
	}
	log.Printf("pipeline_selection_complete schema=%s confidence=%.2f is_fallback=%t source=%s candidates=%d duration_ms=%d",
		sel.Schema, sel.Confidence, sel.IsFallback, candidateSourceLabel(candidates, sel.Schema), len(candidates), durationMs)

	return pResult, nil
}

func fallbackResult(fallbackSchema string) *PipelineResult {
	return &PipelineResult{
		SelectionResult: SelectionResult{
			Schema:     fallbackSchema,
			Confidence: 1.0,
			IsFallback: true,
		},
	}
}

// resolveSourceDocType finds the DocType metadata for a selected schema.
func resolveSourceDocType(candidates []SchemaCandidate, schemaName string) *FrappeDocType {
	for _, c := range candidates {
		if c.Schema != schemaName {
			continue
		}
		if c.Source == nil || c.Source.Source != "doctype" {
			return nil
		}
		return &FrappeDocType{
			Name:     c.Source.DocType,
			Modified: c.Source.Version,
		}
	}
	return nil
}

func candidateSourceLabel(candidates []SchemaCandidate, schemaName string) string {
	for _, c := range candidates {
		if c.Schema != schemaName {
			continue
		}
		if c.Source != nil {
			if c.Source.Source == "doctype" {
				return "doctype:" + c.Source.DocType
			}
			return c.Source.Source
		}
		return "unknown"
	}
	return "unknown"
}

// FallbackSchemaCandidate returns the default fallback schema as a candidate.
func FallbackSchemaCandidate() SchemaCandidate {
	return SchemaCandidate{
		Schema:      fallbackSchemaName,
		Description: "Plain markdown text response. Used when no structured schema matches with sufficient confidence.",
		Example:     json.RawMessage(`{"text":"Response content in markdown format."}`),
		Source:      &CandidateSource{Source: "fallback"},
	}
}

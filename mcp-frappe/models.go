package main

import "encoding/json"

// FrappeDocType is the normalized representation of a Frappe DocType
// used by the A2UI schema-selection pipeline. This is distinct from
// CanonicalDocTypeJSON which is the contract output format.
type FrappeDocType struct {
	Name          string             `json:"name"`
	Module        string             `json:"module"`
	IsSubmittable bool               `json:"is_submittable"`
	Modified      string             `json:"modified"`
	Fields        []FrappeField      `json:"fields"`
	Permissions   []FrappePermission `json:"permissions"`
}

// FrappeField represents a single field definition within a FrappeDocType.
type FrappeField struct {
	Fieldname string  `json:"fieldname"`
	Label     string  `json:"label"`
	Fieldtype string  `json:"fieldtype"`
	Reqd      bool    `json:"reqd"`
	Options   *string `json:"options"`
}

// FrappePermission represents a role-based permission entry for a FrappeDocType.
type FrappePermission struct {
	Role   string `json:"role"`
	Read   bool   `json:"read"`
	Write  bool   `json:"write"`
	Create bool   `json:"create"`
	Delete bool   `json:"delete"`
	Submit bool   `json:"submit"`
}

// CandidateSource tracks the origin of a SchemaCandidate for internal
// logging and precedence decisions. It is not serialized to tool output.
type CandidateSource struct {
	Source  string `json:"source"`  // "static", "doctype", "fallback"
	DocType string `json:"doctype"` // DocType name when source is "doctype"
	Version string `json:"version"` // FrappeDocType.Modified timestamp
}

// SchemaCandidate is one candidate A2UI schema for the selection pipeline.
type SchemaCandidate struct {
	Schema      string          `json:"schema"`
	Description string          `json:"description"`
	Example     json.RawMessage `json:"example"`

	// Source is optional origin metadata used internally for logging and
	// precedence. It is excluded from JSON serialization to tool output.
	Source *CandidateSource `json:"-"`
}

// A2UI is the rendering envelope returned to the frontend.
type A2UI struct {
	Version string `json:"version"`
	Cards   []Card `json:"cards"`
}

// Card is a self-describing unit of UI content.
type Card struct {
	Schema  string          `json:"schema"`
	Payload json.RawMessage `json:"payload"`
}

// SelectionResult holds the outcome of schema selection.
type SelectionResult struct {
	Schema     string  `json:"schema"`
	Confidence float64 `json:"confidence"`
	IsFallback bool    `json:"is_fallback"`
}

// PipelineResult extends SelectionResult with optional DocType context.
type PipelineResult struct {
	SelectionResult

	// SourceDocType is set when the selected schema originated from a Frappe
	// DocType candidate. Nil for static schemas or fallback.
	SourceDocType *FrappeDocType `json:"source_doctype,omitempty"`
}

// PipelineConfig holds optional pipeline settings.
type PipelineConfig struct {
	ConfidenceThreshold float64
	FallbackSchema      string
	MaxCandidates       int
}

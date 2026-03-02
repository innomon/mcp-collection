# DocType A2UI Feature — Specification

> Extracted from `genui-chat` and adapted for standalone implementation in `mcp-frappe`.

## 1. Overview

This feature adds **DocType-to-A2UI schema mapping** and a **schema-selection pipeline** directly into the `mcp-frappe` MCP server. The goal is to make `mcp-frappe` a self-contained server capable of:

1. Fetching DocType metadata from Frappe.
2. Converting DocType definitions into A2UI `SchemaCandidate` entries.
3. Assembling, merging, and ranking schema candidates from multiple sources (static + DocType-derived).
4. Running a schema-selection pipeline (with configurable selector backend) to choose the best A2UI card schema for a given response.
5. Generating and validating canonical DocType JSON contracts.

All of this runs without any dependency on the `genui-chat` project.

## 2. Scope

### In Scope

| Component | Description |
|-----------|-------------|
| **Domain models** | `FrappeDocType`, `FrappeField`, `FrappePermission`, `FrappeRecord`, `SchemaCandidate`, `CandidateSource`, `A2UI`, `Card` |
| **DocType → SchemaCandidate mapper** | Converts DocType metadata to schema candidates (info_card, data_table, markdown, action_card) |
| **Schema assembler** | Merges static + DocType candidates with deterministic precedence, deduplication, and capping |
| **Schema selection pipeline** | Assembler → selector → confidence threshold → fallback, with `PipelineResult` |
| **Selector interface** | Pluggable `Selector` interface; default stub selector included, LLM selector optional |
| **DocType JSON generation service** | `GetDocTypeMeta`, `GenerateDocTypeJSON`, `ValidateDocTypeJSON` |
| **Frappe HTTP adapter with retry** | Shared HTTP client, token auth, exponential backoff retry, allowlist gating |
| **In-memory TTL cache** | Caching layer for DocType metadata |
| **Metrics collector** | Lightweight stdlib-only counter/latency collector |
| **MCP tool registration** | New MCP tools: `frappe_select_schema`, `frappe_map_doctype_to_candidates` |
| **Docs** | Updated `doctype-schema-mapping.md`, `frappe-doctype-json-contract.md`, new `a2ui-doctype-spec.md` |

### Out of Scope

- Flutter/frontend rendering.
- ADK agent routing.
- Authentication (JWT, role resolution).
- Firebase data storage.
- LLM provider implementation (only the `Selector` interface is provided; callers wire their own LLM).

## 3. Architecture

```
┌──────────────────────────────────────────────────────────┐
│                     mcp-frappe server                     │
│                                                          │
│  ┌──────────────────────┐   ┌──────────────────────────┐ │
│  │   Existing CRUD      │   │   New A2UI Pipeline      │ │
│  │   Tools              │   │                          │ │
│  │                      │   │  DocType Mapper          │ │
│  │  frappe_search       │   │  Schema Assembler        │ │
│  │  frappe_get_record   │   │  Schema Pipeline         │ │
│  │  frappe_create_record│   │  Selector Interface      │ │
│  │  frappe_update_record│   │  Metrics Collector       │ │
│  │  frappe_delete_record│   │                          │ │
│  │                      │   │  New Tools:              │ │
│  │  DocType gen/validate│   │  frappe_select_schema    │ │
│  │  frappe_get_doctype_ │   │  frappe_map_doctype_to_  │ │
│  │    meta              │   │    candidates            │ │
│  └──────────────────────┘   └──────────────────────────┘ │
│                                                          │
│  ┌──────────────────────────────────────────────────────┐ │
│  │            Shared Infrastructure                     │ │
│  │  FrappeClient (HTTP + auth + retry)                  │ │
│  │  DocType Cache (TTL)                                 │ │
│  │  Metrics Collector                                   │ │
│  │  Config (env vars)                                   │ │
│  └──────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────┘
```

## 4. Domain Models (Ported from genui-chat)

### 4.1 FrappeDocType / FrappeField / FrappePermission

Normalized representation of a Frappe DocType. Used by the mapper and the schema pipeline.

```go
type FrappeDocType struct {
    Name          string
    Module        string
    IsSubmittable bool
    Modified      string
    Fields        []FrappeField
    Permissions   []FrappePermission
}
```

### 4.2 SchemaCandidate / CandidateSource

```go
type SchemaCandidate struct {
    Schema      string
    Description string
    Example     json.RawMessage
    Source      *CandidateSource // internal-only; not serialized to tool output
}

type CandidateSource struct {
    Source  string // "static", "doctype", "fallback"
    DocType string
    Version string
}
```

### 4.3 A2UI / Card

```go
type A2UI struct {
    Version string `json:"version"`
    Cards   []Card `json:"cards"`
}

type Card struct {
    Schema  string          `json:"schema"`
    Payload json.RawMessage `json:"payload"`
}
```

## 5. Mapping Rules

Port the mapping logic from `genui-chat/backend/internal/schema/mapper.go`:

| Condition | Generated Candidate |
|-----------|-------------------|
| Has non-layout, non-table data fields | `info_card` |
| Has `Table` or `Table MultiSelect` fields | `data_table` |
| Has long-text fields (`Text`, `Text Editor`, `HTML`) | `markdown` |
| `is_submittable=true` + role has submit permission | `action_card` |

Layout fields (`Section Break`, `Column Break`, `Tab Break`) are excluded.

## 6. Schema Pipeline

### 6.1 Assembler

- Accepts static candidates + DocType candidates.
- Deterministic precedence: static > doctype.
- First-seen-wins deduplication by schema name.
- Configurable candidate cap (default 20).

### 6.2 Selector Interface

```go
type Selector interface {
    Select(ctx context.Context, candidates []SchemaCandidate, content string) (*SelectionResult, error)
}
```

Default implementation: `HighestMatchSelector` (returns first candidate with confidence 1.0). Callers can provide an LLM-backed selector.

### 6.3 Pipeline

Assembler → Selector → confidence threshold (default 0.5) → fallback (`markdown`).

Returns `PipelineResult` with optional `SourceDocType` context.

## 7. New MCP Tools

### `frappe_map_doctype_to_candidates`

Fetches DocType metadata and returns derived `SchemaCandidate` entries.

Input: `{ "doctype": "Customer" }`
Output: `{ "candidates": [...] }`

### `frappe_select_schema`

Runs the full pipeline: fetch DocType → map to candidates → merge with optional static candidates → select best schema.

Input: `{ "doctype": "Customer", "content": "Here is the customer info...", "static_candidates": [...] }`
Output: `{ "schema": "info_card", "confidence": 0.85, "is_fallback": false, "source_doctype": "Customer" }`

## 8. Configuration

All existing env vars are preserved. New additions:

| Variable | Default | Description |
|----------|---------|-------------|
| `FRAPPE_MCP_ENABLE_A2UI_PIPELINE` | `false` | Feature flag to enable A2UI pipeline tools |
| `FRAPPE_MCP_CONFIDENCE_THRESHOLD` | `0.5` | Schema selection confidence threshold |
| `FRAPPE_MCP_MAX_CANDIDATES` | `20` | Maximum candidates in merged list |
| `FRAPPE_MCP_FALLBACK_SCHEMA` | `markdown` | Default fallback schema name |
| `FRAPPE_MCP_CACHE_TTL_SEC` | `300` | DocType metadata cache TTL in seconds |

## 9. Metrics

Port the lightweight stdlib-only `metrics.Collector` from genui-chat:

| Metric | Type | Description |
|--------|------|-------------|
| `doctype_fetch_total` | counter | DocType fetch operations |
| `doctype_fetch_latency_ms` | latency | DocType fetch latency |
| `doctype_cache_hit_total` | counter | Cache hits |
| `doctype_cache_miss_total` | counter | Cache misses |
| `schema_assembly_latency_ms` | latency | Schema assembly time |
| `schema_pipeline_latency_ms` | latency | Full pipeline time |
| `schema_fallback_total` | counter | Fallback activations |

## 10. File Structure (Target)

Since `mcp-frappe` is a single-file `package main` server, all new code is added to `main.go` and supporting files:

```
mcp-frappe/
├── main.go              # Existing + new tool registrations
├── models.go            # FrappeDocType, SchemaCandidate, A2UI types
├── mapper.go            # DocType → SchemaCandidate mapping
├── assembler.go         # Schema candidate merge + dedup
├── pipeline.go          # Schema selection pipeline
├── selector.go          # Selector interface + default impl
├── cache.go             # In-memory TTL cache for DocType metadata
├── metrics.go           # Lightweight metrics collector
├── go.mod
├── go.sum
└── docs/
    ├── a2ui-doctype-spec.md
    ├── doctype-schema-mapping.md
    ├── frappe-doctype-json-contract.md
    ├── frappe-doctype-task-checklist.md
    └── frappe-doctype.md
```

## 11. Security

- All existing security controls preserved (allowlist, delete gating, approval tokens).
- A2UI pipeline tools gated by `FRAPPE_MCP_ENABLE_A2UI_PIPELINE` feature flag.
- No credentials exposed in tool output or metrics.
- `Password` field type excluded from schema mapping output.

## 12. Testing Strategy

- Unit tests for mapper (field filtering, candidate generation).
- Unit tests for assembler (precedence, dedup, cap).
- Unit tests for pipeline (fallback, threshold, source resolution).
- Unit tests for cache (TTL, hit/miss).
- Unit tests for metrics (counter increment, latency observation).
- Round-trip tests: generate DocType JSON → validate → map to candidates.
- Build verification: `go build .` and `go vet ./...`

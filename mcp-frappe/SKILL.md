---
name: mcp-frappe
description: >
  Frappe ERP MCP server providing CRUD operations, DocType metadata introspection,
  and A2UI schema selection pipeline over MCP (stdio or SSE transport). Use when
  interacting with Frappe/ERPNext instances, managing records, generating or validating
  DocType JSON contracts, or selecting UI schemas from DocType metadata.
metadata:
  author: innomon
  version: "1.1.0"
  language: go
  protocol: mcp
  sdk: github.com/modelcontextprotocol/go-sdk v1.2.0
compatibility: >
  Requires Go 1.25+ to build. Requires network access to a Frappe/ERPNext instance
  with API key/secret authentication. Supports stdio and SSE transports.
---

# Frappe ERP MCP Server

A Go MCP server that connects to a Frappe/ERPNext instance via REST API and exposes
CRUD operations, DocType metadata tools, and an A2UI schema selection pipeline as
MCP tools.

## When to use this skill

- Searching, reading, creating, updating, or deleting records in Frappe/ERPNext
- Fetching DocType metadata or generating normalized DocType JSON
- Validating DocType JSON against the v0.1.0 contract
- Converting DocType metadata into A2UI schema candidates
- Running the full A2UI schema selection pipeline (fetch → map → merge → select)

## Architecture

Single-binary Go server (`package main`). Multi-file layout:

| File | Purpose |
|------|---------|
| `main.go` | Config, HTTP client, tool registration, transport setup |
| `models.go` | Domain types: FrappeDocType, SchemaCandidate, PipelineResult |
| `mapper.go` | DocType → schema candidate mapping rules |
| `assembler.go` | Candidate merging, deduplication, static precedence |
| `selector.go` | Keyword-based schema selector with confidence scoring |
| `pipeline.go` | Orchestrator: assemble → select → threshold → fallback |
| `cache.go` | In-memory TTL cache for DocType metadata |
| `metrics.go` | Internal counters and latency tracking |

## MCP Tools

### CRUD tools (always enabled)

- **frappe_search** — Search records with filters and field selection
- **frappe_get_record** — Get a single record by DocType and name
- **frappe_create_record** — Create a new record
- **frappe_update_record** — Update fields on an existing record

### Delete tool (feature-flagged)

- **frappe_delete_record** — Delete a record. Requires `FRAPPE_MCP_ENABLE_DELETE=true`. In production (`FRAPPE_ENV=production`), requires `approval_token` matching `FRAPPE_MCP_DELETE_APPROVAL_TOKEN`. Supports `dry_run` mode.

### DocType tools (feature-flagged)

Requires `FRAPPE_MCP_ENABLE_DOCTYPE_GEN=true`:

- **frappe_get_doctype_meta** — Fetch raw DocType metadata from Frappe
- **frappe_generate_doctype_json** — Generate normalized DocType JSON (contract v0.1.0)
- **frappe_validate_doctype_json** — Validate DocType JSON against the contract

### A2UI pipeline tools (feature-flagged)

Requires `FRAPPE_MCP_ENABLE_A2UI_PIPELINE=true`:

- **frappe_map_doctype_to_candidates** — Convert DocType metadata into schema candidates
- **frappe_select_schema** — Full pipeline: fetch DocType, map candidates, merge with optional static candidates, select best schema with confidence scoring and fallback

## Configuration

All configuration is via environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `FRAPPE_BASE_URL` | Yes | — | Frappe instance URL |
| `FRAPPE_API_KEY` | Yes | — | API key for token auth |
| `FRAPPE_API_SECRET` | Yes | — | API secret for token auth |
| `FRAPPE_TIMEOUT_MS` | No | `15000` | HTTP request timeout in ms |
| `FRAPPE_MCP_TRANSPORT` | No | `stdio` | Transport: `stdio` or `sse` |
| `FRAPPE_MCP_ENABLE_DELETE` | No | `false` | Enable delete tool |
| `FRAPPE_MCP_ENABLE_DOCTYPE_GEN` | No | `false` | Enable DocType gen/validate tools |
| `FRAPPE_MCP_ENABLE_A2UI_PIPELINE` | No | `false` | Enable A2UI pipeline tools |
| `FRAPPE_ALLOWED_DOCTYPES` | No | — | Comma-separated allowlist of DocTypes |
| `FRAPPE_ENV` | No | — | Environment (`production` enables delete policy) |
| `FRAPPE_MCP_DELETE_APPROVAL_TOKEN` | No | — | Required for production deletes |
| `FRAPPE_MCP_CONFIDENCE_THRESHOLD` | No | `0.5` | Min confidence for schema selection |
| `FRAPPE_MCP_MAX_CANDIDATES` | No | `20` | Max candidates after merge |
| `FRAPPE_MCP_FALLBACK_SCHEMA` | No | `markdown` | Fallback schema name |
| `FRAPPE_MCP_CACHE_TTL_SEC` | No | `300` | DocType metadata cache TTL in seconds |

SSE-only variables (when `FRAPPE_MCP_TRANSPORT=sse`):

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `FRAPPE_MCP_SSE_HOST` | Yes | — | Listen host |
| `FRAPPE_MCP_SSE_PORT` | Yes | — | Listen port |
| `FRAPPE_MCP_SSE_PATH` | No | `/sse` | SSE endpoint path |

## Build and run

```bash
cd mcp-frappe && go build -o mcpfrp .
FRAPPE_BASE_URL=https://mysite.frappe.cloud \
  FRAPPE_API_KEY=key \
  FRAPPE_API_SECRET=secret \
  ./mcpfrp
```

## Run tests

```bash
cd mcp-frappe && go test -v ./...
```

## A2UI pipeline details

The pipeline converts Frappe DocType metadata into UI schema candidates:

1. **Fetch** — Retrieve DocType metadata (cached with configurable TTL)
2. **Map** — Apply field-type mapping rules to derive schema candidates
3. **Merge** — Combine static and DocType-derived candidates; static wins on name collision; deduplicate; cap at `MAX_CANDIDATES`
4. **Select** — Keyword-based scoring against content; produce schema + confidence
5. **Threshold** — If confidence < threshold, fall back to configurable fallback schema (default: `markdown`)

See `docs/a2ui-implementation-plan.md` and `docs/doctype-schema-mapping.md` for mapping rules and design rationale.

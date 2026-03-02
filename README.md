# MCP Collection

A monorepo of independent Go MCP (Model Context Protocol) servers.

## Servers

### mcp-pg-memory-srv

Knowledge graph memory server over PostgreSQL. Exposes MCP tools for creating, searching, updating, and deleting memories (nodes) and connections (edges) in a graph stored in PostgreSQL.

- **Transport**: Streamable HTTP
- **Database**: Connects directly to PostgreSQL via `pgx/v5` connection pool (parameterized queries)
- **Env vars**: `DATABASE_URL` (required, e.g. `postgres://user:pass@host:5432/dbname`), `MCP_PORT` (default `8080`)

### mcp2mcp-mem-srv

Similar memory server variant that uses genai-toolbox + `tools.yaml` for PostgreSQL access via MCP-to-MCP communication.

### mcp-frappe

Frappe ERP MCP server with CRUD and DocType contract tooling.

- **Transports**: `stdio` (default) and `sse` (configurable)
- **Core tools**:
  - `frappe_search`
  - `frappe_get_record`
  - `frappe_create_record`
  - `frappe_update_record`
  - `frappe_delete_record` (feature-flagged and policy-gated)
- **DocType tools** (feature-flagged):
  - `frappe_get_doctype_meta`
  - `frappe_generate_doctype_json`
  - `frappe_validate_doctype_json`
- **A2UI pipeline tools** (feature-flagged via `FRAPPE_MCP_ENABLE_A2UI_PIPELINE`):
  - `frappe_map_doctype_to_candidates` — converts DocType metadata into A2UI schema candidates
  - `frappe_select_schema` — runs the full schema selection pipeline (fetch → map → merge → select)
- **Key env vars**:
  - `FRAPPE_BASE_URL` (or legacy `FRAPPE_URL`), `FRAPPE_API_KEY`, `FRAPPE_API_SECRET`
  - `FRAPPE_TIMEOUT_MS`
  - `FRAPPE_MCP_TRANSPORT` (`stdio` or `sse`)
  - `FRAPPE_MCP_SSE_HOST`, `FRAPPE_MCP_SSE_PORT`, `FRAPPE_MCP_SSE_PATH` (required in `sse` mode)
  - `FRAPPE_MCP_ENABLE_DELETE`, `FRAPPE_MCP_ENABLE_DOCTYPE_GEN`
  - `FRAPPE_MCP_ENABLE_A2UI_PIPELINE` — enables A2UI schema pipeline tools
  - `FRAPPE_MCP_CONFIDENCE_THRESHOLD` (default `0.5`), `FRAPPE_MCP_MAX_CANDIDATES` (default `20`)
  - `FRAPPE_MCP_FALLBACK_SCHEMA` (default `markdown`), `FRAPPE_MCP_CACHE_TTL_SEC` (default `300`)
  - `FRAPPE_ALLOWED_DOCTYPES`
  - `FRAPPE_ENV`, `FRAPPE_MCP_DELETE_APPROVAL_TOKEN`

## Database Schema

Both memory servers use the same PostgreSQL schema:

- `memories` — nodes with `id`, `label`, `name`, `properties` (JSONB), `created_at`, `updated_at`
- `connections` — edges with `id`, `from_memory_id`, `to_memory_id`, `relationship_type`, `properties` (JSONB), `created_at`, `updated_at`

## Building

Each subproject is independent with its own `go.mod`:

```sh
cd mcp-pg-memory-srv && go build .
```

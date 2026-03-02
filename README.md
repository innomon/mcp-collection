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

### woo-mcp

WooCommerce MCP+UCP server exposing WooCommerce capabilities via both MCP (for AI agents) and REST (for direct platform integration), with A2UI card rendering.

- **Transports**: `stdio` (default), `http`, or `both`
- **UCP Capabilities**: Shopping discovery, checkout, order, fulfillment
- **MCP Tools** (legacy):
  - `search_products`, `get_order_history`, `checkout`, `raise_issue`
- **UCP MCP Tools** (when `ucp_enabled: true`):
  - Discovery: `search_shop_catalog`, `get_product`, `get_product_categories`, `search_shop_policies_and_faqs`
  - Checkout: `create_checkout`, `get_checkout`, `update_checkout`, `complete_checkout`, `cancel_checkout`
  - Order: `get_order`, `list_orders`
- **REST Endpoints** (when `transport` is `http` or `both`):
  - `GET /.well-known/ucp` — UCP business profile
  - `GET/POST/PATCH /ucp/v1/checkout-sessions`, `/ucp/v1/products`, `/ucp/v1/orders`
- **A2UI Cards**: Product, checkout, and order cards (when `a2ui_enabled: true`)
- **Config**: `config.yaml` with `store_url`, `consumer_key`, `consumer_secret`, `transport`, `http_port`, `ucp_enabled`, `a2ui_enabled`

## Database Schema

Both memory servers use the same PostgreSQL schema:

- `memories` — nodes with `id`, `label`, `name`, `properties` (JSONB), `created_at`, `updated_at`
- `connections` — edges with `id`, `from_memory_id`, `to_memory_id`, `relationship_type`, `properties` (JSONB), `created_at`, `updated_at`

## Building

Each subproject is independent with its own `go.mod`:

```sh
cd mcp-pg-memory-srv && go build .
```

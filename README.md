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

Frappe ERP MCP server. CRUD operations against Frappe REST API.

- **Transport**: stdio

## Database Schema

Both memory servers use the same PostgreSQL schema:

- `memories` — nodes with `id`, `label`, `name`, `properties` (JSONB), `created_at`, `updated_at`
- `connections` — edges with `id`, `from_memory_id`, `to_memory_id`, `relationship_type`, `properties` (JSONB), `created_at`, `updated_at`

## Building

Each subproject is independent with its own `go.mod`:

```sh
cd mcp-pg-memory-srv && go build .
```

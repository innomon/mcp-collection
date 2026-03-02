# AGENTS.md

## Commands
- Build a subproject: `cd <subproject> && go build .`
- Run a single test: `cd <subproject> && go test -run TestName -v ./...`
- Vet/lint: `cd <subproject> && go vet ./...`

## Architecture
Monorepo of independent Go MCP (Model Context Protocol) servers, each with its own `go.mod`. All use `github.com/modelcontextprotocol/go-sdk/mcp`.
- **mcp-pg-memory-srv** — Knowledge graph memory server over PostgreSQL (Streamable HTTP transport). Connects directly to PostgreSQL via `pgx/v5` connection pool. Env: `DATABASE_URL`, `MCP_PORT`.
- **mcp2mcp-mem-srv** — Similar memory server variant (same pattern, uses toolbox + `tools.yaml` for Postgres).
- **mcp-frappe** — Frappe ERP MCP server (supports `stdio` and `sse` transports via config). CRUD operations against Frappe REST API plus DocType metadata/generation/validation tools. Includes A2UI schema pipeline (feature-flagged) for converting DocType metadata into schema candidates and running selection pipelines.
- **woo-mcp** — WooCommerce MCP+UCP server (supports `stdio`, `http`, or `both` transports via `config.yaml`). Exposes WooCommerce via legacy MCP tools and UCP-compliant discovery/checkout/order tools. Includes REST/UCP endpoints, UCP profile at `/.well-known/ucp`, and A2UI card rendering. Config via `config.yaml` (store_url, consumer_key, consumer_secret, transport, http_port, ucp_enabled, a2ui_enabled). Multi-file layout: ucp.go (types), ucp_discovery.go, ucp_checkout.go, ucp_order.go, a2ui.go, a2ui_cards.go, rest.go, ucp_profile.go.

DB schema: `memories` (nodes) and `connections` (edges) tables in PostgreSQL. `mcp-pg-memory-srv` uses parameterized queries (`$1`, `$2`, …) via `pgx`. `mcp2mcp-mem-srv` uses string formatting with `escapeSQLString`.

## Code Style
- Go 1.25+, `package main` servers. `mcp-frappe` uses multi-file layout for A2UI pipeline (models.go, mapper.go, assembler.go, pipeline.go, selector.go, cache.go, metrics.go).
- Struct tags: `json` for serialization, `jsonschema:"description=..."` for MCP tool schema generation.
- Tool registration: `mcp.AddTool(server, &mcp.Tool{...}, handlerFunc)` with typed input structs.
- For typed tools, use the current SDK `ToolHandlerFor` signature: `func(ctx, req, input) (*mcp.CallToolResult, Output, error)`.
- Errors: wrap with `fmt.Errorf("context: %w", err)`, return early on error.
- Naming: PascalCase types/exports, camelCase locals. Input/Output suffix for tool IO structs.
- Config via env vars (`os.Getenv`). `mcp2mcp-mem-srv` uses `tools.yaml` for toolbox config.

## Documentation
- update README.md and AGENTS.md after changes are made & accepted.

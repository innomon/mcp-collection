# AGENTS.md

## Commands
- Build a subproject: `cd <subproject> && go build .`
- Run a single test: `cd <subproject> && go test -run TestName -v ./...`
- Vet/lint: `cd <subproject> && go vet ./...`

## Architecture
Monorepo of independent Go MCP (Model Context Protocol) servers, each with its own `go.mod`. All use `github.com/modelcontextprotocol/go-sdk/mcp`.
- **mcp-pg-memory-srv** — Knowledge graph memory server over PostgreSQL (Streamable HTTP transport). Connects directly to PostgreSQL via `pgx/v5` connection pool. Env: `DATABASE_URL`, `MCP_PORT`.
- **mcp2mcp-mem-srv** — Similar memory server variant (same pattern, uses toolbox + `tools.yaml` for Postgres).
- **mcp-frappe** — Frappe ERP MCP server (supports `stdio` and `sse` transports via config). CRUD operations against Frappe REST API plus DocType metadata/generation/validation tools.

DB schema: `memories` (nodes) and `connections` (edges) tables in PostgreSQL. `mcp-pg-memory-srv` uses parameterized queries (`$1`, `$2`, …) via `pgx`. `mcp2mcp-mem-srv` uses string formatting with `escapeSQLString`.

## Code Style
- Go 1.25+, single-file `package main` servers, no tests currently.
- Struct tags: `json` for serialization, `jsonschema:"description=..."` for MCP tool schema generation.
- Tool registration: `mcp.AddTool(server, &mcp.Tool{...}, handlerFunc)` with typed input structs.
- For typed tools, use the current SDK `ToolHandlerFor` signature: `func(ctx, req, input) (*mcp.CallToolResult, Output, error)`.
- Errors: wrap with `fmt.Errorf("context: %w", err)`, return early on error.
- Naming: PascalCase types/exports, camelCase locals. Input/Output suffix for tool IO structs.
- Config via env vars (`os.Getenv`). `mcp2mcp-mem-srv` uses `tools.yaml` for toolbox config.

## Documentation
- update README.md and AGENTS.md after changes are made & accepted.

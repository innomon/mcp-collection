# GEMINI.md: MCP Collection Specification

This document provides a technical overview and engineering standards for the MCP Collection, a monorepo of independent Go-based Model Context Protocol (MCP) servers.

---

## 1. Project Architecture

The repository is a monorepo containing multiple independent MCP servers. Each server resides in its own subdirectory with a dedicated `go.mod` file and follows the official MCP Go SDK patterns.

### Core Subprojects
- **mcp-pg-memory-srv**: A knowledge graph memory server using PostgreSQL.
  - **Transport**: Streamable HTTP.
  - **Key Features**: Direct PostgreSQL connection via `pgx/v5`, parameterized queries.
  - **Environment**: `DATABASE_URL`, `MCP_PORT`.
- **mcp2mcp-mem-srv**: A variant memory server using a toolbox pattern.
  - **Configuration**: Uses `tools.yaml` for toolbox/Postgres configuration.
- **mcp-frappe**: A Frappe ERP MCP server.
  - **Transport**: Supports `stdio` and `sse`.
  - **Key Features**: CRUD operations via Frappe REST API, DocType metadata tools, and a feature-flagged A2UI schema pipeline for generating UI schema candidates.
- **woo-mcp**: A comprehensive WooCommerce MCP + Universal Checkout Protocol (UCP) server.
  - **Transport**: Supports `stdio`, `http`, or `both`.
  - **Key Features**: Legacy MCP tools, UCP-compliant discovery/checkout/order tools, OAuth 2.0 identity linking, and A2UI card rendering.
  - **Configuration**: Managed via `config.yaml`.

---

## 2. Engineering Standards & Code Style

### Language & Libraries
- **Go Version**: 1.25+
- **MCP SDK**: Use ONLY the official [mcp go-sdk](https://github.com/modelcontextprotocol/go-sdk/mcp).
- **CLI Frameworks**: **NEVER** use `spf13` (Cobra/Pflag). Implement handcrafted command registries for CLI and slash commands.
- **Database**: `mcp-pg-memory-srv` uses `pgx/v5`. Always use parameterized queries to prevent SQL injection.

### Development Patterns
- **Server Structure**: Servers should be `package main`. For complex logic (like `mcp-frappe` A2UI or `woo-mcp`), use a multi-file layout (e.g., `models.go`, `pipeline.go`, `rest.go`).
- **Tool Registration**: Use `mcp.AddTool(server, &mcp.Tool{...}, handlerFunc)` with typed input structs.
- **Tool Handlers**: Use the current SDK `ToolHandlerFor` signature:
  ```go
  func(ctx context.Context, req mcp.CallToolRequest, input InputStruct) (*mcp.CallToolResult, OutputStruct, error)
  ```
- **Serialization**: Use `json` struct tags for serialization and `jsonschema:"description=..."` for generating MCP tool schemas.
- **Error Handling**: Wrap errors using `fmt.Errorf("context: %w", err)` and return early.
- **Naming Conventions**: PascalCase for types and exports; camelCase for local variables. Use `Input` and `Output` suffixes for tool I/O structs.

### Configuration
- **Environment Variables**: Primary source for configuration (use `os.Getenv`).
- **YAML Config**: Used by `woo-mcp` (`config.yaml`) and `mcp2mcp-mem-srv` (`tools.yaml`).

---

## 3. Operational Commands

### Build & Test
- **Build a subproject**: `cd <subproject> && go build .`
- **Run a specific test**: `cd <subproject> && go test -run TestName -v ./...`
- **Static Analysis**: `cd <subproject> && go vet ./...`

### Documentation
- Always update `README.md` and `AGENTS.md` after changes are made and accepted.
- Ensure `GEMINI.md` remains the foundational mandate for agent behavior in this workspace.

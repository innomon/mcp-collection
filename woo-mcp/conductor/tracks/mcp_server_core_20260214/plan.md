# Implementation Plan for MCP Server Core

## Track: Implement the core MCP server functionalities, including secure authentication, product discovery, order history, checkout, and issue reporting, to establish a fully functional bridge between an AI Agent and a WooCommerce store.

> **API Corrections from GEMINI.md** (go-sdk v1.3.0 actual API):
> - `mcp.NewMCPServer` → `mcp.NewServer(&mcp.Implementation{...}, nil)`
> - `mcp.ServeStdio(s)` → `s.Connect(ctx, &mcp.StdioTransport{}, nil)`
> - `mcp.WithString("query", mcp.Required())` → typed input structs with `mcp.AddTool`
> - Tool handler signature: `func(ctx, *mcp.CallToolRequest, Input) (*mcp.CallToolResult, Output, error)`

### File Layout (final)
```
woo-mcp/
├── main.go              # Entry point, server wiring
├── main_test.go         # Server integration tests
├── config.go            # YAML config loading
├── config_test.go
├── config.yaml.example
├── auth.go              # JWT/RSA verification + middleware
├── auth_test.go
├── woocommerce.go       # WC REST API client
├── woocommerce_test.go
├── tools.go             # All 4 tool handlers
├── tools_test.go
├── go.mod
└── go.sum
```

---

### Phase 0: Configuration Foundation
> Config must come first — every subsequent component depends on it.

- [x] Task 0.1: Add `gopkg.in/yaml.v3` dependency
    - Run `go get gopkg.in/yaml.v3`.
- [x] Task 0.2: Define `Config` struct and loader
    - [x] Write Failing Tests: For loading config from file path arg, `CONFIG_FILE` env var, and binary-adjacent path. Test missing file error.
    - [x] Implement to Pass Tests: Create `config.go` with `Config` struct (fields: `StoreURL`, `ConsumerKey`, `ConsumerSecret`, `PublicKeyPath`, `ServerName`, `ServerVersion`) and `LoadConfig()` function with 3-tier search order: 1) CLI arg, 2) `CONFIG_FILE` env, 3) same dir as binary.
- [x] Task 0.3: Create `config.yaml.example`
    - Documented template with all configuration fields.
- [ ] Task: Conductor - User Manual Verification 'Configuration Foundation' (Protocol in workflow.md)

### Phase 1: Server Bootstrap & Authentication

- [x] Task 1.1: Create `main.go` with MCP server
    - [x] Write Failing Tests: Update `main_test.go` to use `mcp.NewServer` (fix current broken test using non-existent `mcp.NewMCPServer`). Test server creation and tool registration.
    - [x] Implement to Pass Tests: Create `main.go` using `mcp.NewServer(&mcp.Implementation{...}, nil)`, connect via `&mcp.StdioTransport{}`, load config, block on OS signal.
- [x] Task 1.2: Add `github.com/golang-jwt/jwt/v5` dependency
    - Run `go get github.com/golang-jwt/jwt/v5`.
- [x] Task 1.3: Implement JWT/RSA verification
    - [x] Write Failing Tests: For `VerifyToken` with valid token, expired token, wrong signing method, and malformed token.
    - [x] Implement to Pass Tests: Create `auth.go` with `VerifyToken(tokenStr string, pubKey *rsa.PublicKey) (*jwt.Token, error)` using RS256.
- [x] Task 1.4: Create auth middleware helper
    - [x] Write Failing Tests: For `AuthenticatedHandler` wrapper — valid token passes through, missing/invalid token returns error.
    - [x] Implement to Pass Tests: Add `AuthenticatedHandler` to `auth.go` that extracts JWT from request metadata, verifies, and delegates to inner handler.
- [ ] Task: Conductor - User Manual Verification 'Server Bootstrap & Authentication' (Protocol in workflow.md)

### Phase 2: WooCommerce API Client

- [x] Task 2.1: Implement WooCommerce REST client
    - [x] Write Failing Tests: For `WooClient` methods using `httptest.Server` mocks: `SearchProducts(query)`, `GetOrders(perPage)`, `CreateOrder(items)`, `CreateNote(orderID, note)`. Test success responses and error handling (non-200 status, malformed JSON).
    - [x] Implement to Pass Tests: Create `woocommerce.go` with `WooClient` struct (fields: `baseURL`, `consumerKey`, `consumerSecret`, `*http.Client`). Uses Basic Auth over HTTPS per WooCommerce REST API v3 spec. Endpoints: `GET /wp-json/wc/v3/products`, `GET /wp-json/wc/v3/orders`, `POST /wp-json/wc/v3/orders`, `POST /wp-json/wc/v3/orders/{id}/notes`.
- [ ] Task: Conductor - User Manual Verification 'WooCommerce API Client' (Protocol in workflow.md)

### Phase 3: MCP Tool Handlers

- [x] Task 3.1: Implement `search_products` tool
    - [x] Write Failing Tests: For handler with typed input `SearchInput{Query string}`, verifying it calls `WooClient.SearchProducts` and returns formatted product list.
    - [x] Implement to Pass Tests: Add handler to `tools.go`. Register with `mcp.AddTool` using typed input struct.
- [x] Task 3.2: Implement `get_order_history` tool
    - [x] Write Failing Tests: For handler that calls `WooClient.GetOrders(10)` and maps WooCommerce statuses (`pending`/`processing`/`on-hold` → **Open**, `completed` → **Delivered**, others → **In Process**). Returns formatted list of IDs, statuses, and totals.
    - [x] Implement to Pass Tests: Add handler to `tools.go`.
- [x] Task 3.3: Implement `checkout` tool
    - [x] Write Failing Tests: For handler that creates a pending order via `WooClient.CreateOrder` and returns payment URL in format `{storeURL}/checkout/order-pay/{id}/?pay_for_order=true&key={key}`.
    - [x] Implement to Pass Tests: Add handler to `tools.go`.
- [x] Task 3.4: Implement `raise_issue` tool
    - [x] Write Failing Tests: For handler with typed input `IssueInput{OrderID int, Text string}` that creates an order note via `WooClient.CreateNote` and returns confirmation.
    - [x] Implement to Pass Tests: Add handler to `tools.go`.
- [x] Task 3.5: Register all tools in `main.go`
    - Wire all 4 tools with auth wrapper and `WooClient` instance.
- [ ] Task: Conductor - User Manual Verification 'MCP Tool Handlers' (Protocol in workflow.md)

### Phase 4: Polish & Hardening

- [x] Task 4.1: Add structured logging
    - Using `log` package in `main.go` for server lifecycle logging.
- [x] Task 4.2: Implement graceful shutdown
    - Handle `SIGINT`/`SIGTERM` via `signal.NotifyContext` in `main.go`, close server session cleanly.
- [x] Task 4.3: Coverage check
    - 72.2% total (main.go at 0% is untestable entry point; library code >80%).
- [ ] Task: Conductor - User Manual Verification 'Polish & Hardening' (Protocol in workflow.md)

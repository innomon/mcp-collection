---
name: developing-woo-mcp
description: "Develops the woo-mcp WooCommerce MCP+UCP server. Use when adding features, fixing bugs, or modifying code in the woo-mcp subproject — including MCP tools, UCP endpoints, REST API, OAuth, A2UI cards, or config."
---

# Developing woo-mcp

WooCommerce MCP+UCP server — a single-binary Go server exposing WooCommerce capabilities via MCP (for AI agents), REST (for platform integration), and A2UI (for rich UI cards).

## Architecture

```
woo-mcp/
├── main.go              # Entrypoint: config load, server init, transport setup
├── config.go            # Config struct + YAML loader (config.yaml)
├── woocommerce.go       # WooClient: HTTP client for WC REST API (wp-json/wc/v3/*)
├── tools.go             # MCP tool registration (legacy + UCP dispatch)
├── auth.go              # JWT verification + API key middleware
├── ucp.go               # UCP types (CheckoutSession, UCPProduct, etc.)
├── ucp_discovery.go     # Discovery tools: search_shop_catalog, get_product, etc.
├── ucp_checkout.go      # Checkout tools: create/get/update/complete/cancel checkout
├── ucp_order.go         # Order tools: get_order, list_orders
├── ucp_profile.go       # GET /.well-known/ucp profile endpoint
├── rest.go              # REST endpoints (/ucp/v1/*) + route registration
├── oauth.go             # OAuth 2.0 server (authorize, token, revoke)
├── a2ui.go              # A2UI surface builder (components, data model, rendering)
├── a2ui_cards.go        # Commerce cards: ProductCard, CheckoutCard, OrderCard
├── config.yaml.example  # Reference config
└── SPEC.md              # Full specification & implementation plan
```

## Key Concepts

### Two Operational Modes

| Mode | Config | Behavior |
|------|--------|----------|
| Customer-facing (default) | `super_user: false` | OAuth 2.0 identity linking enabled |
| Super user | `super_user: true` | OAuth disabled; `api_keys` secure REST; full merchant access |

### Transports

- `stdio` — local AI tool use (default)
- `http` — HTTP-only (Streamable HTTP MCP at `/ucp/mcp` + REST endpoints)
- `both` — stdio + HTTP simultaneously

### Feature Flags (config.yaml)

- `ucp_enabled` — registers UCP discovery/checkout/order tools
- `a2ui_enabled` — attaches A2UI card JSONL to tool responses
- `super_user` — trusted backend mode
- `oauth_clients` — enables OAuth 2.0 endpoints

## MCP Tools

### Legacy Tools (always registered)
- `search_products` — basic product search
- `get_order_history` — recent orders list
- `checkout` — create order + payment link
- `raise_issue` — add note to an order

### UCP Tools (when `ucp_enabled: true`)

**Discovery:** `search_shop_catalog`, `get_product`, `get_product_categories`, `search_shop_policies_and_faqs`

**Checkout:** `create_checkout`, `get_checkout`, `update_checkout`, `complete_checkout`, `cancel_checkout`

**Order:** `get_order`, `list_orders`

## REST Endpoints (HTTP transport)

- `GET /.well-known/ucp` — UCP business profile
- `POST /ucp/v1/checkout-sessions` — create checkout
- `GET /ucp/v1/checkout-sessions/{id}` — get checkout
- `PATCH /ucp/v1/checkout-sessions/{id}` — update checkout
- `POST /ucp/v1/checkout-sessions/{id}/complete` — complete checkout
- `POST /ucp/v1/checkout-sessions/{id}/cancel` — cancel checkout
- `GET /ucp/v1/orders/{id}` — get order
- `GET /ucp/v1/products` — search products
- `GET /ucp/v1/products/{id}` — get product

## OAuth 2.0 Endpoints (customer-facing mode + `oauth_clients` configured)

- `GET /.well-known/oauth-authorization-server` — RFC 8414 metadata
- `GET /oauth2/authorize` — HTML login form
- `POST /oauth2/authorize` — validate credentials, issue auth code
- `POST /oauth2/token` — authorization_code + refresh_token grants
- `POST /oauth2/revoke` — RFC 7009 with cascade

## Code Conventions

- All code in `package main`, single binary
- Tool registration: `mcp.AddTool(server, &mcp.Tool{...}, handlerFunc)` with inline `json.RawMessage` schemas
- UCP types defined in `ucp.go` with `json` struct tags
- Prices: WC uses decimal strings (`"19.99"`), UCP uses minor units/cents (`1999`). Convert at boundary with `wcPriceToCents` / `centsToWcPrice`
- Errors: `fmt.Errorf("context: %w", err)`, return early
- A2UI cards: build via `Surface` struct, render as `mcp.EmbeddedResource` with `mimeType: "application/json+a2ui"`
- Config loaded from `config.yaml` via `gopkg.in/yaml.v3`
- In-memory OAuth token storage (maps guarded by `sync.Mutex`)
- WP credential validation via `GET /wp-json/wp/v2/users/me` with Basic Auth
- WC customer lookup via `GET /wc/v3/customers?email=`

## Build & Test

```sh
cd woo-mcp && go build .
cd woo-mcp && go test -v ./...
cd woo-mcp && go vet ./...
```

## Key References

- `SPEC.md` — full specification with UCP mappings, status lifecycle, error codes
- `config.yaml.example` — all config fields with comments
- [UCP Specification](https://ucp.dev/specification/overview/)
- [WooCommerce REST API](https://woocommerce.github.io/woocommerce-rest-api-docs/)
- [A2UI Specification](https://a2ui.org/)

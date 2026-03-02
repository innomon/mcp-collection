# WooCommerce MCP+UCP Server — Specification & Implementation Plan

## 1. Overview

Transform `woo-mcp` from a basic MCP tool server into a **UCP-compliant commerce server** that exposes WooCommerce capabilities via both **MCP** (for AI agents) and **REST** (for direct platform integration), with **A2UI card rendering** for rich UI output.

### Architecture Summary

```
┌──────────────────────────────────────────────────────────────┐
│                      woo-mcp server                          │
│                                                              │
│  ┌────────────┐  ┌────────────┐  ┌────────────────────────┐  │
│  │ MCP Tools  │  │ REST/UCP   │  │ UCP Profile            │  │
│  │ (JSON-RPC) │  │ Endpoints  │  │ /.well-known/ucp       │  │
│  └─────┬──────┘  └─────┬──────┘  └────────────────────────┘  │
│        │               │                                     │
│  ┌─────▼───────────────▼──────┐                              │
│  │    Capability Handlers     │                              │
│  │  ┌─────────┐ ┌──────────┐  │                              │
│  │  │Checkout │ │Discovery │  │                              │
│  │  ├─────────┤ ├──────────┤  │                              │
│  │  │ Order   │ │Policies  │  │                              │
│  │  ├─────────┤ ├──────────┤  │                              │
│  │  │Identity │ │ A2UI     │  │                              │
│  │  └─────────┘ └──────────┘  │                              │
│  └─────────────┬──────────────┘                              │
│                │                                             │
│  ┌─────────────▼──────────────┐                              │
│  │   WooCommerce REST Client  │                              │
│  │   (wp-json/wc/v3/*)        │                              │
│  └────────────────────────────┘                              │
└──────────────────────────────────────────────────────────────┘
         │
         ▼
   WooCommerce Store (WordPress + WooCommerce plugin)
```

### Actors (UCP Roles)

| UCP Role | In This System |
|----------|---------------|
| **Business** | WooCommerce store (the merchant) — woo-mcp acts as the business server |
| **Platform** | AI agent / shopping assistant / app that connects via MCP or REST |
| **Credential Provider** | WooCommerce payment gateways (Stripe, PayPal, etc.) |
| **PSP** | Payment gateway processors configured in WooCommerce |

---

## 2. UCP Capabilities

### 2.1 Discovery (`dev.ucp.shopping.discovery` — custom extension)

Not yet in the UCP standard, but mirrors Shopify Storefront MCP. Enables product catalog search and store policy queries.

**MCP Tools:**

| Tool | Description | WooCommerce API |
|------|-------------|-----------------|
| `search_shop_catalog` | Search products by query + context | `GET /wc/v3/products?search=` |
| `get_product` | Get single product details + variants | `GET /wc/v3/products/{id}` |
| `get_product_categories` | List product categories | `GET /wc/v3/products/categories` |
| `search_shop_policies_and_faqs` | Query store policies | WordPress pages + WC settings |

**Input/Output schemas:**

```
search_shop_catalog:
  Input:
    query: string (required) — search terms
    context: string (required) — buyer context for ranking
    category: string (optional) — filter by category slug
    min_price: number (optional)
    max_price: number (optional)
    per_page: number (optional, default 10)
  Output:
    products[]:
      id: string
      title: string
      price: integer (cents)
      currency: string (ISO 4217)
      image_url: string
      url: string
      description: string
      variant_id: string (first/default variant)
      in_stock: boolean

get_product:
  Input:
    id: string (required)
  Output:
    id, title, price, currency, image_url, url, description,
    variants[]: { id, title, price, in_stock, attributes }
    categories[]: { id, name }
```

### 2.2 Checkout (`dev.ucp.shopping.checkout`)

Full UCP Checkout lifecycle backed by WooCommerce orders.

**MCP Tools (per UCP MCP Binding):**

| Tool | Description | WooCommerce API |
|------|-------------|-----------------|
| `create_checkout` | Create checkout session → WC pending order | `POST /wc/v3/orders` |
| `get_checkout` | Retrieve checkout state | `GET /wc/v3/orders/{id}` |
| `update_checkout` | Update line items, buyer, fulfillment | `PUT /wc/v3/orders/{id}` |
| `complete_checkout` | Finalize → redirect to payment | `PUT /wc/v3/orders/{id}` + payment URL |
| `cancel_checkout` | Cancel checkout session | `PUT /wc/v3/orders/{id}` status=cancelled |

**Checkout Session Lifecycle Mapping:**

| UCP Status | WooCommerce Order Status |
|------------|------------------------|
| `incomplete` | `pending` (no payment info yet) |
| `requires_escalation` | `pending` + `continue_url` set (needs browser payment) |
| `ready_for_complete` | `pending` with all info collected |
| `complete_in_progress` | `on-hold` (payment processing) |
| `completed` | `processing` or `completed` |
| `canceled` | `cancelled` |

**Key Design Decisions:**
- WooCommerce does not support tokenized payment via API for most gateways → checkout will always go through `requires_escalation` with a `continue_url` pointing to WC payment page
- The `continue_url` format: `{store_url}/checkout/order-pay/{order_id}/?pay_for_order=true&key={order_key}`
- Line items map 1:1 between UCP and WC `line_items[]`
- Amounts in UCP are minor units (cents); WC uses decimal strings → convert at boundary

### 2.3 Order (`dev.ucp.shopping.order`)

**MCP Tools:**

| Tool | Description | WooCommerce API |
|------|-------------|-----------------|
| `get_order` | Get order with fulfillment + adjustments | `GET /wc/v3/orders/{id}` + `/notes` + `/refunds` |
| `list_orders` | List buyer's orders | `GET /wc/v3/orders?customer=` |

**WooCommerce → UCP Order Mapping:**

| UCP Field | WooCommerce Source |
|-----------|-------------------|
| `id` | `order.id` (string) |
| `checkout_id` | Same as `id` (WC order IS the checkout) |
| `permalink_url` | `order.order_received_url` or constructed |
| `line_items[]` | `order.line_items[]` |
| `fulfillment.expectations[]` | Derived from `order.shipping_lines[]` + status |
| `fulfillment.events[]` | `order.meta_data` tracking fields + order notes |
| `adjustments[]` | `order.refunds[]` mapped to UCP adjustments |
| `totals[]` | Computed from `order.total`, `order.shipping_total`, `order.total_tax`, `order.discount_total` |

### 2.4 Fulfillment Extension (`dev.ucp.shopping.fulfillment`)

Extends checkout with shipping methods. Maps WC shipping zones/methods.

**Mapping:**
- `fulfillment.methods[].type` → always `"shipping"` for physical goods
- `fulfillment.methods[].groups[].options[]` → WC shipping methods available
- WC's shipping calculation requires address → fulfillment options populated after buyer address is set

### 2.5 Identity Linking (`dev.ucp.common.identity_linking`)

OAuth 2.0 flow for linking platform users to WooCommerce customer accounts.

**Implementation:**
- WooCommerce doesn't natively support OAuth 2.0 for customers → implement via WordPress REST Application Passwords or a custom OAuth plugin bridge
- **Phase 1:** Skip identity linking; use guest checkout
- **Phase 2:** Implement OAuth 2.0 endpoints backed by WP user system
  - `/.well-known/oauth-authorization-server` metadata endpoint
  - `/oauth2/authorize` — redirect to WP login + consent
  - `/oauth2/token` — exchange code for access token
  - `/oauth2/revoke` — revoke token
  - Scopes: `ucp:scopes:checkout_session`

---

## 3. UCP Profile & Discovery

### 3.1 Business Profile (`/.well-known/ucp`)

The server exposes a UCP profile JSON document:

```json
{
  "ucp": {
    "version": "2026-01-11",
    "services": {
      "dev.ucp.shopping": {
        "version": "2026-01-11",
        "spec": "https://ucp.dev/specification/overview",
        "mcp": {
          "schema": "https://ucp.dev/services/shopping/mcp.openrpc.json",
          "endpoint": "{store_url}/ucp/mcp"
        },
        "rest": {
          "schema": "https://ucp.dev/services/shopping/rest.openapi.json",
          "endpoint": "{store_url}/ucp/v1"
        }
      }
    },
    "capabilities": [
      {
        "name": "dev.ucp.shopping.checkout",
        "version": "2026-01-11",
        "spec": "https://ucp.dev/specification/checkout",
        "schema": "https://ucp.dev/schemas/shopping/checkout.json"
      },
      {
        "name": "dev.ucp.shopping.fulfillment",
        "version": "2026-01-11",
        "spec": "https://ucp.dev/specification/fulfillment",
        "schema": "https://ucp.dev/schemas/shopping/fulfillment.json",
        "extends": "dev.ucp.shopping.checkout"
      },
      {
        "name": "dev.ucp.shopping.order",
        "version": "2026-01-11",
        "spec": "https://ucp.dev/specification/order",
        "schema": "https://ucp.dev/schemas/shopping/order.json"
      }
    ]
  },
  "payment": {
    "handlers": [
      {
        "id": "wc_payment_redirect",
        "name": "com.woocommerce.payment_redirect",
        "version": "2026-01-11",
        "spec": "https://woocommerce.com/docs/payment",
        "config": {
          "type": "REDIRECT",
          "note": "WooCommerce handles payment via browser redirect to store checkout page"
        }
      }
    ]
  }
}
```

### 3.2 Platform Profile Validation

On every MCP tool call, extract `_meta.ucp.profile` and validate capability intersection. If absent, operate in legacy MCP-only mode (backward compatible with existing tools).

---

## 4. Transport Layer

### 4.1 MCP Transport (Primary — stdio + Streamable HTTP)

- **stdio**: Existing transport for local AI tool use (Claude, Cursor, etc.)
- **Streamable HTTP**: New transport for remote connections at `{store_url}/ucp/mcp`
  - Uses `github.com/modelcontextprotocol/go-sdk` Streamable HTTP server
  - JSON-RPC 2.0 over HTTP POST with SSE streaming responses

### 4.2 REST Transport (Secondary)

RESTful HTTP endpoints for direct platform integration:

| Method | Path | UCP Operation |
|--------|------|---------------|
| `POST` | `/ucp/v1/checkout-sessions` | Create Checkout |
| `GET` | `/ucp/v1/checkout-sessions/{id}` | Get Checkout |
| `PATCH` | `/ucp/v1/checkout-sessions/{id}` | Update Checkout |
| `POST` | `/ucp/v1/checkout-sessions/{id}/complete` | Complete Checkout |
| `POST` | `/ucp/v1/checkout-sessions/{id}/cancel` | Cancel Checkout |
| `GET` | `/ucp/v1/orders/{id}` | Get Order |
| `GET` | `/ucp/v1/products` | Search Products |
| `GET` | `/ucp/v1/products/{id}` | Get Product |

Headers:
- `UCP-Agent: profile="https://..."` — platform profile URI
- `Content-Type: application/json`

### 4.3 Config Updates

```yaml
# config.yaml additions
transport: "stdio"           # "stdio" | "http" | "both"
http_port: 8080              # port for HTTP transport
ucp_enabled: true            # enable UCP profile + REST endpoints
a2ui_enabled: true           # enable A2UI card generation in tool responses
store_policies_page_ids: []  # WordPress page IDs containing store policies
```

---

## 5. A2UI Card Rendering

### 5.1 Integration Approach

A2UI cards are returned as **structured content** within MCP tool responses. When `a2ui_enabled` is true, tool responses include both:
1. Plain text summary (backward compatible)
2. A2UI JSONL payload in an `a2ui` annotation or embedded content block

### 5.2 Commerce Card Schemas

#### Product Card

```jsonl
{"surfaceUpdate":{"surfaceId":"product-{id}","components":[
  {"id":"root","component":{"Card":{"child":"body"}}},
  {"id":"body","component":{"Column":{"children":{"explicitList":["img","name","price-row","desc","actions"]}}}},
  {"id":"img","component":{"Image":{"url":{"path":"/product/imageUrl"},"fit":"cover","usageHint":"mediumFeature"}}},
  {"id":"name","component":{"Text":{"text":{"path":"/product/name"},"usageHint":"h3"}}},
  {"id":"price-row","component":{"Row":{"children":{"explicitList":["price","stock"]},"distribution":"spaceBetween","alignment":"center"}}},
  {"id":"price","component":{"Text":{"text":{"path":"/product/formattedPrice"},"usageHint":"h4"}}},
  {"id":"stock","component":{"Text":{"text":{"path":"/product/stockStatus"},"usageHint":"caption"}}},
  {"id":"desc","component":{"Text":{"text":{"path":"/product/description"},"usageHint":"body"}}},
  {"id":"actions","component":{"Row":{"children":{"explicitList":["add-btn"]},"distribution":"end"}}},
  {"id":"add-btn-label","component":{"Text":{"text":{"literalString":"Add to Cart"}}}},
  {"id":"add-btn","component":{"Button":{"child":"add-btn-label","primary":true,"action":{"name":"add_to_cart","context":[{"key":"productId","value":{"path":"/product/id"}},{"key":"quantity","value":{"literalNumber":1}}]}}}}
]}}
{"dataModelUpdate":{"surfaceId":"product-{id}","contents":[{"key":"product","valueMap":[
  {"key":"id","valueString":"{id}"},
  {"key":"name","valueString":"{name}"},
  {"key":"formattedPrice","valueString":"${price}"},
  {"key":"imageUrl","valueString":"{image_url}"},
  {"key":"description","valueString":"{description}"},
  {"key":"stockStatus","valueString":"{stock_status}"}
]}]}}
{"beginRendering":{"surfaceId":"product-{id}","root":"root"}}
```

#### Product List Card (search results)

Uses `template` children to render a list of products from search results.

#### Cart/Checkout Card

Shows line items, totals, buyer info, and action buttons (update/complete/cancel).

#### Order Card

Shows order status, line items, fulfillment tracking, and adjustment history.

### 5.3 A2UI Builder (Go Package)

Reuse patterns from `mcp-frappe` A2UI pipeline. Create an internal `a2ui` package with:

```go
// a2ui.go — builder helpers
type Surface struct { ... }
func NewSurface(id string) *Surface
func (s *Surface) AddComponent(id string, component any) *Surface
func (s *Surface) SetData(path string, data map[string]any) *Surface
func (s *Surface) Render(rootID string) []json.RawMessage  // returns JSONL lines

// cards.go — commerce card constructors
func ProductCard(p Product) []json.RawMessage
func ProductListCard(products []Product) []json.RawMessage
func CheckoutCard(checkout UCPCheckout) []json.RawMessage
func OrderCard(order UCPOrder) []json.RawMessage
```

---

## 6. File Structure

```
woo-mcp/
├── main.go                 # entry point, transport setup
├── config.go               # config loading (extended)
├── config.yaml.example     # updated example
├── auth.go                 # JWT auth (existing)
│
├── woocommerce.go          # WooCommerce REST client (extended)
│
├── ucp.go                  # UCP types: Checkout, Order, LineItem, Total, etc.
├── ucp_profile.go          # /.well-known/ucp profile handler
├── ucp_checkout.go         # Checkout capability handlers + WC↔UCP mapping
├── ucp_order.go            # Order capability handlers + WC↔UCP mapping
├── ucp_discovery.go        # Product search + policies handlers
│
├── tools.go                # MCP tool registration (refactored)
├── rest.go                 # REST transport router + handlers
│
├── a2ui.go                 # A2UI builder helpers
├── a2ui_cards.go           # Commerce card constructors
│
├── *_test.go               # Tests for each module
├── go.mod
└── go.sum
```

---

## 7. Implementation Plan — Phased Checklist

### Phase 1: Foundation — UCP Types & Enhanced Discovery
_Goal: UCP data model + richer product search matching Shopify Storefront MCP_

- [x] **1.1** Define UCP core types in `ucp.go`
  - [x] `UCPEnvelope` (version, capabilities array)
  - [x] `UCPCheckout` (id, status, line_items, buyer, currency, totals, links, continue_url, payment, fulfillment, order)
  - [x] `UCPLineItem` (id, item, quantity, totals, parent_id)
  - [x] `UCPItem` (id, title, price, image_url)
  - [x] `UCPTotal` (type enum, display_text, amount)
  - [x] `UCPBuyer` (email, first_name, last_name)
  - [x] `UCPOrder` (id, checkout_id, permalink_url, line_items, fulfillment, adjustments, totals)
  - [x] `UCPFulfillment` (methods/expectations/events)
  - [x] `UCPAdjustment` (id, type, occurred_at, status, line_items, amount, description)
  - [x] `UCPPayment` (handlers, instruments)
  - [x] `UCPLink` (type, url)
  - [x] `UCPMessage` (code, message, severity)
- [x] **1.2** Extend `WooClient` in `woocommerce.go`
  - [x] `GetProduct(ctx, id)` — single product with variants + images
  - [x] `GetProductCategories(ctx)` — list categories
  - [x] `SearchProducts(ctx, SearchParams)` — with category, price range, pagination
  - [ ] `GetShippingZones(ctx)` — for fulfillment options
  - [x] `GetOrderRefunds(ctx, orderID)` — for adjustments
  - [x] `GetOrderNotes(ctx, orderID)` — for fulfillment events
  - [x] `UpdateOrder(ctx, id, payload)` — update existing order
  - [x] `GetStoreSettings(ctx)` — currency, tax settings
  - [x] Price conversion helpers: WC decimal string ↔ UCP minor units (cents)
- [x] **1.3** Implement `ucp_discovery.go`
  - [x] `search_shop_catalog` tool with context-aware search
  - [x] `get_product` tool with full variant details
  - [x] `get_product_categories` tool
  - [x] `search_shop_policies_and_faqs` tool (query WP pages)
- [x] **1.4** Update config
  - [x] Add `transport`, `http_port`, `ucp_enabled`, `a2ui_enabled` fields
  - [x] Update `config.yaml.example`
- [x] **1.5** Refactor `tools.go` → register both legacy and UCP tools
- [x] **1.6** Tests for Phase 1
  - [x] Unit tests for UCP type serialization
  - [x] Unit tests for WC↔UCP price conversion
  - [x] Unit tests for discovery tools

### Phase 2: Checkout Capability
_Goal: Full UCP Checkout lifecycle via MCP_

- [x] **2.1** Implement `ucp_checkout.go`
  - [x] `create_checkout` — create WC order (pending) → return UCPCheckout
  - [x] `get_checkout` — fetch WC order → return UCPCheckout
  - [x] `update_checkout` — update WC order line items/buyer/fulfillment → return UCPCheckout
  - [x] `complete_checkout` — set WC order status + return `continue_url` for payment
  - [x] `cancel_checkout` — cancel WC order → return UCPCheckout
  - [x] WC Order → UCPCheckout mapping function
  - [x] UCPCheckout status derivation from WC order state
  - [x] `continue_url` construction for payment redirect
  - [ ] Idempotency key handling (store in order meta)
  - [ ] `_meta.ucp.profile` extraction and validation
- [x] **2.2** UCP response envelope
  - [ ] Every response includes `ucp.version` + `ucp.capabilities[]`
  - [ ] Every checkout response includes `links[]` (privacy policy, TOS from WC settings)
  - [x] `totals[]` computed from WC: subtotal, discount, fulfillment, tax, total
  - [x] `payment.handlers[]` with redirect handler config
- [x] **2.3** Register MCP tools in `tools.go`
  - [x] `create_checkout`, `get_checkout`, `update_checkout`, `complete_checkout`, `cancel_checkout`
  - [ ] Input validation against UCP schemas
  - [ ] Error responses with UCP error codes (`MERCHANDISE_NOT_AVAILABLE`, etc.)
- [x] **2.4** Tests for Phase 2
  - [x] Checkout lifecycle integration tests (create → update → complete)
  - [x] Status mapping tests
  - [ ] Error handling tests

### Phase 3: Order Capability
_Goal: UCP Order with fulfillment tracking and adjustments_

- [x] **3.1** Implement `ucp_order.go`
  - [x] `get_order` tool — WC order + refunds + notes → UCPOrder
  - [x] `list_orders` tool — list with pagination
  - [x] WC Order → UCPOrder mapping
    - [x] `line_items[]` with quantity.total/fulfilled derivation
    - [x] `fulfillment.expectations[]` from shipping lines
    - [x] `fulfillment.events[]` from order notes (tracking info)
    - [x] `adjustments[]` from WC refunds
    - [x] `totals[]` (subtotal, shipping, tax, discount, total)
    - [x] Line item status derivation (processing/partial/fulfilled)
  - [x] `permalink_url` construction
- [ ] **3.2** Order event webhook (outbound)
  - [ ] WooCommerce webhook listener → transform to UCP order event
  - [ ] POST to platform's `webhook_url` (from capability config)
  - [ ] Request-Signature header (detached JWT, RFC 7797)
  - [ ] Retry logic for failed deliveries
- [x] **3.3** Tests for Phase 3

### Phase 4: A2UI Card Rendering
_Goal: Rich UI cards in tool responses_

- [x] **4.1** Implement `a2ui.go` — builder helpers
  - [x] `Surface` struct with component adjacency list
  - [x] `AddComponent(id, componentType, props)` builder
  - [x] `SetData(path, contents)` data model builder
  - [x] `Render(rootID, catalogID)` → JSONL output (surfaceUpdate + dataModelUpdate + beginRendering)
  - [x] `BoundValue` helpers: `Literal(v)`, `Path(p)`, `LiteralAndPath(v, p)`
- [x] **4.2** Implement `a2ui_cards.go` — commerce cards
  - [x] `ProductCard(product)` — image, title, price, stock, description, add-to-cart button
  - [x] `ProductListCard(products)` — scrollable list with template binding
  - [x] `CheckoutCard(checkout)` — line items, totals, buyer info, action buttons
  - [x] `OrderCard(order)` — status, items, fulfillment timeline, adjustments
  - [ ] `OrderListCard(orders)` — list of order summaries
- [x] **4.3** Integrate A2UI into tool responses
  - [x] When `a2ui_enabled`, attach A2UI JSONL as additional content in `CallToolResult`
  - [x] Embed as `mcp.EmbeddedResource` with `mimeType: "application/json+a2ui"`
  - [x] Maintain backward compat: plain text content always present
- [x] **4.4** Tests for Phase 4

### Phase 5: HTTP Transport & REST Endpoints
_Goal: Streamable HTTP MCP + UCP REST API_

- [x] **5.1** HTTP server setup in `main.go`
  - [x] Conditional HTTP listener based on `transport` config
  - [x] Route multiplexer (stdlib `net/http`)
- [x] **5.2** MCP Streamable HTTP transport
  - [x] Mount at `/ucp/mcp`
  - [x] Use go-sdk's `StreamableHTTPTransport`
- [x] **5.3** UCP Profile endpoint
  - [x] `GET /.well-known/ucp` → profile JSON
  - [ ] Dynamic capability list from config
- [x] **5.4** REST endpoints in `rest.go`
  - [x] `POST /ucp/v1/checkout-sessions` → calls checkout handler
  - [x] `GET /ucp/v1/checkout-sessions/{id}`
  - [x] `PATCH /ucp/v1/checkout-sessions/{id}`
  - [x] `POST /ucp/v1/checkout-sessions/{id}/complete`
  - [x] `POST /ucp/v1/checkout-sessions/{id}/cancel`
  - [x] `GET /ucp/v1/orders/{id}`
  - [x] `GET /ucp/v1/products?query=&category=&min_price=&max_price=`
  - [x] `GET /ucp/v1/products/{id}`
  - [ ] `UCP-Agent` header parsing
  - [x] JSON error responses matching UCP error schema
- [x] **5.5** Tests for Phase 5

### Phase 6: Identity Linking
_Goal: OAuth 2.0 customer account linking_

- [x] **6.1** OAuth 2.0 server endpoints
  - [x] `GET /.well-known/oauth-authorization-server` — metadata (RFC 8414)
  - [x] `GET /oauth2/authorize` — renders HTML login form for credential collection
  - [x] `POST /oauth2/authorize` — validates credentials against WordPress, issues auth code
  - [x] `POST /oauth2/token` — token exchange (authorization_code + refresh_token grants)
  - [x] `POST /oauth2/revoke` — token revocation (RFC 7009, cascade on refresh)
- [x] **6.2** WordPress user integration
  - [x] Config-based OAuth client registration (`oauth_clients` in config.yaml)
  - [x] In-memory token storage (auth codes, access tokens, refresh tokens)
  - [x] Scope validation: `ucp:scopes:checkout_session`
  - [x] WP credential validation via `AuthenticateWPUser` (`GET /wp-json/wp/v2/users/me` with Basic Auth)
  - [x] WC customer lookup via `GetCustomerByEmail` (`GET /wc/v3/customers?email=`)
  - [x] `resolveCustomerID` replaces placeholder `ResolveCustomerID`
  - [ ] Persistent token storage (WP options or custom DB table) — future
- [x] **6.3** Authenticated checkout
  - [x] Bearer token extraction from REST `Authorization` header
  - [x] Pre-fill buyer email from linked account on `create_checkout`
  - [ ] Scope enforcement per checkout operation — future
  - [ ] Access order history via linked identity — future
- [x] **6.4** Tests for Phase 6
  - [x] Metadata endpoint returns valid RFC 8414 JSON
  - [x] Login form rendered on GET /oauth2/authorize
  - [x] Auth code flow: login form → POST credentials → token exchange → validate
  - [x] Invalid password re-renders login form with error
  - [x] Missing credentials re-renders login form with error
  - [x] Customer ID resolved from WC API (not hardcoded 0)
  - [x] Invalid client, redirect URI, response type rejection
  - [x] Code reuse prevention
  - [x] Refresh token grant + old refresh revocation
  - [x] Access token revocation
  - [x] Refresh token cascade revocation
  - [x] Unknown token revocation (RFC 7009 compliance)
  - [x] Full lifecycle integration test
  - [x] Bearer token extraction + authenticated checkout pre-fill
  - [x] UCP profile includes identity_linking capability

#### Phase 6 Implementation Notes

**Problem:** UCP's `dev.ucp.common.identity_linking` capability requires a business server to act as an OAuth 2.0 Authorization Server (RFC 6749) so that platforms can link their users' accounts to the merchant's customer records. WooCommerce has no native OAuth 2.0 flow for customer-facing authentication — its built-in REST API auth is merchant-level (consumer key/secret), and WordPress login is session/cookie-based with no standard token endpoint.

**Design Choices & Rationale:**

1. **Self-contained OAuth 2.0 server (`oauth.go`)** — Rather than depending on an external WordPress plugin or reverse-proxying to WP login, the OAuth server is embedded directly in `woo-mcp`. This keeps the deployment single-binary and avoids coupling to WordPress plugin ecosystems that may break across WP versions. The `/oauth2/authorize` endpoint serves an HTML login form (GET) that collects email and password, then validates credentials against WordPress via `GET /wp-json/wp/v2/users/me` with HTTP Basic Auth (POST). This works with WordPress Application Passwords (WP 5.6+) or plugins that enable Basic Auth for standard credentials. After successful authentication, the WC customer ID is resolved via `GET /wc/v3/customers?email=`.

2. **In-memory token storage** — Auth codes, access tokens, and refresh tokens are stored in `sync.Mutex`-guarded maps. This is deliberately minimal: the goal is a correct, testable OAuth 2.0 implementation that can be swapped to a persistent backend (PostgreSQL, WP options table, Redis) when production-readiness demands it. For a single-instance MCP server handling agent commerce flows, in-memory state is sufficient and avoids adding a database dependency.

3. **Static client registration via `config.yaml`** — OAuth clients are declared in config (`oauth_clients[]` with `client_id`, `client_secret`, `redirect_uris`). This follows the pattern of pre-registered platform credentials typical in UCP business↔platform relationships, where a merchant explicitly onboards each platform. Dynamic Client Registration (RFC 7591) is deferred.

4. **Authorization Code flow only** — Per UCP spec requirements, only `response_type=code` with `client_secret_basic` token endpoint auth is supported. Implicit and client credentials grants are excluded. Refresh tokens use rotation (old refresh token is revoked on use) to limit token leakage risk.

5. **RFC 7009 revocation with cascade** — Revoking a refresh token also revokes all access tokens for that client+customer pair, matching the UCP spec's MUST requirement for recursive revocation.

6. **Buyer pre-fill on authenticated checkout** — When a REST request to `POST /ucp/v1/checkout-sessions` carries a valid `Authorization: Bearer` header, the server resolves the token to the linked customer email and pre-fills the checkout's buyer info. This demonstrates the identity linking value prop without requiring the full scope enforcement and order history scoping (deferred to future work).

**Use Case — Agent Commerce Flow:**

```
Platform (AI Agent)              woo-mcp                    WordPress/WooCommerce
       │                            │                            │
       │  GET /.well-known/         │                            │
       │  oauth-authorization-server│                            │
       │◄──────────────────────────►│  (metadata discovery)      │
       │                            │                            │
       │  GET /oauth2/authorize     │                            │
       │  ?client_id=...&scope=...  │                            │
       │◄────── 200 HTML form ─────│  (login form rendered)     │
       │                            │                            │
       │  POST /oauth2/authorize    │                            │
       │  email=...&password=...    │  GET /wp-json/wp/v2/       │
       │                            │  users/me (Basic Auth)     │
       │                            │◄──────────────────────────►│
       │                            │  (validate credentials)    │
       │                            │  GET /wc/v3/customers      │
       │                            │  ?email=...                │
       │                            │◄──────────────────────────►│
       │                            │  (resolve customer ID)     │
       │◄───── 302 redirect ───────│  (issue auth code)         │
       │                            │                            │
       │  POST /oauth2/token        │                            │
       │  grant_type=auth_code      │                            │
       │◄──────────────────────────►│  (issue access+refresh)    │
       │                            │                            │
       │  POST /ucp/v1/checkout-    │                            │
       │  sessions                  │                            │
       │  Authorization: Bearer AT  │  POST /wc/v3/orders        │
       │◄──────────────────────────►│◄──────────────────────────►│
       │  (buyer email pre-filled)  │  (order created w/ billing)│
```

The platform discovers OAuth metadata, redirects the buyer to the login form, the buyer authenticates with their WordPress credentials (Application Passwords supported), and the server issues tokens. The buyer's identity flows through to checkout calls without the platform needing to separately collect billing info.

**Future Work:**
- Persistent token storage for multi-instance deployments
- Scope enforcement: gate each checkout/order operation on the token's granted scopes
- Order history scoping: filter `list_orders` to the authenticated customer only

---

## 8. WooCommerce ↔ UCP Mapping Reference

### Price Conversion

```
WooCommerce: "19.99" (string, decimal)
UCP:         1999    (integer, minor units / cents)

wcPriceToCents("19.99") → 1999
centsToWcPrice(1999)    → "19.99"
```

### Order Status Mapping

| WooCommerce | UCP Checkout Status | UCP Order Line Item Status |
|-------------|--------------------|-----------------------------|
| `pending` | `incomplete` / `requires_escalation` | — |
| `on-hold` | `complete_in_progress` | `processing` |
| `processing` | `completed` | `processing` |
| `completed` | `completed` | `fulfilled` |
| `cancelled` | `canceled` | — |
| `refunded` | — | varies by adjustment |
| `failed` | — (error) | — |

### Totals Mapping

| UCP Total Type | WooCommerce Source |
|----------------|-------------------|
| `subtotal` | Sum of `line_items[].subtotal` |
| `discount` | `discount_total` (negated) |
| `fulfillment` | `shipping_total` |
| `tax` | `total_tax` |
| `total` | `total` |

---

## 9. Error Handling

UCP error codes mapped to WooCommerce scenarios:

| UCP Error Code | Scenario | Severity |
|---------------|----------|----------|
| `MERCHANDISE_NOT_AVAILABLE` | Product out of stock | `requires_buyer_input` |
| `INVALID_LINE_ITEM` | Invalid product ID | `recoverable` |
| `CHECKOUT_EXPIRED` | Order expired (>6h) | `requires_buyer_input` |
| `PAYMENT_REQUIRED` | Payment not yet submitted | `requires_buyer_review` |
| `FULFILLMENT_ADDRESS_REQUIRED` | Shipping address missing | `recoverable` |

---

## 10. Testing Strategy

- **Unit tests**: Type mapping, price conversion, A2UI card generation
- **Integration tests**: Full checkout lifecycle with mocked WooCommerce API
- **Build verification**: `go build .` and `go vet ./...` pass
- **Test command**: `cd woo-mcp && go test -v ./...`

---

## 11. Dependencies

Current:
- `github.com/modelcontextprotocol/go-sdk v1.3.0`
- `github.com/golang-jwt/jwt/v5`
- `gopkg.in/yaml.v3`

No new dependencies needed. The go-sdk already includes Streamable HTTP transport support.

---

## 12. References

- [UCP Specification](https://ucp.dev/specification/overview/)
- [UCP Checkout Capability](https://ucp.dev/specification/checkout/)
- [UCP Checkout MCP Binding](https://ucp.dev/specification/checkout-mcp/)
- [UCP Order Capability](https://ucp.dev/specification/order/)
- [UCP Identity Linking](https://ucp.dev/specification/identity-linking/)
- [Shopify Storefront MCP](https://shopify.dev/docs/apps/build/storefront-mcp/servers/storefront)
- [Shopify Agent Commerce](https://shopify.dev/docs/agents)
- [A2UI Specification v0.8](https://a2ui.org/)
- [A2UI GitHub](https://github.com/google/A2UI)
- [WooCommerce REST API](https://woocommerce.github.io/woocommerce-rest-api-docs/)

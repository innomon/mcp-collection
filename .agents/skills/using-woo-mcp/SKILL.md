---
name: using-woo-mcp
description: "Connects AI agents to a WooCommerce store via the woo-mcp MCP server. Use when searching products, managing checkout sessions, placing orders, or querying order history on a WooCommerce instance."
---

# Using woo-mcp

Connect to a WooCommerce store through the woo-mcp MCP server. This skill covers product discovery, checkout lifecycle, order management, and identity linking.

## Setup

### 1. Configure `config.yaml`

```yaml
store_url: "https://your-store.example.com"
consumer_key: "ck_your_key"
consumer_secret: "cs_your_secret"
transport: "stdio"          # or "http" or "both"
http_port: 8080             # when using http/both
ucp_enabled: true           # enables UCP tools (recommended)
a2ui_enabled: true          # enables rich UI cards in responses
```

### 2. Connect via MCP

**stdio (local agent):**
```json
{
  "mcpServers": {
    "woo-mcp": {
      "command": "/path/to/woo-mcp",
      "args": ["-config", "/path/to/config.yaml"]
    }
  }
}
```

**Streamable HTTP (remote):**
Connect to `{store_url}/ucp/mcp` when transport is `http` or `both`.

## Available Tools

### Product Discovery

#### `search_shop_catalog`
Search the product catalog with context-aware ranking.

```json
{
  "query": "blue running shoes",
  "context": "Customer looking for athletic shoes under $100",
  "category": "shoes",
  "min_price": 20,
  "max_price": 100,
  "per_page": 10
}
```

Returns: `id`, `title`, `price` (cents), `currency`, `image_url`, `url`, `description`, `variant_id`, `in_stock`.

- `query` (required) — search terms
- `context` (required) — buyer context for ranking relevance
- `category`, `min_price`, `max_price`, `per_page` — optional filters

#### `get_product`
Get detailed product info including variants.

```json
{ "id": "42" }
```

Returns: product details with `variants[]` (id, title, price, in_stock, attributes) and `categories[]`.

#### `get_product_categories`
List all product categories. No input required.

#### `search_shop_policies_and_faqs`
Search store policies and FAQ pages.

```json
{ "query": "return policy" }
```

### Checkout Lifecycle

Checkout follows this flow: **create → update → complete → (payment redirect)**.

#### `create_checkout`
Create a new checkout session (WooCommerce pending order).

```json
{
  "line_items": [
    { "item_id": "42", "quantity": 2 },
    { "item_id": "99", "quantity": 1 }
  ],
  "buyer": {
    "email": "buyer@example.com",
    "first_name": "Jane",
    "last_name": "Doe"
  },
  "currency": "USD"
}
```

- `line_items` (required) — each with `item_id` (product ID as string) and `quantity`
- `buyer` — optional; email, first_name, last_name
- `currency` — optional, defaults to USD

#### `get_checkout`
Retrieve current checkout state.

```json
{ "id": "1234" }
```

#### `update_checkout`
Update line items or buyer info on an existing checkout.

```json
{
  "id": "1234",
  "line_items": [{ "item_id": "42", "quantity": 3 }],
  "buyer": { "email": "updated@example.com" }
}
```

#### `complete_checkout`
Finalize the checkout. Returns a `continue_url` pointing to the WooCommerce payment page — the buyer must complete payment in a browser.

```json
{ "id": "1234" }
```

#### `cancel_checkout`
Cancel a checkout session.

```json
{ "id": "1234" }
```

### Checkout Status Flow

```
create_checkout → "incomplete"
       ↓ (buyer info added)
  "requires_escalation" + continue_url (payment redirect)
       ↓ (buyer pays in browser)
  "complete_in_progress" → "completed"
```

| Status | Meaning |
|--------|---------|
| `incomplete` | Missing buyer info |
| `requires_escalation` | Buyer must complete payment via `continue_url` |
| `complete_in_progress` | Payment processing |
| `completed` | Order confirmed |
| `canceled` | Checkout cancelled |

**Important:** WooCommerce does not support tokenized payment via API. Checkout always transitions through `requires_escalation` with a `continue_url` for browser-based payment.

### Order Management

#### `get_order`
Get full order details including fulfillment tracking and refund adjustments.

```json
{ "id": "1234" }
```

Returns: status, line_items (with fulfillment status), fulfillment expectations/events, adjustments (refunds), and totals breakdown (subtotal, discount, fulfillment, tax, total).

#### `list_orders`
List recent orders.

```json
{ "per_page": 10 }
```

## Legacy Tools

These simpler tools are always available (regardless of `ucp_enabled`):

| Tool | Input | Returns |
|------|-------|---------|
| `search_products` | `{ "query": "..." }` | Product names, prices, URLs |
| `get_order_history` | `{}` | Recent orders with status |
| `checkout` | `{ "items": [{ "product_id": 42, "quantity": 1 }] }` | Payment URL |
| `raise_issue` | `{ "order_id": 1234, "text": "..." }` | Confirmation with note ID |

**Prefer UCP tools** (`search_shop_catalog`, `create_checkout`, etc.) over legacy tools — they provide richer data, structured JSON responses, and A2UI cards.

## Prices

All UCP tool responses return prices in **minor units (cents)**. For example, `1999` = $19.99. Currency is always an ISO 4217 code (e.g., `USD`, `EUR`).

## Typical Shopping Workflow

1. **Discover** — `search_shop_catalog` to find products matching the buyer's request
2. **Detail** — `get_product` for variants, stock, and full descriptions
3. **Create** — `create_checkout` with selected items and buyer info
4. **Review** — `get_checkout` to confirm line items and totals
5. **Adjust** — `update_checkout` if the buyer wants to change items or info
6. **Pay** — `complete_checkout` and present the `continue_url` to the buyer
7. **Track** — `get_order` after payment to check fulfillment status

## Error Codes

| Code | Meaning | Action |
|------|---------|--------|
| `MERCHANDISE_NOT_AVAILABLE` | Product out of stock | Suggest alternatives |
| `INVALID_LINE_ITEM` | Bad product ID | Verify ID with `get_product` |
| `CHECKOUT_EXPIRED` | Order expired (>6h) | Create a new checkout |
| `PAYMENT_REQUIRED` | Payment not yet submitted | Share `continue_url` |
| `FULFILLMENT_ADDRESS_REQUIRED` | Missing shipping address | Collect address, `update_checkout` |

This `GEMINI.md` file provides a technical blueprint for an MCP (Model Context Protocol) server built in Go. This server acts as a secure bridge between an AI Agent and a WooCommerce store, featuring **JWT/RSA** asymmetric authentication and a full customer lifecycle.

---

# GEMINI.md: WooCommerce MCP Server Specification

## 1. Technical Stack

* **Language:** Go (Golang) 1.22+
* **MCP Framework:** `github.com/modelcontextprotocol/go-sdk`
* **Auth:** `github.com/golang-jwt/jwt/v5` (RS256 Asymmetric)
* **API:** WooCommerce REST API v3
* **Transport:** Standard I/O (Stdio) or Server-Sent Events (SSE)

**IMPORTANT**: All parameters should be set from a yaml configuration file config.yaml, the search order for this `config.yaml` should be .1 args, .2 env CONFIG_FILE, .3 same executable path as the binary file.

---

## 2. Server Architecture

The server follows a **Resource-Tool-Handler** pattern. The Agent provides a JWT in the request metadata; the server verifies this via a Public RSA key before interacting with the WooCommerce API using Store API Keys.

### RSA Verification Logic

```go
// verify_auth.go
func VerifyToken(tokenStr string, pubKey *rsa.PublicKey) (*jwt.Token, error) {
    return jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
            return nil, fmt.Errorf("invalid signing method")
        }
        return pubKey, nil
    })
}

```

---

## 3. Tool Specifications & Schemas

### A. Product Discovery

* **Tool:** `search_products`
* **Function:** Fetches products via `GET /wp-json/wc/v3/products`.

### B. Customer Lifecycle ("Where is my order?")

* **Tool:** `get_order_history`
* **Logic:** Returns the last 10 orders with mapped statuses (**Open**, **In Process**, **Delivered**).

```go
// Tool Handler Snippet
func handleGetHistory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    // API Call: GET /orders?per_page=10&orderby=date
    // Result: List of IDs, Statuses, and Totals
    return mcp.NewToolResultText("Order #101: In Process\nOrder #100: Delivered"), nil
}

```

### C. Checkout & Payment

* **Tool:** `create_checkout_session`
* **Logic:** Generates a **Pending** order and returns a secure payment link.

```go
// Payment Link Generation
paymentURL := fmt.Sprintf("%s/checkout/order-pay/%d/?pay_for_order=true&key=%s", 
              storeBaseURL, orderID, orderKey)

```

---

## 4. Full Code Implementation

```go
package main

import (
    "context"
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
    s := mcp.NewMCPServer("WooCommerce-MCP", "1.0.0")

    // 1. Search Tool
    s.AddTool(mcp.NewTool("search_products",
        mcp.WithString("query", mcp.Required()),
    ), handleSearch)

    // 2. History Tool
    s.AddTool(mcp.NewTool("get_order_history",
        mcp.WithDescription("Show last 10 orders"),
    ), handleGetHistory)

    // 3. Checkout Tool
    s.AddTool(mcp.NewTool("checkout",
        mcp.WithDescription("Create order and get payment link"),
    ), handleCheckout)

    // 4. Issue Reporting Tool
    s.AddTool(mcp.NewTool("raise_issue",
        mcp.WithNumber("order_id", mcp.Required()),
        mcp.WithString("text", mcp.Required()),
    ), handleIssue)

    mcp.ServeStdio(s)
}

```

---

## 5. Lifecycle Mapping

| Phase | User Query | MCP Tool Called |
| --- | --- | --- |
| **Discovery** | "Find red running shoes" | `search_products` |
| **Transaction** | "I want to buy these" | `checkout` |
| **Tracking** | "Where is my order?" | `get_order_history` |
| **Support** | "The item arrived broken" | `raise_issue` |

---

Would you like me to generate a **Dockerfile** to containerize this MCP server for deployment?
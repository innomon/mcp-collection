# Specification for MCP Server Core Implementation

## 1. Introduction
This document outlines the specification for implementing the core functionalities of the WooCommerce MCP server. The server will act as a secure intermediary, enabling AI Agents to interact with a WooCommerce store by providing robust authentication, product management, order tracking, and customer support capabilities.

## 2. Goals
*   Establish a secure communication bridge between AI Agents and WooCommerce.
*   Enable AI Agents to perform essential e-commerce operations.
*   Ensure data integrity and confidentiality for all transactions.
*   Provide a robust and scalable foundation for future MCP server enhancements.

## 3. Functional Requirements

### 3.1 Secure Authentication
*   **JWT/RSA Asymmetric Authentication:** The server MUST implement RS256 asymmetric JWT for secure, stateless authentication, as specified in `GEMINI.md`.
*   **Token Verification:** The server MUST verify incoming JWT tokens against a public RSA key to ensure authenticity and integrity.

### 3.2 Product Discovery
*   **Tool: `search_products`:** The server MUST expose a `search_products` tool for AI agents.
*   **WooCommerce API Interaction:** This tool MUST fetch products using `GET /wp-json/wc/v3/products` from the WooCommerce REST API.
*   **Query Parameter:** The `search_products` tool MUST accept a `query` parameter (string, required) to filter products.

### 3.3 Customer Lifecycle: Order History
*   **Tool: `get_order_history`:** The server MUST expose a `get_order_history` tool for AI agents.
*   **Logic:** This tool MUST return the last 10 orders for a customer with mapped statuses (Open, In Process, Delivered).
*   **WooCommerce API Interaction:** This tool MUST utilize `GET /orders?per_page=10&orderby=date` from the WooCommerce REST API.

### 3.4 Checkout & Payment
*   **Tool: `create_checkout_session` (or `checkout`):** The server MUST expose a tool to initiate a checkout process, generating a pending order and returning a secure payment link.
*   **Payment Link Generation:** The server MUST generate payment URLs in the format: `https://[storeBaseURL]/checkout/order-pay/[orderID]/?pay_for_order=true&key=[orderKey]`.

### 3.5 Issue Reporting
*   **Tool: `raise_issue`:** The server MUST expose a `raise_issue` tool for AI agents.
*   **Parameters:** This tool MUST accept `order_id` (number, required) and `text` (string, required) to report issues related to an order.

## 4. Non-Functional Requirements

### 4.1 Security
*   All communications between the AI agent and the MCP server MUST be encrypted (e.g., via HTTPS/TLS).
*   The server MUST handle JWT/RSA key management securely.
*   Protection against common web vulnerabilities (e.g., injection, access control issues).

### 4.2 Performance & Scalability
*   The server MUST handle concurrent requests efficiently without data corruption.
*   Response times for API calls SHOULD be optimized.

### 4.3 Observability
*   Comprehensive logging and error reporting MUST be implemented for all API interactions to facilitate debugging and monitoring.

### 4.4 Technology Stack (as per `tech-stack.md`)
*   **Language:** Go (Golang) 1.22+
*   **MCP Framework:** `github.com/modelcontextprotocol/go-sdk`
*   **Auth:** `github.com/golang-jwt/jwt/v5` (RS256 Asymmetric)
*   **API:** WooCommerce REST API v3
*   **Transport:** Standard I/O (Stdio) or Server-Sent Events (SSE)

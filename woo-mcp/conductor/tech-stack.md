# Tech Stack

## Programming Language
*   **Go (Golang) 1.22+**: Chosen for its high performance, strong typing, and excellent concurrency features, making it well-suited for building efficient server applications.

## MCP Framework
*   **github.com/modelcontextprotocol/go-sdk**: This framework provides the necessary tools and structure for building and managing MCP servers, ensuring compatibility and streamlined development within the Model Context Protocol ecosystem.

## Authentication
*   **JWT/RSA Asymmetric (github.com/golang-jwt/jwt/v5)**: Implementing RS256 asymmetric JWT for secure, stateless authentication. This method is explicitly outlined in the `GEMINI.md` specification and is crucial for securing communications between the AI Agent and the WooCommerce store.

## API Interaction
*   **WooCommerce REST API v3**: The server will interact with WooCommerce using its official REST API v3. This provides comprehensive and standardized access to store data, products, orders, and customer information, as specified in the `GEMINI.md` document.

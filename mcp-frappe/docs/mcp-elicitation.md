# MCP Elicitation Specification & Implementation Plan

Elicitation allows the `mcp-frappe` server to request missing information (like mandatory fields or approval tokens) directly from the user during tool execution. This bridges the gap between static JSON Schema definitions and dynamic Frappe metadata.

## 1. Specification (Scenarios)

| Scenario | Mode | Trigger | Elicitation Content |
| :--- | :--- | :--- | :--- |
| **Missing Mandatory Fields** | `form` | `frappe_create_record` fails with missing fields. | JSON Schema for the missing fields. |
| **Delete Approval** | `form` | `frappe_delete_record` in production without token. | Input field for `approval_token`. |
| **Ambiguous Resource** | `form` | `frappe_get_record` returns multiple potential matches. | Selection list (enum) of record names. |
| **OAuth/Web Login** | `url` | Frappe session expired/Unauthorized. | URL to the Frappe login/authorization page. |

## 2. Technical Mapping (Go SDK v1.2.0)

*   **Capability**: Advertise `Elicitation` in `ClientCapabilities` (Server must check this before eliciting).
*   **Error Handling**: Use `mcp.URLElicitationRequiredError` for URL-based prompts. 
*   **Custom Form Handling**: Since the SDK doesn't have a built-in `FormElicitationRequiredError` yet, we will return a structured `CallToolResult` with `IsError: true` and a custom `Meta` or `Content` block that the client-side UI can interpret as a form request (following the 2025-11-25 spec).

## 3. Implementation Plan

### Phase 1: Infrastructure & Detection
- [x] Update `FrappeClient.Do` to parse detailed Frappe error responses (e.g., `exc_type: "MandatoryError"`).
- [x] Create a `ElicitationBuilder` utility to map Frappe metadata to JSON Schema.
- [x] Add a check in `main.go` to verify if the connected client supports elicitation.

### Phase 2: Tool Enhancements
- [x] **Delete Tool**: Update `frappe_delete_record` to trigger a form elicitation for `approval_token` instead of failing.
- [x] **Create Tool**: Update `frappe_create_record` to trigger a form elicitation for missing mandatory fields.
- [x] **Resource Template**: Update `frappe://doctype/{name}` handler to elicit `{name}` if it's missing or invalid.

### Phase 3: Validation
- [ ] Add unit tests in `completions_test.go` (or a new `elicitation_test.go`) to mock Frappe errors and verify the tool returns the correct elicitation metadata.
- [ ] Verify behavior with an elicitation-capable MCP client (like Claude Desktop or a custom test harness).

## 4. Implementation Checklist

- [ ] **SDK Capability Check**:
  - Inside tool handler: `if !clientCapabilities.Elicitation.SupportsForm { return error }`
- [ ] **Form Construction**:
  - Extract `fieldname`, `label`, and `fieldtype` from Frappe metadata.
  - Convert Frappe types (Data, Select, Check, etc.) to JSON Schema types (string, boolean, enum).
- [ ] **State Management**:
  - Ensure the LLM knows it needs to wait for user input (handled by MCP protocol).
- [ ] **Error Fallback**:
  - Provide a graceful degradation path if the client does not support elicitation.

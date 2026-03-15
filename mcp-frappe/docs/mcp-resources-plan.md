# Frappe MCP Resources Implementation Plan

This plan outlines the steps for adding MCP resource support, completions, and elicitation to the Frappe MCP server.

## Overview
Resources allow the Frappe MCP server to expose read-only system metadata and schemas. Completions and elicitation enhance the interactive capabilities of the server.

## Implementation Tasks

### 1. Handler Registration (main.go)
- [x] Initialize the handlers in `main.go`.
    - `server.SetListResourcesHandler(listResourcesHandler)`
    - `server.SetListResourceTemplatesHandler(listResourceTemplatesHandler)`
    - `server.SetReadResourceHandler(readResourceHandler)`
    - [x] `CompletionHandler` integration in `ServerOptions`.

### 2. FrappeClient Methods (client.go)
- [x] `GetDocTypes(ctx)`: Fetch a list of all DocTypes.
- [x] `GetModules(ctx)`: Fetch a list of all modules.
- [x] `GetApps(ctx)`: Fetch a list of installed apps.
- [x] `GetSystemInfo(ctx)`: Fetch system and user details.
- [x] `GetRecords(ctx)`: Fetch record names with prefix matching.

### 3. Resource Mapping Logic
- [x] **Static Resources**:
    - `frappe://doctypes` -> `GetDocTypes`
    - `frappe://modules` -> `GetModules`
    - `frappe://apps` -> `GetApps`
    - `frappe://system/info` -> `GetSystemInfo`
- [x] **Template Resources**:
    - `frappe://doctype/{name}` -> `getDocTypeMeta`
    - `frappe://doctype/{name}/schema` -> `normalizeDocTypeMeta`
    - `frappe://doctype/{name}/ui-candidates` -> `DocTypeToCandidates`

### 4. Completions & Elicitation (New Phases)
- [x] **Completions Phase**:
    - Implement `newCompletionHandler` in `completions.go`.
    - Support `ref/tool` and `ref/resource` references.
    - Implement prefix-based filtering for DocTypes and record names.
- [x] **Elicitation Phase**:
    - Implement `ElicitationBuilder` in `elicitation.go`.
    - Support missing mandatory fields elicitation in `frappe_create_record`.
    - Support production delete approval elicitation in `frappe_delete_record`.
    - Support DocType selection elicitation in resource templates.

### 5. Code Integration
- [x] Add the handler functions to `resources.go`.
- [x] Implement structured `FrappeError` for better error context.
- [x] Ensure proper error handling and logging.

### 6. Testing & Documentation
- [x] Create `completions_test.go` and `elicitation_test.go`.
- [x] Verify behavior with automated unit and integration tests.
- [x] Update documentation (`mcp-resources-spec.md`, `mcp-elicitation.md`).

## Timeline (Completed)
1. **Phase 1**: Infrastructure & Static Resources
2. **Phase 2**: Resource Templates & Metadata
3. **Phase 3**: Completions (v1.1.0)
4. **Phase 4**: Elicitation (v1.2.0)

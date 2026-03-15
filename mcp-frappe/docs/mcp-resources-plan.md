# Frappe MCP Resources Implementation Plan

This plan outlines the steps for adding MCP resource support to the Frappe MCP server.

## Overview
Resources will allow the Frappe MCP server to expose read-only system metadata and schemas, facilitating better discovery for LLM agents.

## Implementation Tasks

### 1. Handler Registration (main.go)
- [ ] Initialize the handlers in `main.go`.
    - `server.SetListResourcesHandler(listResourcesHandler)`
    - `server.SetListResourceTemplatesHandler(listResourceTemplatesHandler)`
    - `server.SetReadResourceHandler(readResourceHandler)`

### 2. FrappeClient Methods (main.go)
- [ ] `GetDocTypes(ctx)`: Fetch a list of all DocTypes.
- [ ] `GetModules(ctx)`: Fetch a list of all modules.
- [ ] `GetApps(ctx)`: Fetch a list of installed apps.
- [ ] `GetSystemInfo(ctx)`: Fetch system and user details.

### 3. Resource Mapping Logic
- [ ] **Static Resources**:
    - `frappe://doctypes` -> `GetDocTypes`
    - `frappe://modules` -> `GetModules`
    - `frappe://apps` -> `GetApps`
    - `frappe://system/info` -> `GetSystemInfo`
- [ ] **Template Resources**:
    - `frappe://doctype/{name}` -> `getDocTypeMeta`
    - `frappe://doctype/{name}/schema` -> `normalizeDocTypeMeta`
    - `frappe://doctype/{name}/ui-candidates` -> `DocTypeToCandidates`

### 4. Code Integration
- [ ] Add the handler functions to `main.go` or a new `resources.go`.
- [ ] Use `mcp.NewResource` to wrap the returned data.
- [ ] Ensure proper error handling and logging.

### 5. Testing & Documentation
- [ ] Create `resources_test.go` to test the URI routing and data fetching.
- [ ] Verify that resources appear in the MCP server discovery.
- [ ] Update `GEMINI.md` with resource examples.

## Timeline
1. **Phase 1**: Infrastructure & Static Resources (Estimated: 2 hours)
2. **Phase 2**: Resource Templates & Metadata (Estimated: 2 hours)
3. **Phase 3**: Testing & Final Polishing (Estimated: 1 hour)

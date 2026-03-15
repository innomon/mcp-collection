# Frappe MCP Resources Specification

This document defines the MCP Resource capability for the Frappe MCP server. Resources allow agents to discover system metadata and schemas without performing active tool calls.

## URI Scheme: `frappe://`

## Discovery Resources (Static)

### `frappe://doctypes`
- **Description**: Lists all DocTypes the current user has access to.
- **MIME Type**: `application/json`
- **Purpose**: Helps agents browse available data structures.

### `frappe://modules`
- **Description**: Lists all modules in the system.
- **MIME Type**: `application/json`
- **Purpose**: Provides high-level categorization of the system.

### `frappe://apps`
- **Description**: Lists installed Frappe applications and their versions.
- **MIME Type**: `application/json`

### `frappe://system/info`
- **Description**: Basic system information including Frappe/ERPNext versions and current user profile.
- **MIME Type**: `application/json`

## Metadata Resources (Templates)

### `frappe://doctype/{name}`
- **Description**: Full raw metadata for a specific DocType.
- **MIME Type**: `application/json`
- **Parameters**: 
    - `name`: The name of the DocType (e.g., `Customer`).
- **Completion**: Supports autocompletion for the `{name}` parameter based on available DocTypes.
- **Elicitation**: If `{name}` is missing (e.g., `frappe://doctype/`), the server will trigger a form elicitation to prompt the user for a DocType name selection.

### `frappe://doctype/{name}/schema`
- **Description**: Normalized canonical JSON schema for the DocType.
- **MIME Type**: `application/json`
- **Purpose**: Provides a clean, versioned schema for agents to use when generating data.

### `frappe://doctype/{name}/ui-candidates`
- **Description**: A2UI schema candidates derived from the DocType metadata.
- **MIME Type**: `application/json`
- **Purpose**: Integrates with the A2UI pipeline for frontend rendering hints.

## Implementation Details

### ListResources
The server will return the list of static URIs.

### ListResourceTemplates
The server will return the template patterns with descriptions.

### Completions (Argument Suggestions)
The server implements `CompletionHandler` to provide dynamic suggestions for:
- **Tools**: `doctype` and `name` arguments for Frappe-specific tools.
- **Resources**: `{name}` parameters in resource templates.
- **Allowlist Filtering**: All suggestions respect `FRAPPE_ALLOWED_DOCTYPES`.

### Elicitation (Interactive Prompts)
The server implements interactive elicitation for:
- **Missing Mandatory Fields**: Prompts for required fields during record creation.
- **Production Delete Approval**: Prompts for a secret token when deleting in production.
- **Resource Parameter Selection**: Prompts for DocType name when a template is accessed without parameters.

### ReadResource
The server will parse the URI and fetch data from Frappe using the `FrappeClient`.
- Metadata is cached using the existing `DocTypeCache`.
- Discovery resources are fetched on-demand or with short-term caching.

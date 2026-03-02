# Frappe DocType JSON Contract — v0.1.0 (Draft)

This document defines the canonical project-owned JSON contract for DocType generation.

Why this exists:

- `docs/frappe-doctype.md` is a reference list, not a normative schema.
- Integration logic and MCP generation tools need a stable versioned contract.

## 1. Contract Envelope

Generated DocType JSON must include:

```json
{
  "contract_version": "0.1.0",
  "doctype": {
    "name": "Customer",
    "module": "Selling",
    "is_submittable": false,
    "modified": "2026-02-25T10:00:00Z",
    "fields": [],
    "permissions": []
  }
}
```

## 2. Required Fields

| Path | Type | Notes |
|------|------|-------|
| `contract_version` | string | Must match supported version in generator/validator. |
| `doctype.name` | string | Canonical DocType identifier. |
| `doctype.module` | string | Frappe module grouping. |
| `doctype.fields` | array | List of normalized field entries. |

## 3. Field Schema (`doctype.fields[]`)

Minimum field keys:

- `fieldname` (string)
- `label` (string)
- `fieldtype` (string)
- `reqd` (boolean)
- `options` (string, nullable)

## 4. Permission Schema (`doctype.permissions[]`)

Minimum permission keys:

- `role` (string)
- `read` (boolean)
- `write` (boolean)
- `create` (boolean)
- `delete` (boolean)

## 5. Compatibility Policy

- Additive fields are backward compatible.
- Removal/rename of existing required keys is a breaking change and requires contract version bump.
- MCP `frappe_generate_doctype_json` must emit `contract_version`.
- MCP `frappe_validate_doctype_json` must return machine-readable violations by path.

## 6. Validation Outcome Model

Recommended validation result envelope:

```json
{
  "contract_version": "0.1.0",
  "valid": false,
  "violations": [
    {
      "path": "doctype.fields[2].fieldtype",
      "code": "unsupported_fieldtype",
      "message": "Field type 'FooType' is not supported in v0.1.0"
    }
  ]
}
```

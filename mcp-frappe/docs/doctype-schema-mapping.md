# DocType → SchemaCandidate Mapping and Precedence — v0.1.0

This document defines how Frappe DocType metadata is normalized into `SchemaCandidate` entries for the A2UI schema-selection pipeline, and how static and DocType-derived candidates are merged with deterministic precedence.

## 1. Mapping Overview

A Frappe DocType is a metadata descriptor for a business entity. The backend normalizes relevant DocType metadata into one or more `SchemaCandidate` values so they can participate in the existing schema-selection pipeline alongside static `app_role_schemas` candidates.

### Mapping Flow

```
FrappeDocType (remote)
  → normalize fields + permissions
  → FrappeDocType / FrappeField (internal model)
  → derive SchemaCandidate(s)
  → merge with static candidates
  → Selector.Select(candidates, content)
```

---

## 2. Frappe Field Type → A2UI Mapping Table

Each Frappe field type maps to an A2UI payload constraint that determines which card schema(s) the DocType can produce.

| Frappe Field Type   | A2UI Constraint          | Candidate Schema   | Notes                                              |
|---------------------|--------------------------|--------------------|----------------------------------------------------|
| `Data`              | text / key-value         | `info_card`        | Short text field; maps to label-value pair.         |
| `Small Text`        | text / key-value         | `info_card`        | Same as `Data` but allows slightly longer content.  |
| `Text`              | markdown body            | `markdown`         | Long-form text; rendered as markdown.               |
| `Text Editor`       | markdown body            | `markdown`         | Rich-text content rendered as markdown.             |
| `Int`               | numeric key-value        | `info_card`        | Integer field; rendered as label-value.             |
| `Float`             | numeric key-value        | `info_card`        | Decimal field; rendered as label-value.             |
| `Currency`          | numeric key-value        | `info_card`        | Currency-formatted value.                           |
| `Date`              | date key-value           | `info_card`        | ISO date string.                                    |
| `Datetime`          | datetime key-value       | `info_card`        | ISO datetime string.                                |
| `Select`            | enumerated key-value     | `info_card`        | Value from `options` list.                          |
| `Link`              | reference key-value      | `info_card`        | Reference to another DocType.                       |
| `Check`             | boolean key-value        | `info_card`        | Boolean rendered as yes/no or true/false.           |
| `Table`             | tabular data             | `data_table`       | Child table DocType; rows map to table rows.        |
| `Table MultiSelect` | tabular data             | `data_table`       | Multi-select link table.                            |
| `Attach`            | text / key-value         | `info_card`        | File attachment URL; label-value only.              |
| `Attach Image`      | text / key-value         | `info_card`        | Image URL; label-value only (no inline rendering).  |
| `Section Break`     | layout (ignored)         | —                  | Layout field; excluded from schema mapping.         |
| `Column Break`      | layout (ignored)         | —                  | Layout field; excluded from schema mapping.         |
| `Tab Break`         | layout (ignored)         | —                  | Layout field; excluded from schema mapping.         |
| `HTML`              | markdown body            | `markdown`         | Rendered as markdown content.                       |
| `Read Only`         | text / key-value         | `info_card`        | Display-only field.                                 |
| `Password`          | redacted                 | —                  | Never included in schema output.                    |

### Layout and Meta Fields

The following field types are **excluded** from SchemaCandidate derivation:

- `Section Break`, `Column Break`, `Tab Break` — layout-only, no data.
- `Password` — sensitive; must never appear in rendered output.
- `Button` — server-side action trigger; not a data field.

---

## 3. SchemaCandidate Derivation Rules

A single `FrappeDocType` may produce **one or more** `SchemaCandidate` entries depending on its field composition.

### Rule 1: Info Card Candidate

If the DocType contains **at least one non-layout, non-table data field** (e.g., `Data`, `Int`, `Select`, `Link`, `Currency`), generate an `info_card` candidate.

```json
{
  "schema": "info_card",
  "description": "Key-value summary of <DocType.name> fields: <comma-separated field labels>.",
  "example": {
    "title": "<DocType.name>",
    "fields": [
      { "label": "<field.label>", "value": "<placeholder>" }
    ]
  }
}
```

### Rule 2: Data Table Candidate

If the DocType contains **at least one `Table` or `Table MultiSelect` field**, generate a `data_table` candidate.

```json
{
  "schema": "data_table",
  "description": "Tabular view of <child DocType name> entries within <DocType.name>.",
  "example": {
    "title": "<DocType.name> — <child table label>",
    "columns": ["<child field labels...>"],
    "rows": [["<placeholder values...>"]]
  }
}
```

### Rule 3: Markdown Fallback Candidate

If the DocType contains **at least one long-text field** (`Text`, `Text Editor`, `HTML`), generate a `markdown` candidate.

```json
{
  "schema": "markdown",
  "description": "Long-form content from <DocType.name> rendered as markdown.",
  "example": {
    "text": "## <DocType.name>\n\n<placeholder content>"
  }
}
```

### Rule 4: Action Card Candidate

If the DocType has `is_submittable: true` **and** the user's resolved role has `submit` permission, generate an `action_card` candidate.

```json
{
  "schema": "action_card",
  "description": "Submission actions for <DocType.name>.",
  "example": {
    "title": "<DocType.name>",
    "description": "Review and take action.",
    "actions": [
      { "label": "Submit", "action_id": "submit_<doctype_name>", "style": "primary" },
      { "label": "Cancel", "action_id": "cancel_<doctype_name>", "style": "danger" }
    ]
  }
}
```

### Source Annotation

Each DocType-derived `SchemaCandidate` carries a `source` metadata tag (not serialized to the LLM prompt but used internally for logging and precedence):

| Field        | Value                              |
|--------------|------------------------------------|
| `source`     | `"doctype"`                        |
| `doctype`    | DocType name (e.g., `"Customer"`)  |
| `version`    | `FrappeDocType.modified` timestamp |

---

## 4. Candidate Precedence and Merge Policy

When both static (`app_role_schemas`) and DocType-derived candidates are available, they are merged into a single ordered list using deterministic precedence.

### Precedence Order (highest to lowest)

| Priority | Source                   | Description                                              |
|----------|--------------------------|----------------------------------------------------------|
| 1        | Static app-role override | Explicit schemas from `app_role_schemas/{appid}_{role}`. |
| 2        | DocType-derived          | Candidates generated from Frappe DocType metadata.       |
| 3        | Default fallback         | `markdown` schema (always present as last resort).       |

### Merge Rules

1. **Static candidates first.** All static `app_role_schemas` candidates are placed at the front of the list in their original order.
2. **DocType candidates second.** DocType-derived candidates are appended after static candidates.
3. **Deduplication by schema name.** If a DocType-derived candidate has the same `schema` value as an existing static candidate, the **static candidate wins** and the DocType duplicate is dropped. This prevents the LLM from seeing conflicting examples for the same card type.
4. **Fallback always last.** If no static or DocType candidate has `schema: "markdown"`, a default markdown fallback candidate is appended as the final entry.
5. **Candidate cap.** The merged list is capped at **10 candidates** to bound LLM context size. If the cap is exceeded, lower-priority DocType candidates are dropped first.

### Merge Algorithm (Pseudocode)

```
func MergeCandidates(static, doctype []SchemaCandidate) []SchemaCandidate:
    seen := set of schema names from static
    merged := copy of static

    for each candidate in doctype:
        if candidate.schema not in seen:
            merged.append(candidate)
            seen.add(candidate.schema)

    if "markdown" not in seen:
        merged.append(defaultMarkdownCandidate)

    if len(merged) > 10:
        merged = merged[:10]

    return merged
```

### Fallback Behavior

| Condition                              | Behavior                                                |
|----------------------------------------|---------------------------------------------------------|
| Frappe unavailable or timeout          | Pipeline uses static candidates only. No error surfaced to user. |
| Frappe returns malformed DocType       | DocType is skipped; static candidates used. Warning logged. |
| No static candidates exist             | DocType candidates used alone (with markdown fallback). |
| No candidates at all                   | Markdown fallback is the sole candidate.                |
| Selector confidence below threshold    | Markdown fallback is selected regardless of candidates. |

---

## 5. Versioning

- This mapping spec version: **v0.1.0**
- Additive changes (new field type mappings, new candidate rules) are backward compatible (minor bump).
- Removal or change of existing mapping semantics requires a major version bump.
- The `contract_version` in `frappe-doctype-json-contract.md` is independent of this mapping version.

package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// layoutFieldTypes are Frappe field types that represent layout structure
// rather than data fields. They are excluded from descriptions and examples.
var layoutFieldTypes = map[string]struct{}{
	"Section Break": {},
	"Column Break":  {},
	"Tab Break":     {},
}

// DocTypeToCandidates converts a slice of FrappeDocType definitions into
// SchemaCandidate entries suitable for the selection pipeline.
func DocTypeToCandidates(doctypes []FrappeDocType) []SchemaCandidate {
	candidates := make([]SchemaCandidate, 0, len(doctypes))
	for i := range doctypes {
		candidates = append(candidates, docTypeToCandidate(&doctypes[i]))
	}
	return candidates
}

func docTypeToCandidate(dt *FrappeDocType) SchemaCandidate {
	return SchemaCandidate{
		Schema:      docTypeSchemaName(dt.Name),
		Description: docTypeDescription(dt),
		Example:     docTypeExample(dt),
		Source: &CandidateSource{
			Source:  "doctype",
			DocType: dt.Name,
			Version: dt.Modified,
		},
	}
}

// docTypeSchemaName normalises a Frappe DocType name into a schema identifier.
// "Sales Order" → "sales_order", "Customer" → "customer".
func docTypeSchemaName(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

// docTypeDescription builds a human-readable description of the DocType
// including module, data fields, and required-field markers.
func docTypeDescription(dt *FrappeDocType) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Data schema for %s (%s module).", dt.Name, dt.Module)

	dataFields := filterDataFields(dt.Fields)
	if len(dataFields) > 0 {
		b.WriteString(" Fields: ")
		for i, f := range dataFields {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(f.Label)
			if f.Label == "" {
				b.WriteString(f.Fieldname)
			}
			fmt.Fprintf(&b, " (%s", f.Fieldtype)
			if f.Reqd {
				b.WriteString(", required")
			}
			if f.Options != nil && *f.Options != "" {
				fmt.Fprintf(&b, ", %s", *f.Options)
			}
			b.WriteByte(')')
		}
		b.WriteByte('.')
	}
	return b.String()
}

func filterDataFields(fields []FrappeField) []FrappeField {
	out := make([]FrappeField, 0, len(fields))
	for _, f := range fields {
		if _, isLayout := layoutFieldTypes[f.Fieldname]; isLayout {
			continue
		}
		if _, isLayout := layoutFieldTypes[f.Fieldtype]; isLayout {
			continue
		}
		out = append(out, f)
	}
	return out
}

// docTypeExample produces a minimal JSON example payload derived from
// the DocType's data fields.
func docTypeExample(dt *FrappeDocType) json.RawMessage {
	payload := map[string]any{
		"doctype": dt.Name,
	}
	fields := make(map[string]any)
	for _, f := range filterDataFields(dt.Fields) {
		fields[f.Fieldname] = fieldPlaceholder(f)
	}
	if len(fields) > 0 {
		payload["fields"] = fields
	}
	data, _ := json.Marshal(payload)
	return data
}

func fieldPlaceholder(f FrappeField) any {
	switch f.Fieldtype {
	case "Int":
		return 0
	case "Float", "Currency":
		return 0.0
	case "Check":
		return false
	case "Table", "Table MultiSelect":
		return []any{}
	default:
		return ""
	}
}

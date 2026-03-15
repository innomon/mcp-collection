package main

import (
	"fmt"
	"strconv"
	"strings"
)

const doctypeContractVersion = "0.1.0"

type DocTypeContractEnvelope struct {
	ContractVersion string               `json:"contract_version"`
	DocType         CanonicalDocTypeJSON `json:"doctype"`
}

type CanonicalDocTypeJSON struct {
	Name          string               `json:"name"`
	Module        string               `json:"module"`
	IsSubmittable bool               `json:"is_submittable"`
	Modified      string               `json:"modified"`
	Fields        []CanonicalFieldJSON `json:"fields"`
	Permissions   []PermissionJSON     `json:"permissions"`
}

type CanonicalFieldJSON struct {
	FieldName string  `json:"fieldname"`
	Label     string  `json:"label"`
	FieldType string  `json:"fieldtype"`
	Reqd      bool    `json:"reqd"`
	Options   *string `json:"options"`
}

type PermissionJSON struct {
	Role   string `json:"role"`
	Read   bool   `json:"read"`
	Write  bool   `json:"write"`
	Create bool   `json:"create"`
	Delete bool   `json:"delete"`
}

type ValidationViolation struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ValidationResult struct {
	ContractVersion string                `json:"contract_version"`
	Valid           bool                  `json:"valid"`
	Violations      []ValidationViolation `json:"violations"`
}

// metaToFrappeDocType converts raw Frappe metadata into the pipeline's FrappeDocType model.
func metaToFrappeDocType(docType string, source map[string]any) *FrappeDocType {
	docNode := source
	if nested, ok := source["data"].(map[string]any); ok {
		docNode = nested
	}

	name := asString(docNode["name"])
	if name == "" {
		name = docType
	}

	var fields []FrappeField
	if rawFields, ok := docNode["fields"].([]any); ok {
		for _, raw := range rawFields {
			entry, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			f := FrappeField{
				Fieldname: asString(entry["fieldname"]),
				Label:     asString(entry["label"]),
				Fieldtype: asString(entry["fieldtype"]),
				Reqd:      asBool(entry["reqd"]),
			}
			options := strings.TrimSpace(asString(entry["options"]))
			if options != "" {
				f.Options = &options
			}
			fields = append(fields, f)
		}
	}

	var permissions []FrappePermission
	if rawPerms, ok := docNode["permissions"].([]any); ok {
		for _, raw := range rawPerms {
			entry, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			permissions = append(permissions, FrappePermission{
				Role:   asString(entry["role"]),
				Read:   asBool(entry["read"]),
				Write:  asBool(entry["write"]),
				Create: asBool(entry["create"]),
				Delete: asBool(entry["delete"]),
				Submit: asBool(entry["submit"]),
			})
		}
	}

	return &FrappeDocType{
		Name:          name,
		Module:        asString(docNode["module"]),
		IsSubmittable: asBool(docNode["is_submittable"]),
		Modified:      asString(docNode["modified"]),
		Fields:        fields,
		Permissions:   permissions,
	}
}

func normalizeDocTypeMeta(docType string, source map[string]any) (DocTypeContractEnvelope, error) {
	docNode := source
	if nested, ok := source["data"].(map[string]any); ok {
		docNode = nested
	}

	name := asString(docNode["name"])
	if name == "" {
		name = docType
	}

	fields := make([]CanonicalFieldJSON, 0)
	if rawFields, ok := docNode["fields"].([]any); ok {
		for idx, raw := range rawFields {
			entry, ok := raw.(map[string]any)
			if !ok {
				return DocTypeContractEnvelope{}, fmt.Errorf("field index %d is not an object", idx)
			}
			field := CanonicalFieldJSON{
				FieldName: asString(entry["fieldname"]),
				Label:     asString(entry["label"]),
				FieldType: asString(entry["fieldtype"]),
				Reqd:      asBool(entry["reqd"]),
			}
			options := strings.TrimSpace(asString(entry["options"]))
			if options != "" {
				field.Options = &options
			}
			fields = append(fields, field)
		}
	}

	permissions := make([]PermissionJSON, 0)
	if rawPerms, ok := docNode["permissions"].([]any); ok {
		for idx, raw := range rawPerms {
			entry, ok := raw.(map[string]any)
			if !ok {
				return DocTypeContractEnvelope{}, fmt.Errorf("permission index %d is not an object", idx)
			}
			permissions = append(permissions, PermissionJSON{
				Role:   asString(entry["role"]),
				Read:   asBool(entry["read"]),
				Write:  asBool(entry["write"]),
				Create: asBool(entry["create"]),
				Delete: asBool(entry["delete"]),
			})
		}
	}

	return DocTypeContractEnvelope{
		ContractVersion: doctypeContractVersion,
		DocType: CanonicalDocTypeJSON{
			Name:          name,
			Module:        asString(docNode["module"]),
			IsSubmittable: asBool(docNode["is_submittable"]),
			Modified:      asString(docNode["modified"]),
			Fields:        fields,
			Permissions:   permissions,
		},
	}, nil
}

func validateDocTypeJSON(payload map[string]any) ValidationResult {
	result := ValidationResult{
		ContractVersion: doctypeContractVersion,
		Valid:           true,
		Violations:      make([]ValidationViolation, 0),
	}

	addViolation := func(path, code, message string) {
		result.Valid = false
		result.Violations = append(result.Violations, ValidationViolation{Path: path, Code: code, Message: message})
	}

	if asString(payload["contract_version"]) != doctypeContractVersion {
		addViolation("contract_version", "unsupported_contract_version", fmt.Sprintf("expected %s", doctypeContractVersion))
	}

	doc, ok := payload["doctype"].(map[string]any)
	if !ok {
		addViolation("doctype", "missing_or_invalid", "doctype must be an object")
		return result
	}

	if strings.TrimSpace(asString(doc["name"])) == "" {
		addViolation("doctype.name", "required", "doctype.name is required")
	}
	if strings.TrimSpace(asString(doc["module"])) == "" {
		addViolation("doctype.module", "required", "doctype.module is required")
	}

	fields, ok := doc["fields"].([]any)
	if !ok {
		addViolation("doctype.fields", "missing_or_invalid", "doctype.fields must be an array")
	} else {
		for idx, raw := range fields {
			field, ok := raw.(map[string]any)
			if !ok {
				addViolation(fmt.Sprintf("doctype.fields[%d]", idx), "invalid_type", "field entry must be an object")
				continue
			}
			if strings.TrimSpace(asString(field["fieldname"])) == "" {
				addViolation(fmt.Sprintf("doctype.fields[%d].fieldname", idx), "required", "fieldname is required")
			}
			if strings.TrimSpace(asString(field["label"])) == "" {
				addViolation(fmt.Sprintf("doctype.fields[%d].label", idx), "required", "label is required")
			}
			if strings.TrimSpace(asString(field["fieldtype"])) == "" {
				addViolation(fmt.Sprintf("doctype.fields[%d].fieldtype", idx), "required", "fieldtype is required")
			}
			if !isBooleanLike(field["reqd"]) {
				addViolation(fmt.Sprintf("doctype.fields[%d].reqd", idx), "invalid_type", "reqd must be boolean")
			}
			if _, exists := field["options"]; exists {
				if field["options"] != nil {
					if _, ok := field["options"].(string); !ok {
						addViolation(fmt.Sprintf("doctype.fields[%d].options", idx), "invalid_type", "options must be string or null")
					}
				}
			}
		}
	}

	if rawPerms, exists := doc["permissions"]; exists {
		permissions, ok := rawPerms.([]any)
		if !ok {
			addViolation("doctype.permissions", "invalid_type", "permissions must be an array")
		} else {
			for idx, raw := range permissions {
				perm, ok := raw.(map[string]any)
				if !ok {
					addViolation(fmt.Sprintf("doctype.permissions[%d]", idx), "invalid_type", "permission entry must be an object")
					continue
				}
				if strings.TrimSpace(asString(perm["role"])) == "" {
					addViolation(fmt.Sprintf("doctype.permissions[%d].role", idx), "required", "role is required")
				}
				for _, key := range []string{"read", "write", "create", "delete"} {
					if !isBooleanLike(perm[key]) {
						addViolation(fmt.Sprintf("doctype.permissions[%d].%s", idx, key), "invalid_type", fmt.Sprintf("%s must be boolean", key))
					}
				}
			}
		}
	}

	return result
}

func asString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func asBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case int:
		return v != 0
	case int64:
		return v != 0
	case string:
		lower := strings.ToLower(strings.TrimSpace(v))
		return lower == "1" || lower == "true" || lower == "yes"
	default:
		return false
	}
}

func isBooleanLike(value any) bool {
	if value == nil {
		return false
	}
	switch value.(type) {
	case bool, float64, int, int64:
		return true
	default:
		return false
	}
}

package main

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ElicitationBuilder helps construct MCP elicitation requests from Frappe metadata.
type ElicitationBuilder struct{}

// BuildFormSchema converts a slice of FrappeFields into a JSON Schema for form elicitation.
func (b *ElicitationBuilder) BuildFormSchema(fields []FrappeField) map[string]any {
	properties := make(map[string]any)
	required := []string{}

	for _, field := range fields {
		prop := map[string]any{
			"type":        b.mapFrappeTypeToJSONSchema(field.Fieldtype),
			"description": field.Label,
		}

		if field.Fieldtype == "Select" && field.Options != nil {
			// Basic parsing of Frappe select options (newline separated)
			options := []string{}
			for _, opt := range splitOptions(*field.Options) {
				if opt != "" {
					options = append(options, opt)
				}
			}
			if len(options) > 0 {
				prop["enum"] = options
			}
		}

		properties[field.Fieldname] = prop
		if field.Reqd {
			required = append(required, field.Fieldname)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

func (b *ElicitationBuilder) mapFrappeTypeToJSONSchema(frappeType string) string {
	switch frappeType {
	case "Int", "Check":
		return "integer"
	case "Float", "Currency", "Percent":
		return "number"
	case "Date", "Datetime", "Time", "Text", "Small Text", "Long Text", "Code", "Data", "Link", "Dynamic Link", "Password", "Read Only":
		return "string"
	default:
		return "string"
	}
}

// BuildElicitParams creates the ElicitParams for a form-based elicitation.
func (b *ElicitationBuilder) BuildElicitParams(message string, fields []FrappeField) *mcp.ElicitParams {
	return &mcp.ElicitParams{
		Mode:            "form",
		Message:         message,
		RequestedSchema: b.BuildFormSchema(fields),
	}
}

func splitOptions(options string) []string {
	// Frappe Select options are usually newline separated
	var result []string
	curr := ""
	for i := 0; i < len(options); i++ {
		if options[i] == '\n' {
			result = append(result, curr)
			curr = ""
		} else {
			curr += string(options[i])
		}
	}
	if curr != "" {
		result = append(result, curr)
	}
	return result
}

// SupportsElicitation checks if the client supports form or url elicitation.
func SupportsElicitation(req *mcp.CallToolRequest, mode string) bool {
	if req.Session == nil {
		return false
	}
	params := req.Session.InitializeParams()
	if params == nil || params.Capabilities == nil || params.Capabilities.Elicitation == nil {
		return false
	}

	elicit := params.Capabilities.Elicitation
	switch mode {
	case "form":
		// Per spec, if neither Form nor URL is set, Form is assumed.
		return elicit.Form != nil || elicit.URL == nil
	case "url":
		return elicit.URL != nil
	default:
		return false
	}
}

// SupportsElicitationFromRead checks if the client supports form or url elicitation for resource requests.
func SupportsElicitationFromRead(req *mcp.ReadResourceRequest, mode string) bool {
	if req.Session == nil {
		return false
	}
	params := req.Session.InitializeParams()
	if params == nil || params.Capabilities == nil || params.Capabilities.Elicitation == nil {
		return false
	}

	elicit := params.Capabilities.Elicitation
	switch mode {
	case "form":
		return elicit.Form != nil || elicit.URL == nil
	case "url":
		return elicit.URL != nil
	default:
		return false
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Surface represents an A2UI surface with components and data.
type Surface struct {
	ID         string
	Components []SurfaceComponent
	Data       map[string]any
}

type SurfaceComponent struct {
	ID        string         `json:"id"`
	Component map[string]any `json:"component"`
}

type SurfaceUpdate struct {
	SurfaceUpdate struct {
		SurfaceID  string             `json:"surfaceId"`
		Components []SurfaceComponent `json:"components"`
	} `json:"surfaceUpdate"`
}

type DataModelUpdate struct {
	DataModelUpdate struct {
		SurfaceID string `json:"surfaceId"`
		Contents  []any  `json:"contents"`
	} `json:"dataModelUpdate"`
}

type BeginRendering struct {
	BeginRendering struct {
		SurfaceID string `json:"surfaceId"`
		Root      string `json:"root"`
	} `json:"beginRendering"`
}

func NewSurface(id string) *Surface {
	return &Surface{
		ID:   id,
		Data: make(map[string]any),
	}
}

func (s *Surface) AddComponent(id string, component map[string]any) *Surface {
	s.Components = append(s.Components, SurfaceComponent{ID: id, Component: component})
	return s
}

func (s *Surface) SetData(key string, value any) *Surface {
	s.Data[key] = value
	return s
}

// Render produces the JSONL lines: surfaceUpdate, dataModelUpdate, beginRendering.
func (s *Surface) Render(rootID string) []json.RawMessage {
	su := SurfaceUpdate{}
	su.SurfaceUpdate.SurfaceID = s.ID
	su.SurfaceUpdate.Components = s.Components

	du := DataModelUpdate{}
	du.DataModelUpdate.SurfaceID = s.ID
	var contents []any
	for k, v := range s.Data {
		contents = append(contents, map[string]any{"key": k, "value": v})
	}
	du.DataModelUpdate.Contents = contents

	br := BeginRendering{}
	br.BeginRendering.SurfaceID = s.ID
	br.BeginRendering.Root = rootID

	var lines []json.RawMessage
	for _, item := range []any{su, du, br} {
		data, _ := json.Marshal(item)
		lines = append(lines, data)
	}
	return lines
}

// BoundLiteral creates a bound value from a literal.
func BoundLiteral(v any) map[string]any {
	switch val := v.(type) {
	case string:
		return map[string]any{"literalString": val}
	case int:
		return map[string]any{"literalNumber": val}
	case float64:
		return map[string]any{"literalNumber": val}
	default:
		return map[string]any{"literalString": fmt.Sprintf("%v", val)}
	}
}

// BoundPath creates a bound value referencing a data model path.
func BoundPath(p string) map[string]any {
	return map[string]any{"path": p}
}

// BoundLiteralAndPath creates a bound value with both a literal fallback and a path.
func BoundLiteralAndPath(v string, p string) map[string]any {
	return map[string]any{"literalString": v, "path": p}
}

// a2uiToEmbeddedResource converts A2UI JSONL lines into an mcp.EmbeddedResource
// suitable for inclusion in a CallToolResult.Content slice.
func a2uiToEmbeddedResource(lines []json.RawMessage) *mcp.EmbeddedResource {
	var parts []string
	for _, line := range lines {
		parts = append(parts, string(line))
	}
	return &mcp.EmbeddedResource{
		Resource: &mcp.ResourceContents{
			URI:      "a2ui://surface",
			MIMEType: "application/json+a2ui",
			Text:     strings.Join(parts, "\n"),
		},
	}
}

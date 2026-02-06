package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Domain Models ---

type Memory struct {
	ID         string         `json:"id,omitempty"`
	Label      string         `json:"label" jsonschema:"description=The type/label of the memory in lowercase (person, place, project, skill, etc.)"`
	Name       string         `json:"name" jsonschema:"description=The name or title of the memory"`
	Properties map[string]any `json:"properties,omitempty" jsonschema:"description=Key-value pairs of properties associated with the memory"`
	CreatedAt  string         `json:"created_at,omitempty"`
	UpdatedAt  string         `json:"updated_at,omitempty"`
}

type Connection struct {
	ID               string         `json:"id,omitempty"`
	FromMemoryID     string         `json:"from_memory_id" jsonschema:"description=The ID of the source memory"`
	ToMemoryID       string         `json:"to_memory_id" jsonschema:"description=The ID of the target memory"`
	RelationshipType string         `json:"relationship_type" jsonschema:"description=The type of relationship in UPPER_SNAKE_CASE (KNOWS, WORKS_AT, LIVES_IN, etc.)"`
	Properties       map[string]any `json:"properties,omitempty" jsonschema:"description=Key-value pairs of properties for the relationship"`
	CreatedAt        string         `json:"created_at,omitempty"`
	UpdatedAt        string         `json:"updated_at,omitempty"`
}

type LabelCount struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// --- Database Client using MCP ---

type DBClient struct {
	session *mcp.ClientSession
}

func NewDBClient(ctx context.Context, toolboxCmd string) (*DBClient, error) {
	parts := strings.Fields(toolboxCmd)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty toolbox command")
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "memory-server-db-client",
		Version: "1.0.0",
	}, nil)

	transport := &mcp.CommandTransport{
		Command: exec.Command(parts[0], parts[1:]...),
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to toolbox: %w", err)
	}

	return &DBClient{session: session}, nil
}

func (c *DBClient) Close() error {
	return c.session.Close()
}

func (c *DBClient) ExecuteSQL(ctx context.Context, sql string) ([]map[string]any, error) {
	result, err := c.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "execute_sql",
		Arguments: map[string]any{"sql": sql},
	})
	if err != nil {
		return nil, fmt.Errorf("call execute_sql: %w", err)
	}

	if result.IsError {
		if len(result.Content) > 0 {
			if tc, ok := result.Content[0].(*mcp.TextContent); ok {
				return nil, fmt.Errorf("SQL error: %s", tc.Text)
			}
		}
		return nil, fmt.Errorf("SQL execution failed")
	}

	if len(result.Content) == 0 {
		return nil, nil
	}

	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return nil, nil
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &rows); err != nil {
		return nil, nil
	}
	return rows, nil
}

// --- Manager Logic ---

type MemoryManager struct {
	db *DBClient
}

func NewMemoryManager(db *DBClient) *MemoryManager {
	return &MemoryManager{db: db}
}

func (m *MemoryManager) SearchMemories(ctx context.Context, input SearchMemoriesInput) (*SearchMemoriesOutput, error) {
	var conditions []string
	var joins []string

	if input.Query != "" {
		words := strings.Fields(input.Query)
		var wordConditions []string
		for _, word := range words {
			escapedWord := escapeSQLString(strings.ToLower(word))
			wordConditions = append(wordConditions, fmt.Sprintf(
				`(LOWER(m.name) LIKE '%%%s%%' OR LOWER(m.label) LIKE '%%%s%%' OR LOWER(COALESCE(m.properties::text, '')) LIKE '%%%s%%')`,
				escapedWord, escapedWord, escapedWord,
			))
		}
		if len(wordConditions) > 0 {
			conditions = append(conditions, "("+strings.Join(wordConditions, " OR ")+")")
		}
	}

	if input.Label != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(m.label) = '%s'", escapeSQLString(strings.ToLower(input.Label))))
	}

	if input.SinceDate != "" {
		conditions = append(conditions, fmt.Sprintf("m.created_at >= '%s'", escapeSQLString(input.SinceDate)))
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	joinClause := ""
	if len(joins) > 0 {
		joinClause = strings.Join(joins, " ")
	}

	orderBy := "m.created_at DESC"
	if input.SortBy != "" {
		orderBy = fmt.Sprintf("m.%s", escapeSQLString(input.SortBy))
	}

	limit := 50
	if input.Limit > 0 && input.Limit <= 100 {
		limit = input.Limit
	}

	sql := fmt.Sprintf(
		`SELECT DISTINCT m.id, m.label, m.name, m.properties, m.created_at, m.updated_at
		 FROM memories m %s %s
		 ORDER BY %s
		 LIMIT %d`,
		joinClause, whereClause, orderBy, limit,
	)

	rows, err := m.db.ExecuteSQL(ctx, sql)
	if err != nil {
		return nil, err
	}

	var memories []Memory
	memoryIDs := make(map[string]bool)
	for _, row := range rows {
		mem := rowToMemory(row)
		memories = append(memories, mem)
		memoryIDs[mem.ID] = true
	}

	var connections []Connection
	if input.Depth > 0 && len(memoryIDs) > 0 {
		connections, err = m.loadConnectionsForMemories(ctx, memoryIDs, input.Depth)
		if err != nil {
			return nil, err
		}
	}

	return &SearchMemoriesOutput{Memories: memories, Connections: connections}, nil
}

func (m *MemoryManager) CreateMemory(ctx context.Context, input CreateMemoryInput) (*Memory, error) {
	propsJSON := "{}"
	if input.Properties != nil {
		b, err := json.Marshal(input.Properties)
		if err != nil {
			return nil, fmt.Errorf("marshal properties: %w", err)
		}
		propsJSON = string(b)
	}

	sql := fmt.Sprintf(
		`INSERT INTO memories (label, name, properties) VALUES ('%s', '%s', '%s') RETURNING id, label, name, properties, created_at, updated_at`,
		escapeSQLString(strings.ToLower(input.Label)),
		escapeSQLString(input.Name),
		escapeSQLString(propsJSON),
	)

	rows, err := m.db.ExecuteSQL(ctx, sql)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("failed to create memory")
	}

	mem := rowToMemory(rows[0])
	return &mem, nil
}

func (m *MemoryManager) CreateConnection(ctx context.Context, input CreateConnectionInput) (*Connection, error) {
	propsJSON := "{}"
	if input.Properties != nil {
		b, err := json.Marshal(input.Properties)
		if err != nil {
			return nil, fmt.Errorf("marshal properties: %w", err)
		}
		propsJSON = string(b)
	}

	sql := fmt.Sprintf(
		`INSERT INTO connections (from_memory_id, to_memory_id, relationship_type, properties)
		 VALUES ('%s', '%s', '%s', '%s')
		 ON CONFLICT (from_memory_id, to_memory_id, relationship_type) DO UPDATE SET properties = EXCLUDED.properties, updated_at = NOW()
		 RETURNING id, from_memory_id, to_memory_id, relationship_type, properties, created_at, updated_at`,
		escapeSQLString(input.FromMemoryID),
		escapeSQLString(input.ToMemoryID),
		escapeSQLString(strings.ToUpper(input.RelationshipType)),
		escapeSQLString(propsJSON),
	)

	rows, err := m.db.ExecuteSQL(ctx, sql)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("failed to create connection")
	}

	conn := rowToConnection(rows[0])
	return &conn, nil
}

func (m *MemoryManager) UpdateMemory(ctx context.Context, input UpdateMemoryInput) (*Memory, error) {
	if len(input.Properties) == 0 {
		return nil, fmt.Errorf("no properties to update")
	}

	currentSQL := fmt.Sprintf(`SELECT properties FROM memories WHERE id = '%s'`, escapeSQLString(input.ID))
	currentRows, err := m.db.ExecuteSQL(ctx, currentSQL)
	if err != nil {
		return nil, err
	}
	if len(currentRows) == 0 {
		return nil, fmt.Errorf("memory not found: %s", input.ID)
	}

	currentProps := make(map[string]any)
	if propsRaw, ok := currentRows[0]["properties"]; ok {
		switch v := propsRaw.(type) {
		case map[string]any:
			currentProps = v
		case string:
			json.Unmarshal([]byte(v), &currentProps)
		}
	}

	for k, v := range input.Properties {
		if v == nil {
			delete(currentProps, k)
		} else {
			currentProps[k] = v
		}
	}

	propsJSON, err := json.Marshal(currentProps)
	if err != nil {
		return nil, fmt.Errorf("marshal properties: %w", err)
	}

	sql := fmt.Sprintf(
		`UPDATE memories SET properties = '%s', updated_at = NOW() WHERE id = '%s' RETURNING id, label, name, properties, created_at, updated_at`,
		escapeSQLString(string(propsJSON)),
		escapeSQLString(input.ID),
	)

	rows, err := m.db.ExecuteSQL(ctx, sql)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("memory not found: %s", input.ID)
	}

	mem := rowToMemory(rows[0])
	return &mem, nil
}

func (m *MemoryManager) UpdateConnection(ctx context.Context, input UpdateConnectionInput) (*Connection, error) {
	if len(input.Properties) == 0 {
		return nil, fmt.Errorf("no properties to update")
	}

	currentSQL := fmt.Sprintf(`SELECT properties FROM connections WHERE id = '%s'`, escapeSQLString(input.ID))
	currentRows, err := m.db.ExecuteSQL(ctx, currentSQL)
	if err != nil {
		return nil, err
	}
	if len(currentRows) == 0 {
		return nil, fmt.Errorf("connection not found: %s", input.ID)
	}

	currentProps := make(map[string]any)
	if propsRaw, ok := currentRows[0]["properties"]; ok {
		switch v := propsRaw.(type) {
		case map[string]any:
			currentProps = v
		case string:
			json.Unmarshal([]byte(v), &currentProps)
		}
	}

	for k, v := range input.Properties {
		if v == nil {
			delete(currentProps, k)
		} else {
			currentProps[k] = v
		}
	}

	propsJSON, err := json.Marshal(currentProps)
	if err != nil {
		return nil, fmt.Errorf("marshal properties: %w", err)
	}

	sql := fmt.Sprintf(
		`UPDATE connections SET properties = '%s', updated_at = NOW() WHERE id = '%s' RETURNING id, from_memory_id, to_memory_id, relationship_type, properties, created_at, updated_at`,
		escapeSQLString(string(propsJSON)),
		escapeSQLString(input.ID),
	)

	rows, err := m.db.ExecuteSQL(ctx, sql)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("connection not found: %s", input.ID)
	}

	conn := rowToConnection(rows[0])
	return &conn, nil
}

func (m *MemoryManager) DeleteMemory(ctx context.Context, id string) error {
	sql := fmt.Sprintf(`DELETE FROM memories WHERE id = '%s'`, escapeSQLString(id))
	_, err := m.db.ExecuteSQL(ctx, sql)
	return err
}

func (m *MemoryManager) DeleteConnection(ctx context.Context, id string) error {
	sql := fmt.Sprintf(`DELETE FROM connections WHERE id = '%s'`, escapeSQLString(id))
	_, err := m.db.ExecuteSQL(ctx, sql)
	return err
}

func (m *MemoryManager) ListMemoryLabels(ctx context.Context) ([]LabelCount, error) {
	sql := `SELECT label, COUNT(*) as count FROM memories GROUP BY label ORDER BY count DESC`
	rows, err := m.db.ExecuteSQL(ctx, sql)
	if err != nil {
		return nil, err
	}

	var labels []LabelCount
	for _, row := range rows {
		label := getString(row, "label")
		count := 0
		if c, ok := row["count"].(float64); ok {
			count = int(c)
		} else if c, ok := row["count"].(int); ok {
			count = c
		}
		labels = append(labels, LabelCount{Label: label, Count: count})
	}
	return labels, nil
}

func (m *MemoryManager) loadConnectionsForMemories(ctx context.Context, memoryIDs map[string]bool, depth int) ([]Connection, error) {
	if len(memoryIDs) == 0 || depth <= 0 {
		return nil, nil
	}

	idList := make([]string, 0, len(memoryIDs))
	for id := range memoryIDs {
		idList = append(idList, fmt.Sprintf("'%s'", escapeSQLString(id)))
	}
	idIn := strings.Join(idList, ",")

	sql := fmt.Sprintf(
		`SELECT id, from_memory_id, to_memory_id, relationship_type, properties, created_at, updated_at
		 FROM connections
		 WHERE from_memory_id IN (%s) OR to_memory_id IN (%s)`,
		idIn, idIn,
	)

	rows, err := m.db.ExecuteSQL(ctx, sql)
	if err != nil {
		return nil, err
	}

	var connections []Connection
	for _, row := range rows {
		connections = append(connections, rowToConnection(row))
	}
	return connections, nil
}

// --- Helpers ---

func escapeSQLString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func getString(row map[string]any, key string) string {
	if v, ok := row[key].(string); ok {
		return v
	}
	return ""
}

func rowToMemory(row map[string]any) Memory {
	mem := Memory{
		ID:        getString(row, "id"),
		Label:     getString(row, "label"),
		Name:      getString(row, "name"),
		CreatedAt: getString(row, "created_at"),
		UpdatedAt: getString(row, "updated_at"),
	}

	if propsRaw, ok := row["properties"]; ok {
		switch v := propsRaw.(type) {
		case map[string]any:
			mem.Properties = v
		case string:
			var props map[string]any
			if err := json.Unmarshal([]byte(v), &props); err == nil {
				mem.Properties = props
			}
		}
	}

	return mem
}

func rowToConnection(row map[string]any) Connection {
	conn := Connection{
		ID:               getString(row, "id"),
		FromMemoryID:     getString(row, "from_memory_id"),
		ToMemoryID:       getString(row, "to_memory_id"),
		RelationshipType: getString(row, "relationship_type"),
		CreatedAt:        getString(row, "created_at"),
		UpdatedAt:        getString(row, "updated_at"),
	}

	if propsRaw, ok := row["properties"]; ok {
		switch v := propsRaw.(type) {
		case map[string]any:
			conn.Properties = v
		case string:
			var props map[string]any
			if err := json.Unmarshal([]byte(v), &props); err == nil {
				conn.Properties = props
			}
		}
	}

	return conn
}

func getGuidance(topic string) string {
	guidance := map[string]string{
		"labels": `## Memory Labels Best Practices

Labels should be lowercase singular nouns that describe the type of memory:
- person, place, project, skill, company, event, document, idea, task, goal

Common labels:
- person: People you know or interact with
- place: Locations, addresses, venues
- project: Work projects, personal projects
- company: Organizations, businesses
- skill: Technologies, languages, abilities
- event: Meetings, appointments, occasions
- document: Files, articles, books
- idea: Concepts, thoughts, plans
- task: Action items, to-dos
- goal: Objectives, targets

Use list_memory_labels to see what labels are already in use and maintain consistency.`,

		"relationships": `## Relationship Types Best Practices

Relationships should be UPPER_SNAKE_CASE verbs in active voice:
- KNOWS, WORKS_AT, LIVES_IN, MANAGES, REPORTS_TO, CREATED, OWNS, USES

Common relationship types:
- KNOWS: Person knows another person
- WORKS_AT: Person works at company/organization
- LIVES_IN: Person lives in place
- MANAGES: Person manages project/team/person
- REPORTS_TO: Person reports to another person
- CREATED: Person/entity created something
- OWNS: Ownership relationship
- USES: Uses a skill, tool, or technology
- PART_OF: Component of a larger whole
- RELATED_TO: General association

Always use active voice: "Alice MANAGES Project" not "Project IS_MANAGED_BY Alice"`,

		"best-practices": `## Memory Tools Best Practices

1. **Search before creating**: Always search_memories first to avoid duplicates
2. **Use consistent labels**: Check list_memory_labels for existing labels
3. **Add properties liberally**: Store relevant metadata as properties
4. **Create meaningful connections**: Build a rich knowledge graph
5. **Use descriptive relationship types**: Be specific about the relationship
6. **Update over delete**: Prefer updating memories to deleting them
7. **Include timestamps**: Use since_date for temporal filtering
8. **Word-based search**: Queries match ANY word, use specific terms`,

		"examples": `## Example Usage

### Create a person memory:
{
  "label": "person",
  "name": "Alice Johnson",
  "properties": {
    "email": "alice@example.com",
    "role": "Software Engineer",
    "team": "Platform"
  }
}

### Create a connection:
{
  "from_memory_id": "<alice-id>",
  "to_memory_id": "<company-id>",
  "relationship_type": "WORKS_AT",
  "properties": {
    "since": "2023-01-15",
    "role": "Senior Engineer"
  }
}

### Search memories:
{
  "query": "Alice engineer",
  "label": "person",
  "depth": 1
}`,
	}

	if g, ok := guidance[topic]; ok {
		return g
	}

	return `## Available Guidance Topics

Use get_guidance with one of these topics:
- labels: Best practices for memory labels
- relationships: Best practices for relationship types  
- best-practices: General usage best practices
- examples: Example tool usage`
}

// --- Tool Input/Output Structs ---

type SearchMemoriesInput struct {
	Query     string `json:"query,omitempty" jsonschema:"description=Search query - matches ANY word in name, label, or properties"`
	Label     string `json:"label,omitempty" jsonschema:"description=Filter by memory type (person, place, project, etc.)"`
	SinceDate string `json:"since_date,omitempty" jsonschema:"description=Filter memories created since this date (ISO format: 2024-01-15)"`
	Depth     int    `json:"depth,omitempty" jsonschema:"description=Relationship traversal depth (0=no connections, 1=direct connections)"`
	Limit     int    `json:"limit,omitempty" jsonschema:"description=Maximum number of results (default 50, max 100)"`
	SortBy    string `json:"sort_by,omitempty" jsonschema:"description=Sort by field (created_at, name, label)"`
}

type SearchMemoriesOutput struct {
	Memories    []Memory     `json:"memories"`
	Connections []Connection `json:"connections,omitempty"`
}

type CreateMemoryInput struct {
	Label      string         `json:"label" jsonschema:"description=The type/label in lowercase (person, place, project, skill, etc.)"`
	Name       string         `json:"name" jsonschema:"description=The name or title of the memory"`
	Properties map[string]any `json:"properties,omitempty" jsonschema:"description=Key-value pairs of properties"`
}

type CreateConnectionInput struct {
	FromMemoryID     string         `json:"from_memory_id" jsonschema:"description=The ID of the source memory"`
	ToMemoryID       string         `json:"to_memory_id" jsonschema:"description=The ID of the target memory"`
	RelationshipType string         `json:"relationship_type" jsonschema:"description=The relationship type in UPPER_SNAKE_CASE (KNOWS, WORKS_AT, etc.)"`
	Properties       map[string]any `json:"properties,omitempty" jsonschema:"description=Key-value pairs of relationship properties"`
}

type UpdateMemoryInput struct {
	ID         string         `json:"id" jsonschema:"description=The ID of the memory to update"`
	Properties map[string]any `json:"properties" jsonschema:"description=Properties to add/update (set to null to remove)"`
}

type UpdateConnectionInput struct {
	ID         string         `json:"id" jsonschema:"description=The ID of the connection to update"`
	Properties map[string]any `json:"properties" jsonschema:"description=Properties to add/update (set to null to remove)"`
}

type DeleteMemoryInput struct {
	ID string `json:"id" jsonschema:"description=The ID of the memory to delete"`
}

type DeleteConnectionInput struct {
	ID string `json:"id" jsonschema:"description=The ID of the connection to delete"`
}

type ListMemoryLabelsOutput struct {
	Labels []LabelCount `json:"labels"`
}

type GetGuidanceInput struct {
	Topic string `json:"topic,omitempty" jsonschema:"description=Topic: labels, relationships, best-practices, examples"`
}

type EmptyInput struct{}

// --- Main ---

var manager *MemoryManager

func main() {
	ctx := context.Background()

	toolboxCmd := os.Getenv("TOOLBOX_MCP_CMD")
	if toolboxCmd == "" {
		toolboxCmd = "toolbox --tools-file tools.yaml"
	}

	dbClient, err := NewDBClient(ctx, toolboxCmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to database MCP server: %v\n", err)
		os.Exit(1)
	}
	defer dbClient.Close()

	manager = NewMemoryManager(dbClient)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "memory-server",
		Version: "2.0.0",
	}, nil)

	// --- Register Tools ---

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "search_memories",
			Description: "Search and retrieve memories from the knowledge graph. Word-based search matches ANY word in query. Filter by label and date, control relationship depth.",
		},
		func(ctx context.Context, req *mcp.CallToolRequest, input SearchMemoriesInput) (*mcp.CallToolResult, SearchMemoriesOutput, error) {
			result, err := manager.SearchMemories(ctx, input)
			if err != nil {
				return nil, SearchMemoriesOutput{}, err
			}
			return nil, *result, nil
		},
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "create_memory",
			Description: "Create a new memory in the knowledge graph with a label (type), name, and optional properties.",
		},
		func(ctx context.Context, req *mcp.CallToolRequest, input CreateMemoryInput) (*mcp.CallToolResult, Memory, error) {
			result, err := manager.CreateMemory(ctx, input)
			if err != nil {
				return nil, Memory{}, err
			}
			return nil, *result, nil
		},
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "create_connection",
			Description: "Create a relationship between two memories using semantic relationship types (KNOWS, WORKS_AT, LIVES_IN, etc.).",
		},
		func(ctx context.Context, req *mcp.CallToolRequest, input CreateConnectionInput) (*mcp.CallToolResult, Connection, error) {
			result, err := manager.CreateConnection(ctx, input)
			if err != nil {
				return nil, Connection{}, err
			}
			return nil, *result, nil
		},
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "update_memory",
			Description: "Update properties of an existing memory. Set properties to null to remove them.",
		},
		func(ctx context.Context, req *mcp.CallToolRequest, input UpdateMemoryInput) (*mcp.CallToolResult, Memory, error) {
			result, err := manager.UpdateMemory(ctx, input)
			if err != nil {
				return nil, Memory{}, err
			}
			return nil, *result, nil
		},
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "update_connection",
			Description: "Update properties of an existing connection. Set properties to null to remove them.",
		},
		func(ctx context.Context, req *mcp.CallToolRequest, input UpdateConnectionInput) (*mcp.CallToolResult, Connection, error) {
			result, err := manager.UpdateConnection(ctx, input)
			if err != nil {
				return nil, Connection{}, err
			}
			return nil, *result, nil
		},
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "delete_memory",
			Description: "Delete a memory and all its connections permanently. Use with caution.",
		},
		func(ctx context.Context, req *mcp.CallToolRequest, input DeleteMemoryInput) (*mcp.CallToolResult, string, error) {
			err := manager.DeleteMemory(ctx, input.ID)
			if err != nil {
				return nil, "", err
			}
			return nil, "Memory deleted successfully", nil
		},
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "delete_connection",
			Description: "Delete a specific connection between memories. The memories remain intact.",
		},
		func(ctx context.Context, req *mcp.CallToolRequest, input DeleteConnectionInput) (*mcp.CallToolResult, string, error) {
			err := manager.DeleteConnection(ctx, input.ID)
			if err != nil {
				return nil, "", err
			}
			return nil, "Connection deleted successfully", nil
		},
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "list_memory_labels",
			Description: "List all unique memory labels in use with counts. Helps maintain consistency and prevents duplicate label variations.",
		},
		func(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, ListMemoryLabelsOutput, error) {
			result, err := manager.ListMemoryLabels(ctx)
			if err != nil {
				return nil, ListMemoryLabelsOutput{}, err
			}
			return nil, ListMemoryLabelsOutput{Labels: result}, nil
		},
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "get_guidance",
			Description: "Get help on using memory tools effectively. Topics: labels, relationships, best-practices, examples.",
		},
		func(ctx context.Context, req *mcp.CallToolRequest, input GetGuidanceInput) (*mcp.CallToolResult, string, error) {
			return nil, getGuidance(input.Topic), nil
		},
	)

	port := os.Getenv("MCP_PORT")
	if port == "" {
		port = "8080"
	}

	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return server }, nil)

	fmt.Fprintf(os.Stderr, "MCP memory server listening on :%s\n", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

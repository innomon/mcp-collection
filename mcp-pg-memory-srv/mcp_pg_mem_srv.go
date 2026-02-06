package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

// --- Database Client using pgx ---

type DBClient struct {
	pool *pgxpool.Pool
}

func NewDBClient(ctx context.Context, connString string) (*DBClient, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &DBClient{pool: pool}, nil
}

func (c *DBClient) Close() {
	c.pool.Close()
}

func scanMemory(row pgx.Row) (Memory, error) {
	var mem Memory
	var propsJSON []byte
	var createdAt, updatedAt *string
	err := row.Scan(&mem.ID, &mem.Label, &mem.Name, &propsJSON, &createdAt, &updatedAt)
	if err != nil {
		return Memory{}, err
	}
	if len(propsJSON) > 0 {
		json.Unmarshal(propsJSON, &mem.Properties)
	}
	if createdAt != nil {
		mem.CreatedAt = *createdAt
	}
	if updatedAt != nil {
		mem.UpdatedAt = *updatedAt
	}
	return mem, nil
}

func scanConnection(row pgx.Row) (Connection, error) {
	var conn Connection
	var propsJSON []byte
	var createdAt, updatedAt *string
	err := row.Scan(&conn.ID, &conn.FromMemoryID, &conn.ToMemoryID, &conn.RelationshipType, &propsJSON, &createdAt, &updatedAt)
	if err != nil {
		return Connection{}, err
	}
	if len(propsJSON) > 0 {
		json.Unmarshal(propsJSON, &conn.Properties)
	}
	if createdAt != nil {
		conn.CreatedAt = *createdAt
	}
	if updatedAt != nil {
		conn.UpdatedAt = *updatedAt
	}
	return conn, nil
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
	var args []any
	argIdx := 1

	if input.Query != "" {
		words := strings.Fields(input.Query)
		var wordConditions []string
		for _, word := range words {
			pattern := "%" + strings.ToLower(word) + "%"
			wordConditions = append(wordConditions, fmt.Sprintf(
				`(LOWER(m.name) LIKE $%d OR LOWER(m.label) LIKE $%d OR LOWER(COALESCE(m.properties::text, '')) LIKE $%d)`,
				argIdx, argIdx, argIdx,
			))
			args = append(args, pattern)
			argIdx++
		}
		if len(wordConditions) > 0 {
			conditions = append(conditions, "("+strings.Join(wordConditions, " OR ")+")")
		}
	}

	if input.Label != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(m.label) = $%d", argIdx))
		args = append(args, strings.ToLower(input.Label))
		argIdx++
	}

	if input.SinceDate != "" {
		conditions = append(conditions, fmt.Sprintf("m.created_at >= $%d", argIdx))
		args = append(args, input.SinceDate)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	orderBy := "m.created_at DESC"
	allowedSorts := map[string]bool{"created_at": true, "name": true, "label": true}
	if input.SortBy != "" && allowedSorts[input.SortBy] {
		orderBy = "m." + input.SortBy
	}

	limit := 50
	if input.Limit > 0 && input.Limit <= 100 {
		limit = input.Limit
	}

	query := fmt.Sprintf(
		`SELECT DISTINCT m.id, m.label, m.name, m.properties, m.created_at::text, m.updated_at::text
		 FROM memories m %s
		 ORDER BY %s
		 LIMIT %d`,
		whereClause, orderBy, limit,
	)

	rows, err := m.db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}
	defer rows.Close()

	var memories []Memory
	memoryIDs := make(map[string]bool)
	for rows.Next() {
		mem, err := scanMemory(rows)
		if err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		memories = append(memories, mem)
		memoryIDs[mem.ID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memories: %w", err)
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
	propsJSON := []byte("{}")
	if input.Properties != nil {
		b, err := json.Marshal(input.Properties)
		if err != nil {
			return nil, fmt.Errorf("marshal properties: %w", err)
		}
		propsJSON = b
	}

	row := m.db.pool.QueryRow(ctx,
		`INSERT INTO memories (label, name, properties) VALUES ($1, $2, $3)
		 RETURNING id, label, name, properties, created_at::text, updated_at::text`,
		strings.ToLower(input.Label), input.Name, propsJSON,
	)

	mem, err := scanMemory(row)
	if err != nil {
		return nil, fmt.Errorf("create memory: %w", err)
	}
	return &mem, nil
}

func (m *MemoryManager) CreateConnection(ctx context.Context, input CreateConnectionInput) (*Connection, error) {
	propsJSON := []byte("{}")
	if input.Properties != nil {
		b, err := json.Marshal(input.Properties)
		if err != nil {
			return nil, fmt.Errorf("marshal properties: %w", err)
		}
		propsJSON = b
	}

	row := m.db.pool.QueryRow(ctx,
		`INSERT INTO connections (from_memory_id, to_memory_id, relationship_type, properties)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (from_memory_id, to_memory_id, relationship_type) DO UPDATE SET properties = EXCLUDED.properties, updated_at = NOW()
		 RETURNING id, from_memory_id, to_memory_id, relationship_type, properties, created_at::text, updated_at::text`,
		input.FromMemoryID, input.ToMemoryID, strings.ToUpper(input.RelationshipType), propsJSON,
	)

	conn, err := scanConnection(row)
	if err != nil {
		return nil, fmt.Errorf("create connection: %w", err)
	}
	return &conn, nil
}

func (m *MemoryManager) UpdateMemory(ctx context.Context, input UpdateMemoryInput) (*Memory, error) {
	if len(input.Properties) == 0 {
		return nil, fmt.Errorf("no properties to update")
	}

	var currentPropsJSON []byte
	err := m.db.pool.QueryRow(ctx, `SELECT properties FROM memories WHERE id = $1`, input.ID).Scan(&currentPropsJSON)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("memory not found: %s", input.ID)
		}
		return nil, fmt.Errorf("fetch memory: %w", err)
	}

	currentProps := make(map[string]any)
	if len(currentPropsJSON) > 0 {
		json.Unmarshal(currentPropsJSON, &currentProps)
	}

	for k, v := range input.Properties {
		if v == nil {
			delete(currentProps, k)
		} else {
			currentProps[k] = v
		}
	}

	mergedJSON, err := json.Marshal(currentProps)
	if err != nil {
		return nil, fmt.Errorf("marshal properties: %w", err)
	}

	row := m.db.pool.QueryRow(ctx,
		`UPDATE memories SET properties = $1, updated_at = NOW() WHERE id = $2
		 RETURNING id, label, name, properties, created_at::text, updated_at::text`,
		mergedJSON, input.ID,
	)

	mem, err := scanMemory(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("memory not found: %s", input.ID)
		}
		return nil, fmt.Errorf("update memory: %w", err)
	}
	return &mem, nil
}

func (m *MemoryManager) UpdateConnection(ctx context.Context, input UpdateConnectionInput) (*Connection, error) {
	if len(input.Properties) == 0 {
		return nil, fmt.Errorf("no properties to update")
	}

	var currentPropsJSON []byte
	err := m.db.pool.QueryRow(ctx, `SELECT properties FROM connections WHERE id = $1`, input.ID).Scan(&currentPropsJSON)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("connection not found: %s", input.ID)
		}
		return nil, fmt.Errorf("fetch connection: %w", err)
	}

	currentProps := make(map[string]any)
	if len(currentPropsJSON) > 0 {
		json.Unmarshal(currentPropsJSON, &currentProps)
	}

	for k, v := range input.Properties {
		if v == nil {
			delete(currentProps, k)
		} else {
			currentProps[k] = v
		}
	}

	mergedJSON, err := json.Marshal(currentProps)
	if err != nil {
		return nil, fmt.Errorf("marshal properties: %w", err)
	}

	row := m.db.pool.QueryRow(ctx,
		`UPDATE connections SET properties = $1, updated_at = NOW() WHERE id = $2
		 RETURNING id, from_memory_id, to_memory_id, relationship_type, properties, created_at::text, updated_at::text`,
		mergedJSON, input.ID,
	)

	conn, err := scanConnection(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("connection not found: %s", input.ID)
		}
		return nil, fmt.Errorf("update connection: %w", err)
	}
	return &conn, nil
}

func (m *MemoryManager) DeleteMemory(ctx context.Context, id string) error {
	_, err := m.db.pool.Exec(ctx, `DELETE FROM memories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	return nil
}

func (m *MemoryManager) DeleteConnection(ctx context.Context, id string) error {
	_, err := m.db.pool.Exec(ctx, `DELETE FROM connections WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}
	return nil
}

func (m *MemoryManager) ListMemoryLabels(ctx context.Context) ([]LabelCount, error) {
	rows, err := m.db.pool.Query(ctx, `SELECT label, COUNT(*) as count FROM memories GROUP BY label ORDER BY count DESC`)
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	defer rows.Close()

	var labels []LabelCount
	for rows.Next() {
		var lc LabelCount
		if err := rows.Scan(&lc.Label, &lc.Count); err != nil {
			return nil, fmt.Errorf("scan label: %w", err)
		}
		labels = append(labels, lc)
	}
	return labels, rows.Err()
}

func (m *MemoryManager) loadConnectionsForMemories(ctx context.Context, memoryIDs map[string]bool, depth int) ([]Connection, error) {
	if len(memoryIDs) == 0 || depth <= 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(memoryIDs))
	for id := range memoryIDs {
		ids = append(ids, id)
	}

	rows, err := m.db.pool.Query(ctx,
		`SELECT id, from_memory_id, to_memory_id, relationship_type, properties, created_at::text, updated_at::text
		 FROM connections
		 WHERE from_memory_id = ANY($1) OR to_memory_id = ANY($1)`,
		ids,
	)
	if err != nil {
		return nil, fmt.Errorf("load connections: %w", err)
	}
	defer rows.Close()

	var connections []Connection
	for rows.Next() {
		conn, err := scanConnection(rows)
		if err != nil {
			return nil, fmt.Errorf("scan connection: %w", err)
		}
		connections = append(connections, conn)
	}
	return connections, rows.Err()
}

// --- Guidance ---

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

	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		fmt.Fprintf(os.Stderr, "DATABASE_URL environment variable is required\n")
		os.Exit(1)
	}

	dbClient, err := NewDBClient(ctx, connString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to PostgreSQL: %v\n", err)
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

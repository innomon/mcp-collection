# Memory MCP Server (Go)

A Model Context Protocol server that provides knowledge graph management capabilities. This server enables LLMs to create, read, update, and delete memories and connections in a persistent knowledge graph, helping AI assistants maintain memory across conversations.

## Features

- **Memory Management**: Create and track memories with flexible labels and properties.
- **Connection Mapping**: Define semantic relationships between memories.
- **Property Storage**: Attach arbitrary key-value properties to memories and connections.
- **Graph Search**: Word-based search across all memory properties with filtering.
- **Persistence**: Stores the knowledge graph in PostgreSQL via [genai-toolbox](https://github.com/googleapis/genai-toolbox) MCP server.

## Tools

| Tool | Description |
|------|-------------|
| `search_memories` | Search and retrieve memories with word-based matching, filtering by label/date |
| `create_memory` | Create a new memory with label, name, and properties |
| `create_connection` | Create relationships between memories (KNOWS, WORKS_AT, etc.) |
| `update_memory` | Update properties of existing memories |
| `update_connection` | Update relationship properties |
| `delete_memory` | Remove memories and all their connections |
| `delete_connection` | Remove specific relationships |
| `list_memory_labels` | List all unique memory labels with counts |
| `get_guidance` | Get help on labels, relationships, best-practices, examples |

### Tool Details

- **search_memories**: Word-based search matches ANY word in query (e.g., "Ben Weeks" finds memories containing "Ben" OR "Weeks"). Filter by memory type, date with `since_date`, control relationship depth and result limits, sort by any field.

- **create_memory**: Flexible type system using lowercase labels (person, place, project, skill, etc.). Store any properties as key-value pairs with automatic timestamps.

- **create_connection**: Link memories using semantic relationship types in UPPER_SNAKE_CASE (KNOWS, WORKS_AT, LIVES_IN, etc.). Add properties to relationships (since, role, status).

- **update_memory** / **update_connection**: Add or modify any property. Set properties to null to remove them.

- **delete_memory**: Permanent deletion that automatically removes all relationships.

- **delete_connection**: Precise relationship removal while keeping memories intact.

- **list_memory_labels**: Shows all labels with counts to maintain consistency and prevent duplicate label variations.

- **get_guidance**: Returns comprehensive guidance for LLMs. Topics: labels, relationships, best-practices, examples.

## Architecture

```
┌─────────────────┐     MCP      ┌─────────────────┐     SQL      ┌──────────────┐
│  Claude/LLM     │◄────────────►│  Memory Server  │◄────────────►│  Toolbox     │
│  (MCP Client)   │   stdio      │  (MCP Server)   │   MCP        │  (postgres)  │
└─────────────────┘              └─────────────────┘              └──────┬───────┘
                                                                         │
                                                                         ▼
                                                                  ┌──────────────┐
                                                                  │  Doltgres/   │
                                                                  │  PostgreSQL  │
                                                                  └──────────────┘
```

The memory server acts as:
1. An **MCP server** exposing memory tools to LLM clients
2. An **MCP client** connecting to genai-toolbox for database operations

## Installation

### Prerequisites

- Go 1.24+
- [genai-toolbox](https://github.com/googleapis/genai-toolbox/releases) installed
- PostgreSQL-compatible database (Doltgres or PostgreSQL)

### Build from source

```bash
go build -o mcp-memory-server ./cmd/mcp-memory-server
```

### Database Setup

Run the migration to create the memory tables:

```bash
psql $DATABASE_URL -f migrations/003_memories.sql
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TOOLBOX_MCP_CMD` | Command to start genai-toolbox | `toolbox --tools-file tools.yaml` |
| `DB_HOST` | Database host (for toolbox) | - |
| `DB_PORT` | Database port (for toolbox) | `5432` |
| `DB_DATABASE` | Database name (for toolbox) | - |
| `DB_USER` | Database user (for toolbox) | - |
| `DB_PASSWORD` | Database password (for toolbox) | - |

### Toolbox Configuration

The included `tools.yaml` configures genai-toolbox to connect to PostgreSQL:

```yaml
sources:
  kg-db:
    kind: postgres
    host: ${DB_HOST}
    port: ${DB_PORT}
    database: ${DB_DATABASE}
    user: ${DB_USER}
    password: ${DB_PASSWORD}

tools:
  execute_sql:
    kind: postgres-sql
    source: kg-db
    description: Execute SQL queries against the memory database
    statement: $1
    parameters:
      - name: sql
        type: string
        description: The SQL statement to execute
```

## Usage with Claude Desktop

Add the following to your `claude_desktop_config.json`:

### macOS/Linux

```json
{
  "mcpServers": {
    "memory": {
      "command": "/path/to/mcp-memory-server",
      "env": {
        "TOOLBOX_MCP_CMD": "toolbox --tools-file /path/to/tools.yaml",
        "DB_HOST": "localhost",
        "DB_PORT": "5432",
        "DB_DATABASE": "whatschat",
        "DB_USER": "postgres",
        "DB_PASSWORD": "your-password"
      }
    }
  }
}
```

### Windows

```json
{
  "mcpServers": {
    "memory": {
      "command": "C:\\path\\to\\mcp-memory-server.exe",
      "env": {
        "TOOLBOX_MCP_CMD": "toolbox --tools-file C:\\path\\to\\tools.yaml",
        "DB_HOST": "localhost",
        "DB_PORT": "5432",
        "DB_DATABASE": "whatschat",
        "DB_USER": "postgres",
        "DB_PASSWORD": "your-password"
      }
    }
  }
}
```

## Database Schema

The memory graph is stored in two tables:

```sql
-- Memories (nodes)
memories (id, label, name, properties, created_at, updated_at)

-- Connections (edges)
connections (id, from_memory_id, to_memory_id, relationship_type, properties, created_at, updated_at)
```

See [migrations/003_memories.sql](../../migrations/003_memories.sql) for the full schema.

---
name: using-mcp-pg-memory-srv
description: Instructions for using the MCP PG Memory server to manage a knowledge graph of memories and connections via PostgreSQL. Use when the user wants to build, search, or update a network of entities and relationships.
---

# Using MCP PG Memory Server

This skill provides instructions for building and managing a knowledge graph using the `mcp-pg-memory-srv` server, which uses PostgreSQL to store memories and their connections.

## Core Concepts

- **Memory**: An entity with a `label` (lowercase type), `name` (title), and `properties` (JSON metadata).
- **Connection**: A relationship between two memories with a `from_memory_id`, `to_memory_id`, and `relationship_type` (UPPER_SNAKE_CASE).

## Best Practices

- **Search Before Creating**: Always use `search_memories` to verify if a memory already exists before creating a new one to prevent duplicates.
- **Consistent Labels**: Use `list_memory_labels` to see existing labels and maintain consistency (e.g., `person`, `project`, `skill`).
- **Semantic Relationships**: Use clear, active-voice relationship types like `WORKS_AT`, `MANAGES`, `KNOWS`, or `USES`.
- **Rich Metadata**: Store relevant attributes (e.g., email, role, technologies used) in the `properties` map of memories and connections.
- **Temporal Context**: Use the `since_date` parameter in `search_memories` to find recently added or updated information.

## Primary Workflows

### 1. Information Discovery
- List all labels to understand the graph's schema using `list_memory_labels`.
- Search for specific entities or topics using `search_memories`.
- Use the `depth` parameter (e.g., `depth: 1`) to retrieve direct connections for any matched memory.
- Get help on specific topics like `labels` or `relationships` using `get_guidance`.

### 2. Graph Construction
- Create new entities with `create_memory`, providing a descriptive name and appropriate label.
- Establish relationships with `create_connection`. Ensure the `from` and `to` IDs are correct and the relationship type is in `UPPER_SNAKE_CASE`.

### 3. Maintenance
- Refine existing information using `update_memory` or `update_connection` (provide a map of properties to merge; set a property to `null` to remove it).
- Prune incorrect or obsolete data using `delete_memory` (removes the memory and all its connections) or `delete_connection` (removes only the relationship).

## Example Schemas

### Person Memory
- **Label**: `person`
- **Name**: `John Doe`
- **Properties**: `{"role": "Lead Developer", "team": "Infrastructure"}`

### Project Memory
- **Label**: `project`
- **Name**: `Memory Graph`
- **Properties**: `{"status": "active", "priority": "high"}`

### Relationship
- **From**: `John Doe` (ID)
- **To**: `Memory Graph` (ID)
- **Type**: `WORKS_ON`
- **Properties**: `{"since": "2024-01-01"}`

# todo-service

A small Go microservice that exposes TODO CRUD over both HTTP REST and MCP tools.

## Features

- In-memory TODO store
- REST endpoints:
  - `POST /todos`
  - `GET /todos`
  - `PUT /todos/{id}`
  - `DELETE /todos/{id}`
- MCP tools:
  - `todo_create`
  - `todo_list`
  - `todo_update`
  - `todo_delete`

## Run

```bash
go run ./cmd/todo-service
```

By default the HTTP server listens on `:8080`.

## REST examples

Create:

```bash
curl -X POST http://localhost:8080/todos \
  -H 'Content-Type: application/json' \
  -d '{"title":"Buy milk"}'
```

List:

```bash
curl http://localhost:8080/todos
```

Update:

```bash
curl -X PUT http://localhost:8080/todos/1 \
  -H 'Content-Type: application/json' \
  -d '{"title":"Buy oat milk","completed":true}'
```

Delete:

```bash
curl -X DELETE http://localhost:8080/todos/1
```

## MCP transport

The service exposes MCP over Streamable HTTP at `/mcp` on the same port as the REST server.

Example MCP tool names:
- `todo_create`
- `todo_list`
- `todo_update`
- `todo_delete`

### MCP Session Manager

Use the MCP session manager to interact with MCP tools without manually managing sessions:

```bash
# List all todos
python3 internal/mcp_session_manager.py list

# Create a todo
python3 internal/mcp_session_manager.py create "Buy milk"

# Update a todo
python3 internal/mcp_session_manager.py update 1 title="Buy oat milk" completed=true

# Delete a todo
python3 internal/mcp_session_manager.py delete 1
```

The session manager:
- Automatically initializes and manages MCP sessions
- Persists session IDs to disk to avoid re-initialization
- Provides a clean Python API and CLI interface
- Handles the MCP protocol complexities transparently

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

### CLI Tool: todo-cli

The `todo-cli` tool provides a convenient command-line interface to interact with MCP tools.

**Build the CLI:**

```bash
go build -o todo-cli ./cmd/todo-cli
```

**Usage:**

```bash
# List all todos
./todo-cli list

# Create a todo
./todo-cli create "Buy milk"

# Update a todo
./todo-cli update 1 title="Buy oat milk" completed=true

# Delete a todo
./todo-cli delete 1
```

**Features:**
- Automatically initializes and manages MCP sessions
- Persists session IDs to disk to avoid re-initialization
- Handles the MCP protocol complexities transparently
- Pure Go implementation, consistent with the service

**Configuration:**

Set `TODO_SERVICE_URL` environment variable to connect to a different service instance:

```bash
export TODO_SERVICE_URL=http://example.com:8080
./todo-cli list
```

## Using with GitHub Copilot CLI

The todo-service can be integrated as an MCP server in GitHub Copilot CLI, giving the AI assistant access to todo management tools.

### Setup

1. **Start the todo-service** (ensure it's running on `localhost:8080`):
   ```bash
   go run ./cmd/todo-service
   ```

2. **Configure Copilot CLI** to use this MCP server. In your Copilot CLI session, use the `/mcp` command:
   ```
   /mcp add
   ```

3. **Add the server configuration**:
   - Name: `todo-service`
   - Type: `http`
   - URL: `http://localhost:8080/mcp`

### Using Todo Tools in Copilot

Once configured, you can ask Copilot to manage your todos naturally:

```
copilot> Create a todo to "Review pull requests"
copilot> List all my todos
copilot> Mark todo 1 as completed
copilot> Delete todo 3
```

Copilot will automatically call the appropriate todo tools to fulfill your requests.

### Example Workflow

```bash
# Terminal 1: Start the service
$ go run ./cmd/todo-service
todo-service listening on http://localhost:8080

# Terminal 2: Start Copilot with todo-service MCP configured
$ copilot
> /mcp add
# Configure todo-service as described above

> I need to track my project tasks. Create todos for: code review, write tests, and update docs
# Copilot uses todo_create tool to create three todos

> What's on my todo list?
# Copilot uses todo_list tool to show your todos

> Mark the first todo as done
# Copilot uses todo_update tool to mark it complete

> Remove the completed todo
# Copilot uses todo_delete tool to clean up
```

### MCP Server Details

- **Endpoint**: `http://localhost:8080/mcp`
- **Protocol**: JSON-RPC 2.0 over HTTP
- **Available Tools**:
  - `todo_create` - Create a new todo item
  - `todo_list` - List all todo items
  - `todo_update` - Update a todo (title or completion status)
  - `todo_delete` - Delete a todo

The service manages session state automatically, so Copilot can make multiple tool calls in sequence without manual session management.

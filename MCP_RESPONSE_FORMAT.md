# MCP Response Format Explained

The todo-service MCP endpoint uses Streamable HTTP with JSON-RPC 2.0.

Responses are plain JSON (`application/json`), not SSE-framed `event:` / `data:` lines.

The current Go MCP SDK handler still validates that POST requests include both
`application/json` and `text/event-stream` in the `Accept` header. Keep both
values in examples even though responses are JSON.

## Transport Layer

Each POST returns a JSON-RPC payload directly:

```json
{"jsonrpc":"2.0","id":1,"result":{...}}
```

Request expectations for this service:
- `Content-Type: application/json`
- `Accept: application/json, text/event-stream`

---

## 1. Initialize

**Request:**
```bash
curl -sD - -X POST http://localhost:8080/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize",...}'
```

**Response Headers:**
```
HTTP/1.1 200 OK
Content-Type: application/json
Mcp-Session-Id: YNIH3Q54VARVQAYTJLGSTMNM23
```

**Response Body (JSON-RPC):**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2024-11-05",
    "serverInfo": {
      "name": "todo-service",
      "version": "0.1.0"
    },
    "capabilities": {
      "logging": {},
      "tools": {
        "listChanged": true
      }
    }
  }
}
```

**Key Points:**
- Response is HTTP 200 with JSON body
- Extract **`Mcp-Session-Id`** header — you'll need this for all subsequent requests
- `result.serverInfo` identifies the server
- `result.capabilities.tools` indicates tools are available
- Even in JSON response mode, include both media types in `Accept`

---

## 2. Send Initialized Notification

**Request:**
```bash
curl -s -o /dev/null -X POST http://localhost:8080/mcp \
  -H 'Content-Type: application/json' \
  -H 'Mcp-Session-Id: YNIH3Q54VARVQAYTJLGSTMNM23' \
  -d '{"jsonrpc":"2.0","method":"notifications/initialized"}'
```

**Response:**
```
HTTP/1.1 202 Accepted
```

**Key Points:**
- Use the **`Mcp-Session-Id`** from initialize
- Notification (no `id` field in request)
- Returns HTTP 202 (no body)
- After this, the session is initialized and ready for tool calls

---

## 3. List Available Tools

**Request:**
```bash
curl -s -X POST http://localhost:8080/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -H 'Mcp-Session-Id: YNIH3Q54VARVQAYTJLGSTMNM23' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
```

**Response Body (JSON-RPC):**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "tools": [
      {
        "name": "todo_create",
        "description": "Create a todo item",
        "inputSchema": {
          "type": "object",
          "properties": {
            "title": { "type": "string" }
          },
          "required": ["title"],
          "additionalProperties": false
        },
        "outputSchema": { "type": "object" }
      },
      {
        "name": "todo_list",
        "description": "List all todo items",
        "inputSchema": { "type": "object" },
        "outputSchema": { "type": "object" }
      },
      {
        "name": "todo_update",
        "description": "Update a todo item",
        "inputSchema": {
          "type": "object",
          "properties": {
            "id": { "type": "integer" },
            "title": { "type": ["null", "string"] },
            "completed": { "type": ["null", "boolean"] }
          },
          "required": ["id"]
        }
      },
      {
        "name": "todo_delete",
        "description": "Delete a todo item",
        "inputSchema": {
          "type": "object",
          "properties": {
            "id": { "type": "integer" }
          },
          "required": ["id"]
        }
      }
    ]
  }
}
```

**Key Points:**
- `result.tools` is an array of tool definitions
- Each tool has `name`, `description`, `inputSchema` (what arguments it accepts), `outputSchema`
- `inputSchema.required` shows which fields are mandatory
- `additionalProperties: false` means strict schema validation

---

## 4. Call a Tool

**Request (todo_create):**
```bash
curl -s -X POST http://localhost:8080/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -H 'Mcp-Session-Id: YNIH3Q54VARVQAYTJLGSTMNM23' \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"todo_create","arguments":{"title":"Buy milk"}}}'
```

**Response Body (JSON-RPC):**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"id\":17,\"title\":\"Buy milk\",\"completed\":false}"
      }
    ],
    "structuredContent": {}
  }
}
```

**Parsing the Result:**
The actual tool result is nested in `result.content[0].text` as a JSON string:

```bash
# Extract and parse the tool result:
curl ... | \
  python3 -c "import sys,json; d=json.load(sys.stdin); print(d['result']['content'][0]['text'])" | \
  python3 -m json.tool
```

Output:
```json
{
  "id": 17,
  "title": "Buy milk",
  "completed": false
}
```

**Key Points:**
- `method: "tools/call"` with `params: {name, arguments}`
- Result is wrapped in `content[0].text` as a JSON string
- Always extract `result.content[0].text`, parse it, and then use the data

---

## 5. Tool Error Response

**Request (empty title):**
```bash
curl -s -X POST http://localhost:8080/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -H 'Mcp-Session-Id: YNIH3Q54VARVQAYTJLGSTMNM23' \
  -d '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"todo_create","arguments":{"title":"  "}}}'
```

**Response Body (JSON-RPC with isError):**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "title is required"
      }
    ],
    "isError": true
  }
}
```

**Key Points:**
- Error responses include `isError: true`
- Error message is in `content[0].text`
- HTTP status is still 200 (error semantics are in the JSON-RPC body)
- Tool calls still require standard MCP headers, including `Mcp-Session-Id`

---

## Session Management

```
┌─ Session starts
│  └─ POST /mcp with initialize method → HTTP 200 + Mcp-Session-Id header
│
├─ POST /mcp with notifications/initialized → HTTP 202 (no body)
│
└─ All subsequent tool calls
   └─ POST /mcp with -H "Mcp-Session-Id: {sid}" for each request
```

Each session has its own isolated todo store. Different session IDs = different todo lists.

---

## Reference: Full Handshake Example

```bash
# 1. Initialize and capture session ID
SESSION=$(curl -sD - -X POST http://localhost:8080/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.1"}}}' \
  | grep -i 'Mcp-Session-Id:' | awk '{print $2}' | tr -d '\r\n')

# 2. Send initialized notification
curl -s -o /dev/null -X POST http://localhost:8080/mcp \
  -H 'Content-Type: application/json' \
  -H "Mcp-Session-Id: $SESSION" \
  -d '{"jsonrpc":"2.0","method":"notifications/initialized"}'

# 3. List tools
curl -s -X POST http://localhost:8080/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -H "Mcp-Session-Id: $SESSION" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  | python3 -m json.tool

# 4. Call a tool
curl -s -X POST http://localhost:8080/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -H "Mcp-Session-Id: $SESSION" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"todo_create","arguments":{"title":"Test"}}}' \
  | python3 -m json.tool
```

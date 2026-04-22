#!/usr/bin/env bash
# test.sh — smoke-tests for todo-service HTTP and MCP endpoints
set -euo pipefail

BASE="${BASE_URL:-http://localhost:8080}"
PASS=0
FAIL=0

green() { printf '\033[0;32m✓ %s\033[0m\n' "$*"; }
red()   { printf '\033[0;31m✗ %s\033[0m\n' "$*"; }

# Print the exact curl command that is about to run
log_curl() {
  printf '\033[2m  curl' >&2
  for arg in "$@"; do
    # quote args that contain spaces or special chars so the line is copy-pasteable
    case "$arg" in
      *\ *|*\{*|*\}*|*\"*) printf " '%s'" "$arg" >&2 ;;
      *) printf ' %s' "$arg" >&2 ;;
    esac
  done
  printf '\033[0m\n' >&2
}

assert_eq() {
  local label="$1" expected="$2" actual="$3"
  if [ "$actual" = "$expected" ]; then
    green "$label"
    PASS=$((PASS + 1))
  else
    red "$label (expected: $expected, got: $actual)"
    FAIL=$((FAIL + 1))
  fi
}

assert_contains() {
  local label="$1" needle="$2" haystack="$3"
  if echo "$haystack" | grep -qF "$needle"; then
    green "$label"
    PASS=$((PASS + 1))
  else
    red "$label (expected to contain: $needle)"
    red "  actual: $haystack"
    FAIL=$((FAIL + 1))
  fi
}

http() {
  local method="$1" path="$2"; shift 2
  local args=(-s -w '\n%{http_code}' -X "$method" "$BASE$path"
              -H "Content-Type: application/json" "$@")
  log_curl "${args[@]}"
  curl "${args[@]}"
}

http_status() { echo "$1" | tail -1; }
http_body()   { echo "$1" | head -n -1; }

json_field() { echo "$1" | python3 -c "import sys,json; print(json.load(sys.stdin)$2)" 2>/dev/null; }

# ── MCP helpers ────────────────────────────────────────────────────────────────

mcp_session() {
  local init='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-script","version":"0.1"}}}'
  local args=(-sD - -X POST "$BASE/mcp"
              -H "Content-Type: application/json"
              -H "Accept: application/json, text/event-stream"
              -d "$init")
  log_curl "${args[@]}"
  curl "${args[@]}" | grep -i 'Mcp-Session-Id:' | awk '{print $2}' | tr -d '\r\n'
}

mcp_notify_initialized() {
  local sid="$1"
  local body='{"jsonrpc":"2.0","method":"notifications/initialized"}'
  local args=(-s -o /dev/null -X POST "$BASE/mcp"
              -H "Content-Type: application/json"
              -H "Mcp-Session-Id: $sid"
              -d "$body")
  log_curl "${args[@]}"
  curl "${args[@]}"
}

mcp_call() {
  local sid="$1" body="$2"
  local args=(-s -X POST "$BASE/mcp"
              -H "Content-Type: application/json"
              -H "Accept: application/json, text/event-stream"
              -H "Mcp-Session-Id: $sid"
              -d "$body")
  log_curl "${args[@]}"
  curl "${args[@]}"
}

mcp_tool_text() { json_field "$1" "['result']['content'][0]['text']"; }
mcp_is_error()  { json_field "$1" ".get('result',{}).get('isError', False)"; }

# ── cleanup: delete any todos left over from a previous run ───────────────────
existing=$(curl -s "$BASE/todos")
echo "$existing" | python3 -c "
import sys, json, urllib.request
items = json.load(sys.stdin)
for item in items:
    req = urllib.request.Request('$BASE/todos/' + str(item['id']), method='DELETE')
    urllib.request.urlopen(req)
" 2>/dev/null || true

# ══════════════════════════════════════════════════════════════════════════════
echo "── HTTP ──────────────────────────────────────────────────────────────────"
# ══════════════════════════════════════════════════════════════════════════════

# health
log_curl -s -w '\n%{http_code}' "$BASE/healthz"
resp=$(curl -s -w '\n%{http_code}' "$BASE/healthz")
assert_eq "GET /healthz → 200" "200" "$(http_status "$resp")"
assert_contains "GET /healthz body" "ok" "$(http_body "$resp")"

# initial empty list
resp=$(http GET /todos)
assert_eq "GET /todos status → 200" "200" "$(http_status "$resp")"
assert_eq "GET /todos body → []" "[]" "$(http_body "$resp" | tr -d ' \n')"

# create
resp=$(http POST /todos -d '{"title":"Buy groceries"}')
assert_eq "POST /todos status → 201" "201" "$(http_status "$resp")"
body=$(http_body "$resp")
assert_contains "POST /todos title" '"title":"Buy groceries"' "$body"
assert_contains "POST /todos completed=false" '"completed":false' "$body"
TODO_ID=$(json_field "$body" "['id']")

# list contains item
resp=$(http GET /todos)
assert_contains "GET /todos contains created item" "\"id\":$TODO_ID" "$(http_body "$resp")"

# update
resp=$(http PUT "/todos/$TODO_ID" -d '{"completed":true}')
assert_eq "PUT /todos/:id status → 200" "200" "$(http_status "$resp")"
assert_contains "PUT /todos/:id completed=true" '"completed":true' "$(http_body "$resp")"

# update title
resp=$(http PUT "/todos/$TODO_ID" -d '{"title":"Buy milk"}')
assert_contains "PUT /todos/:id title updated" '"title":"Buy milk"' "$(http_body "$resp")"

# delete
resp=$(http DELETE "/todos/$TODO_ID")
assert_eq "DELETE /todos/:id status → 204" "204" "$(http_status "$resp")"

# list empty again
resp=$(http GET /todos)
assert_eq "GET /todos empty after delete → []" "[]" "$(http_body "$resp" | tr -d ' \n')"

# not found errors
resp=$(http PUT "/todos/99999" -d '{"title":"ghost"}')
assert_eq "PUT non-existent → 404" "404" "$(http_status "$resp")"

resp=$(http DELETE "/todos/99999")
assert_eq "DELETE non-existent → 404" "404" "$(http_status "$resp")"

# bad request — empty title
resp=$(http POST /todos -d '{"title":"  "}')
assert_eq "POST empty title → 400" "400" "$(http_status "$resp")"

# bad request — missing title field
resp=$(http POST /todos -d '{}')
assert_eq "POST missing title → 400" "400" "$(http_status "$resp")"

# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "── MCP ───────────────────────────────────────────────────────────────────"
# ══════════════════════════════════════════════════════════════════════════════

SID=$(mcp_session)
assert_eq "MCP initialize → session ID present" "0" "$([ -n "$SID" ] && echo 0 || echo 1)"

mcp_notify_initialized "$SID"

# tools/list
resp=$(mcp_call "$SID" '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}')
assert_contains "MCP tools/list contains todo.create" "todo.create" "$resp"
assert_contains "MCP tools/list contains todo.list"   "todo.list"   "$resp"
assert_contains "MCP tools/list contains todo.update" "todo.update" "$resp"
assert_contains "MCP tools/list contains todo.delete" "todo.delete" "$resp"

# todo.create
resp=$(mcp_call "$SID" '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"todo.create","arguments":{"title":"MCP task"}}}')
text=$(mcp_tool_text "$resp")
assert_contains "MCP todo.create returns item" '"title":"MCP task"' "$text"
MCP_ID=$(json_field "$text" "['id']")

# todo.list
resp=$(mcp_call "$SID" '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"todo.list","arguments":{}}}')
assert_contains "MCP todo.list contains created item" "\"id\":$MCP_ID" "$(mcp_tool_text "$resp")"

# todo.update
resp=$(mcp_call "$SID" "{\"jsonrpc\":\"2.0\",\"id\":5,\"method\":\"tools/call\",\"params\":{\"name\":\"todo.update\",\"arguments\":{\"id\":$MCP_ID,\"completed\":true}}}")
assert_contains "MCP todo.update completed=true" '"completed":true' "$(mcp_tool_text "$resp")"

# todo.delete
resp=$(mcp_call "$SID" "{\"jsonrpc\":\"2.0\",\"id\":6,\"method\":\"tools/call\",\"params\":{\"name\":\"todo.delete\",\"arguments\":{\"id\":$MCP_ID}}}")
assert_contains "MCP todo.delete returns deleted:true" '"deleted":true' "$(mcp_tool_text "$resp")"

# todo.create — empty title should be an MCP error
resp=$(mcp_call "$SID" '{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"todo.create","arguments":{"title":"  "}}}')
assert_eq "MCP todo.create empty title → isError" "True" "$(mcp_is_error "$resp")"

# ══════════════════════════════════════════════════════════════════════════════
echo ""
echo "── Results ───────────────────────────────────────────────────────────────"
echo "  passed: $PASS"
echo "  failed: $FAIL"
[ "$FAIL" -eq 0 ] && echo "" && green "All tests passed" && exit 0
echo "" && red "$FAIL test(s) failed" && exit 1

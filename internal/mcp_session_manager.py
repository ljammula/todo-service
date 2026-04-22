#!/usr/bin/env python3
"""
MCP Session Manager for todo-service

Manages MCP session lifecycle and provides a clean interface for calling MCP tools.
Persists session ID to disk to avoid re-initialization on every call.
"""

import json
import os
import sys
import tempfile
from pathlib import Path
from typing import Any, Optional
import subprocess

# Session file location (in temp directory)
SESSION_FILE = Path(tempfile.gettempdir()) / ".todo_service_mcp_session"
BASE_URL = os.environ.get("TODO_SERVICE_URL", "http://localhost:8080")


def _curl(method: str, data: Optional[dict] = None, session_id: Optional[str] = None) -> dict:
    """Make a curl request to the MCP endpoint."""
    headers = [
        "-H", "Content-Type: application/json",
        "-H", "Accept: application/json, text/event-stream",
    ]
    
    if session_id:
        headers.extend(["-H", f"Mcp-Session-Id: {session_id}"])
    
    # Add headers first
    cmd = ["curl", "-s"]
    cmd.extend(headers)
    
    # Add -D - to capture headers for initialize
    if method == "initialize":
        cmd.append("-D")
        cmd.append("-")
    
    # Add -X POST
    cmd.extend(["-X", "POST"])
    
    # Add data if provided
    if data:
        cmd.extend(["-d", json.dumps(data)])
    
    # Add URL at the end
    cmd.append(f"{BASE_URL}/mcp")
    
    result = subprocess.run(cmd, capture_output=True, text=True)
    
    if result.returncode != 0:
        raise RuntimeError(f"curl failed: {result.stderr}")
    
    if method == "initialize":
        # Parse headers and body separately
        output = result.stdout
        lines = output.split('\n')
        session_id = None
        body_start = 0
        
        for i, line in enumerate(lines):
            if line.lower().startswith("mcp-session-id:"):
                session_id = line.split(":", 1)[1].strip()
            if line.strip() == "":
                body_start = i + 1
                break
        
        body_lines = lines[body_start:]
        body_text = '\n'.join(body_lines).strip()
        
        if not body_text:
            raise RuntimeError("No response body from initialize")
        
        response = json.loads(body_text)
        response["_session_id"] = session_id
        return response
    
    if method == "notify":
        # Notification returns 202 with no body, that's OK
        return {}
    
    # For other methods, parse JSON response
    if result.stdout.strip():
        return json.loads(result.stdout)
    return {}


def _load_session_id() -> Optional[str]:
    """Load saved session ID from disk."""
    if SESSION_FILE.exists():
        try:
            return SESSION_FILE.read_text().strip()
        except Exception:
            return None
    return None


def _save_session_id(session_id: str) -> None:
    """Save session ID to disk."""
    try:
        SESSION_FILE.write_text(session_id)
    except Exception:
        pass


def _initialize_session() -> str:
    """Initialize a new MCP session."""
    data = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "mcp-session-manager", "version": "0.1"}
        }
    }
    
    response = _curl("initialize", data)
    session_id = response.get("_session_id")
    
    if not session_id:
        raise RuntimeError("No session ID in initialize response")
    
    # Send initialized notification
    notify_data = {
        "jsonrpc": "2.0",
        "method": "notifications/initialized"
    }
    _curl("notify", notify_data, session_id)
    
    # Save for future use
    _save_session_id(session_id)
    return session_id


def get_session_id() -> str:
    """Get or create an MCP session ID."""
    # Try to load existing session
    session_id = _load_session_id()
    if session_id:
        return session_id
    
    # Initialize new session
    return _initialize_session()


def call_tool(tool_name: str, arguments: dict) -> Any:
    """Call an MCP tool and return the result."""
    session_id = get_session_id()
    
    data = {
        "jsonrpc": "2.0",
        "id": 2,
        "method": "tools/call",
        "params": {
            "name": tool_name,
            "arguments": arguments
        }
    }
    
    response = _curl("tools/call", data, session_id)
    
    # Extract result from content[0].text
    if "result" in response:
        result = response["result"]
        if "content" in result and len(result["content"]) > 0:
            content = result["content"][0]
            if "text" in content:
                text = content["text"]
                # Try to parse as JSON
                try:
                    return json.loads(text)
                except json.JSONDecodeError:
                    return text
    
    if "error" in response:
        raise RuntimeError(f"MCP error: {response['error']}")
    
    raise RuntimeError(f"Unexpected response format: {response}")


def list_todos() -> list:
    """List all todos."""
    return call_tool("todo_list", {})


def create_todo(title: str) -> dict:
    """Create a new todo."""
    return call_tool("todo_create", {"title": title})


def update_todo(todo_id: int, title: Optional[str] = None, completed: Optional[bool] = None) -> dict:
    """Update a todo."""
    arguments = {"id": todo_id}
    if title is not None:
        arguments["title"] = title
    if completed is not None:
        arguments["completed"] = completed
    return call_tool("todo_update", arguments)


def delete_todo(todo_id: int) -> dict:
    """Delete a todo."""
    return call_tool("todo_delete", {"id": todo_id})


if __name__ == "__main__":
    # Simple CLI for testing
    if len(sys.argv) < 2:
        print("Usage: mcp_session_manager.py <command> [args...]")
        print("Commands:")
        print("  list")
        print("  create <title>")
        print("  update <id> [title=<title>] [completed=<true|false>]")
        print("  delete <id>")
        sys.exit(1)
    
    command = sys.argv[1]
    
    try:
        if command == "list":
            result = list_todos()
            print(json.dumps(result, indent=2))
        elif command == "create":
            if len(sys.argv) < 3:
                print("Error: title required")
                sys.exit(1)
            result = create_todo(sys.argv[2])
            print(json.dumps(result, indent=2))
        elif command == "update":
            if len(sys.argv) < 3:
                print("Error: id required")
                sys.exit(1)
            todo_id = int(sys.argv[2])
            title = None
            completed = None
            for arg in sys.argv[3:]:
                if arg.startswith("title="):
                    title = arg[6:]
                elif arg.startswith("completed="):
                    completed = arg[10:].lower() == "true"
            result = update_todo(todo_id, title, completed)
            print(json.dumps(result, indent=2))
        elif command == "delete":
            if len(sys.argv) < 3:
                print("Error: id required")
                sys.exit(1)
            result = delete_todo(int(sys.argv[2]))
            print(json.dumps(result, indent=2))
        else:
            print(f"Unknown command: {command}")
            sys.exit(1)
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)

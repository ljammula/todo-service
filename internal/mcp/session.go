package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const sessionFileName = ".todo_service_mcp_session"

// Session manages MCP session state and provides a high-level API for tool calls.
type Session struct {
	id      string
	baseURL string
}

type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// NewSession creates a new or loads an existing MCP session.
func NewSession(baseURL string) (*Session, error) {
	// Try to load existing session from disk
	if sessionID, err := loadSessionID(); err == nil && sessionID != "" {
		return &Session{
			id:      sessionID,
			baseURL: baseURL,
		}, nil
	}

	// Create new session
	sessionID, err := initializeSession(baseURL)
	if err != nil {
		return nil, fmt.Errorf("initialize session: %w", err)
	}

	// Save session ID
	if err := saveSessionID(sessionID); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	return &Session{
		id:      sessionID,
		baseURL: baseURL,
	}, nil
}

// initializeSession creates a new MCP session and returns the session ID.
func initializeSession(baseURL string) (string, error) {
	// Send initialize request
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "todo-cli",
				"version": "0.1",
			},
		},
	}

	_, headers, err := doRequest(baseURL, req, "")
	if err != nil {
		return "", err
	}

	// Extract session ID from headers
	sessionID := headers.Get("Mcp-Session-Id")
	if sessionID == "" {
		return "", fmt.Errorf("no session ID in response headers")
	}

	// Send initialized notification
	notify := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}

	_, _, err = doRequest(baseURL, notify, sessionID)
	if err != nil {
		return "", fmt.Errorf("send initialized notification: %w", err)
	}

	return sessionID, nil
}

// CallTool calls an MCP tool and returns the result.
func (s *Session) CallTool(toolName string, arguments map[string]interface{}) (interface{}, error) {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      toolName,
			"arguments": arguments,
		},
	}

	resp, _, err := doRequest(s.baseURL, req, s.id)
	if err != nil {
		return nil, err
	}

	// Extract result from response
	if result, ok := resp["result"].(map[string]interface{}); ok {
		if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
			if contentItem, ok := content[0].(map[string]interface{}); ok {
				if text, ok := contentItem["text"].(string); ok {
					// Try to parse as JSON
					var parsed interface{}
					if err := json.Unmarshal([]byte(text), &parsed); err == nil {
						return parsed, nil
					}
					return text, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("unexpected response format: %v", resp)
}

// doRequest sends a JSON-RPC request to the MCP endpoint.
func doRequest(baseURL string, req jsonRPCRequest, sessionID string) (map[string]interface{}, http.Header, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", baseURL+"/mcp", bytes.NewReader(payload))
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", sessionID)
	}

	client := &http.Client{}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("send request: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read response: %w", err)
	}

	var resp map[string]interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, nil, fmt.Errorf("parse response: %w", err)
		}
	}

	return resp, httpResp.Header, nil
}

// sessionPath returns the path to the session file.
func sessionPath() (string, error) {
	tmpDir := os.TempDir()
	return filepath.Join(tmpDir, sessionFileName), nil
}

// loadSessionID loads the session ID from disk.
func loadSessionID() (string, error) {
	path, err := sessionPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil // File doesn't exist, return empty
	}

	return strings.TrimSpace(string(data)), nil
}

// saveSessionID saves the session ID to disk.
func saveSessionID(sessionID string) error {
	path, err := sessionPath()
	if err != nil {
		return err
	}

	return os.WriteFile(path, []byte(sessionID), 0644)
}

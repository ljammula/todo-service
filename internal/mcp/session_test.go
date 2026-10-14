package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"
)

func isolateSessionFile(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	// os.TempDir uses TMPDIR on Unix and TMP/TEMP on Windows.
	t.Setenv("TMPDIR", tmpDir)
	t.Setenv("TMP", tmpDir)
	t.Setenv("TEMP", tmpDir)

	path, err := sessionPath()
	if err != nil {
		t.Fatalf("sessionPath() error = %v", err)
	}
	return path
}

func supportsUnixPermissionBits() bool {
	switch runtime.GOOS {
	case "aix", "android", "darwin", "dragonfly", "freebsd", "ios", "linux", "netbsd", "openbsd", "solaris":
		return true
	default:
		return false
	}
}

func TestSaveSessionIDRoundTripAndPermissions(t *testing.T) {
	isolateSessionFile(t)
	const want = "session-id"

	if err := saveSessionID(want); err != nil {
		t.Fatalf("saveSessionID() error = %v", err)
	}
	got, err := loadSessionID()
	if err != nil {
		t.Fatalf("loadSessionID() error = %v", err)
	}
	if got != want {
		t.Fatalf("loadSessionID() = %q, want %q", got, want)
	}

	if !supportsUnixPermissionBits() {
		t.Skip("permission-bit assertions require Unix file permission semantics")
	}

	path, err := sessionPath()
	if err != nil {
		t.Fatalf("sessionPath() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat session file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Errorf("session file permissions = %04o, want 0600", got)
	}
}

func TestSaveSessionIDOverwritesPermissiveFile(t *testing.T) {
	path := isolateSessionFile(t)
	if err := os.WriteFile(path, []byte("old-session-id"), 0666); err != nil {
		t.Fatalf("create session file: %v", err)
	}
	if supportsUnixPermissionBits() {
		if err := os.Chmod(path, 0666); err != nil {
			t.Fatalf("make session file permissive: %v", err)
		}
	}

	const want = "replacement-session-id"
	if err := saveSessionID(want); err != nil {
		t.Fatalf("saveSessionID() error = %v", err)
	}
	got, err := loadSessionID()
	if err != nil {
		t.Fatalf("loadSessionID() error = %v", err)
	}
	if got != want {
		t.Fatalf("loadSessionID() = %q, want %q", got, want)
	}

	if !supportsUnixPermissionBits() {
		t.Skip("permission-bit assertions require Unix file permission semantics")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat session file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Errorf("session file permissions = %04o, want 0600", got)
	}
}

func TestNewSessionUsesPersistedIDWithoutInitialization(t *testing.T) {
	isolateSessionFile(t)
	const want = "persisted-session-id"
	if err := saveSessionID(want); err != nil {
		t.Fatalf("saveSessionID() error = %v", err)
	}

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests++
	}))
	defer server.Close()

	session, err := NewSession(server.URL)
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if session.id != want {
		t.Errorf("session.id = %q, want %q", session.id, want)
	}
	if requests != 0 {
		t.Errorf("server received %d requests, want 0", requests)
	}
}

func TestNewSessionInitializesWhenIDIsUnavailable(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, path string)
	}{
		{
			name:  "absent",
			setup: func(*testing.T, string) {},
		},
		{
			name: "empty",
			setup: func(t *testing.T, path string) {
				if err := os.WriteFile(path, []byte(" \n"), 0600); err != nil {
					t.Fatalf("create empty session file: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := isolateSessionFile(t)
			tt.setup(t, path)

			const want = "new-session-id"
			var methods []string
			var notificationSessionID string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request jsonRPCRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Errorf("decode request: %v", err)
					return
				}
				methods = append(methods, request.Method)

				switch request.Method {
				case "initialize":
					if got := r.Header.Get("Mcp-Session-Id"); got != "" {
						t.Errorf("initialize session header = %q, want empty", got)
					}
					w.Header().Set("Mcp-Session-Id", want)
					_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
				case "notifications/initialized":
					notificationSessionID = r.Header.Get("Mcp-Session-Id")
					w.WriteHeader(http.StatusAccepted)
				default:
					t.Errorf("unexpected request method %q", request.Method)
				}
			}))
			defer server.Close()

			session, err := NewSession(server.URL)
			if err != nil {
				t.Fatalf("NewSession() error = %v", err)
			}
			if session.id != want {
				t.Errorf("session.id = %q, want %q", session.id, want)
			}
			if got, wantMethods := methods, []string{"initialize", "notifications/initialized"}; len(got) != len(wantMethods) || got[0] != wantMethods[0] || got[1] != wantMethods[1] {
				t.Errorf("request methods = %v, want %v", got, wantMethods)
			}
			if notificationSessionID != want {
				t.Errorf("notification session header = %q, want %q", notificationSessionID, want)
			}
			if got, err := loadSessionID(); err != nil || got != want {
				t.Errorf("loadSessionID() = %q, %v; want %q, nil", got, err, want)
			}
		})
	}
}

func TestCallToolSendsSessionIDAndParsesResponse(t *testing.T) {
	isolateSessionFile(t)
	const sessionID = "loaded-session-id"
	if err := saveSessionID(sessionID); err != nil {
		t.Fatalf("saveSessionID() error = %v", err)
	}

	var gotSessionID string
	var gotRequest jsonRPCRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSessionID = r.Header.Get("Mcp-Session-Id")
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"{\"count\":2}"}]}}`))
	}))
	defer server.Close()

	session, err := NewSession(server.URL)
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	result, err := session.CallTool("todo_list", map[string]interface{}{"completed": false})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if gotSessionID != sessionID {
		t.Errorf("Mcp-Session-Id header = %q, want %q", gotSessionID, sessionID)
	}
	if gotRequest.Method != "tools/call" {
		t.Errorf("request method = %q, want %q", gotRequest.Method, "tools/call")
	}
	gotResult, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("CallTool() result type = %T, want map[string]interface{}", result)
	}
	if gotResult["count"] != float64(2) {
		t.Errorf("CallTool() result = %v, want count 2", gotResult)
	}
}

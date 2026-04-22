package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"todo-service/internal/todo"
)

type app struct {
	service *todo.Service
}

type errorResponse struct {
	Error string `json:"error"`
}

func main() {
	store := todo.NewStore()
	service := todo.NewService(store)
	app := &app{service: service}

	mux := http.NewServeMux()
	mux.HandleFunc("/todos", app.handleTodos)
	mux.HandleFunc("/todos/", app.handleTodoByID)
	mux.Handle("/mcp", app.newMCPHandler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	port := envOrDefault("PORT", "8080")
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	log.Printf("todo-service listening on http://localhost:%s", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func (a *app) handleTodos(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, a.service.ListItems())
	case http.MethodPost:
		var input todo.CreateInput
		if err := decodeJSON(r, &input); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		item, err := a.service.Create(input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *app) handleTodoByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.URL.Path, "/todos/")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	switch r.Method {
	case http.MethodPut:
		var input todo.UpdateInput
		if err := decodeJSON(r, &input); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		item, err := a.service.Update(id, input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		if err := a.service.Delete(id); err != nil {
			writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *app) newMCPHandler() http.Handler {
	server := mcp.NewServer(&mcp.Implementation{Name: "todo-service", Version: "0.1.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "todo_create",
		Description: "Create a todo item",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args struct {
		Title string `json:"title"`
	}) (*mcp.CallToolResult, todo.Item, error) {
		item, err := a.service.Create(todo.CreateInput{Title: args.Title})
		if err != nil {
			return nil, todo.Item{}, err
		}
		return nil, item, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "todo_list",
		Description: "List all todo items",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, todo.ListResult, error) {
		return nil, a.service.List(), nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "todo_update",
		Description: "Update a todo item",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args struct {
		ID        int64   `json:"id"`
		Title     *string `json:"title,omitempty"`
		Completed *bool   `json:"completed,omitempty"`
	}) (*mcp.CallToolResult, todo.Item, error) {
		item, err := a.service.Update(args.ID, todo.UpdateInput{Title: args.Title, Completed: args.Completed})
		if err != nil {
			return nil, todo.Item{}, err
		}
		return nil, item, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "todo_delete",
		Description: "Delete a todo item",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args struct {
		ID int64 `json:"id"`
	}) (*mcp.CallToolResult, map[string]any, error) {
		if err := a.service.Delete(args.ID); err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"deleted": true, "id": args.ID}, nil
	})

	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{
		JSONResponse: true,
	})
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}

func parseID(path, prefix string) (int64, error) {
	idText := strings.TrimPrefix(path, prefix)
	if idText == "" || strings.Contains(idText, "/") {
		return 0, fmt.Errorf("invalid todo id")
	}
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid todo id")
	}
	return id, nil
}

func writeServiceError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, todo.ErrNotFound) {
		status = http.StatusNotFound
	}
	writeJSON(w, status, errorResponse{Error: err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response error: %v", err)
	}
}

type responseRecorder struct {
	http.ResponseWriter
	status       int
	bytesWritten int
}

func (rr *responseRecorder) WriteHeader(status int) {
	rr.status = status
	rr.ResponseWriter.WriteHeader(status)
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	n, err := rr.ResponseWriter.Write(b)
	rr.bytesWritten += n
	return n, err
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		var reqBody []byte
		if r.Body != nil && r.ContentLength > 0 {
			reqBody, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(reqBody))
		}

		rr := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rr, r)

		log.Printf("%s %s %d %dB in=%dB %s",
			r.Method,
			r.URL.RequestURI(),
			rr.status,
			rr.bytesWritten,
			len(reqBody),
			time.Since(start),
		)
	})
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

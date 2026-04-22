package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"todo-service/internal/mcp"
)

func main() {
	// Get subcommand
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Get service URL from env or use default
	baseURL := os.Getenv("TODO_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Create or load session
	session, err := mcp.NewSession(baseURL)
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "list":
		cmdList(session)
	case "create":
		cmdCreate(session, args)
	case "update":
		cmdUpdate(session, args)
	case "delete":
		cmdDelete(session, args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func cmdList(session *mcp.Session) {
	result, err := session.CallTool("todo_list", map[string]interface{}{})
	if err != nil {
		log.Fatalf("Failed to list todos: %v", err)
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal result: %v", err)
	}

	fmt.Println(string(output))
}

func cmdCreate(session *mcp.Session, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: todo-cli create <title>\n")
		os.Exit(1)
	}

	title := strings.Join(args, " ")

	result, err := session.CallTool("todo_create", map[string]interface{}{
		"title": title,
	})
	if err != nil {
		log.Fatalf("Failed to create todo: %v", err)
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal result: %v", err)
	}

	fmt.Println(string(output))
}

func cmdUpdate(session *mcp.Session, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: todo-cli update <id> [title=<title>] [completed=<true|false>]\n")
		os.Exit(1)
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		log.Fatalf("Invalid todo ID: %v", err)
	}

	arguments := map[string]interface{}{
		"id": id,
	}

	// Parse additional arguments
	for _, arg := range args[1:] {
		parts := strings.Split(arg, "=")
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "Invalid argument format: %s (use key=value)\n", arg)
			os.Exit(1)
		}

		key := parts[0]
		value := parts[1]

		switch key {
		case "title":
			arguments["title"] = value
		case "completed":
			completed := strings.ToLower(value) == "true"
			arguments["completed"] = completed
		default:
			fmt.Fprintf(os.Stderr, "Unknown argument: %s\n", key)
			os.Exit(1)
		}
	}

	result, err := session.CallTool("todo_update", arguments)
	if err != nil {
		log.Fatalf("Failed to update todo: %v", err)
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal result: %v", err)
	}

	fmt.Println(string(output))
}

func cmdDelete(session *mcp.Session, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: todo-cli delete <id>\n")
		os.Exit(1)
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		log.Fatalf("Invalid todo ID: %v", err)
	}

	result, err := session.CallTool("todo_delete", map[string]interface{}{
		"id": id,
	})
	if err != nil {
		log.Fatalf("Failed to delete todo: %v", err)
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal result: %v", err)
	}

	fmt.Println(string(output))
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `todo-cli - MCP client for todo-service

Usage:
  todo-cli <command> [arguments]

Commands:
  list                              List all todos
  create <title>                    Create a new todo
  update <id> [title=...] [...]     Update a todo
  delete <id>                       Delete a todo
  help                              Show this help message

Examples:
  todo-cli list
  todo-cli create "Buy groceries"
  todo-cli update 1 title="Buy oat milk" completed=true
  todo-cli delete 1

Environment variables:
  TODO_SERVICE_URL  Base URL of todo-service (default: http://localhost:8080)
`)
}

func init() {
	flag.Usage = func() {
		printUsage()
	}
}

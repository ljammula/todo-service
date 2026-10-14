# Agent instructions

This repository is a Go 1.25 todo service with its business logic in `internal/todo`.

## Verification

- Run `go test ./...` after changing Go code.
- Keep HTTP and MCP behavior consistent with the examples in `README.md` and `MCP_RESPONSE_FORMAT.md`.
- Do not require a live server for unit tests; `test.sh` is an integration smoke test for a running service.

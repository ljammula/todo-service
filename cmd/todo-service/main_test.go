package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"todo-service/internal/todo"
)

func newTestApp() *app {
	return &app{service: todo.NewService(todo.NewStore())}
}

func performRequest(t *testing.T, handler http.HandlerFunc, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	recorder := httptest.NewRecorder()
	handler(recorder, req)
	return recorder
}

func requireInvalidJSONResponse(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}

	var response errorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v; body = %s", err, recorder.Body.String())
	}
	if !strings.HasPrefix(response.Error, "invalid JSON body: ") {
		t.Fatalf("error = %q, want prefix %q", response.Error, "invalid JSON body: ")
	}
}

func requireEmptyTodoList(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if got := recorder.Body.String(); got != "[]\n" {
		t.Fatalf("body = %q, want %q", got, "[]\n")
	}
}

func TestHandleTodosRejectsConcatenatedJSON(t *testing.T) {
	app := newTestApp()

	response := performRequest(t, app.handleTodos, http.MethodPost, "/todos", `{"title":"a"}{"title":"b"}`)
	requireInvalidJSONResponse(t, response)

	list := performRequest(t, app.handleTodos, http.MethodGet, "/todos", "")
	requireEmptyTodoList(t, list)
}

func TestHandleTodosRejectsTrailingGarbage(t *testing.T) {
	app := newTestApp()

	response := performRequest(t, app.handleTodos, http.MethodPost, "/todos", `{"title":"a"} garbage`)
	requireInvalidJSONResponse(t, response)

	list := performRequest(t, app.handleTodos, http.MethodGet, "/todos", "")
	requireEmptyTodoList(t, list)
}

func TestHandleTodoByIDRejectsTrailingDataBeforeUpdate(t *testing.T) {
	app := newTestApp()
	if _, err := app.service.Create(todo.CreateInput{Title: "a"}); err != nil {
		t.Fatalf("seed todo: %v", err)
	}

	response := performRequest(t, app.handleTodoByID, http.MethodPut, "/todos/1", `{"completed":true}x`)
	requireInvalidJSONResponse(t, response)

	list := performRequest(t, app.handleTodos, http.MethodGet, "/todos", "")
	if list.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", list.Code, http.StatusOK, list.Body.String())
	}
	var items []todo.Item
	if err := json.Unmarshal(list.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode todo list: %v; body = %s", err, list.Body.String())
	}
	want := []todo.Item{{ID: 1, Title: "a", Completed: false}}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("items = %#v, want %#v", items, want)
	}
}

func TestHandleTodosAcceptsTrailingJSONWhitespace(t *testing.T) {
	app := newTestApp()

	response := performRequest(t, app.handleTodos, http.MethodPost, "/todos", "{\"title\":\"a\"}\n")
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if got := response.Body.String(); got != "{\"id\":1,\"title\":\"a\",\"completed\":false}\n" {
		t.Fatalf("body = %q, want %q", got, "{\"id\":1,\"title\":\"a\",\"completed\":false}\n")
	}
}

func TestHandleTodosRejectsUnknownFieldAndEmptyTitle(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantError string
	}{
		{
			name:      "unknown field",
			body:      `{"title":"a","unexpected":1}`,
			wantError: `invalid JSON body: json: unknown field "unexpected"`,
		},
		{
			name:      "empty title",
			body:      `{"title":""}`,
			wantError: "title is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newTestApp()
			response := performRequest(t, app.handleTodos, http.MethodPost, "/todos", test.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusBadRequest, response.Body.String())
			}

			var errorBody errorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &errorBody); err != nil {
				t.Fatalf("decode error response: %v; body = %s", err, response.Body.String())
			}
			if errorBody.Error != test.wantError {
				t.Fatalf("error = %q, want %q", errorBody.Error, test.wantError)
			}

			list := performRequest(t, app.handleTodos, http.MethodGet, "/todos", "")
			requireEmptyTodoList(t, list)
		})
	}
}

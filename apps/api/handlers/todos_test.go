package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"example.com/hello-go/middleware"
	"example.com/hello-go/store"
)

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://localhost:5432/hellodb_test"
	}
	db, err := store.Open(url)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec("TRUNCATE todos, accounts, sessions"); err != nil {
		t.Fatal(err)
	}
	return db
}

func setupTodos(t *testing.T) *Todos {
	return &Todos{DB: setupDB(t)}
}

// asUser builds a request carrying the username the Auth middleware would set.
func asUser(user, method, target string, body string) *http.Request {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	return req.WithContext(middleware.WithUser(req.Context(), user))
}

func TestCreateTodo(t *testing.T) {
	h := setupTodos(t)

	rec := httptest.NewRecorder()
	h.Create(rec, asUser("ann", "POST", "/todos", `{"title":"buy milk"}`))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var todo Todo
	if err := json.NewDecoder(rec.Body).Decode(&todo); err != nil {
		t.Fatal(err)
	}
	if todo.ID == 0 || todo.Title != "buy milk" {
		t.Errorf("expected id and title back, got %+v", todo)
	}
}

func TestCreateTodo_EmptyTitle(t *testing.T) {
	h := setupTodos(t)

	for _, body := range []string{`{"title":""}`, `{"title":"   "}`} {
		rec := httptest.NewRecorder()
		h.Create(rec, asUser("ann", "POST", "/todos", body))

		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %s: expected 400, got %d", body, rec.Code)
		}
	}
}

func TestCreateTodo_InvalidJSON(t *testing.T) {
	h := setupTodos(t)

	rec := httptest.NewRecorder()
	h.Create(rec, asUser("ann", "POST", "/todos", `not json`))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCreateTodo_Duplicate(t *testing.T) {
	h := setupTodos(t)

	for i, want := range []int{http.StatusCreated, http.StatusConflict} {
		rec := httptest.NewRecorder()
		h.Create(rec, asUser("ann", "POST", "/todos", `{"title":"buy milk"}`))

		if rec.Code != want {
			t.Errorf("request %d: expected %d, got %d", i+1, want, rec.Code)
		}
	}
}

func TestCreateTodo_SameTitleDifferentUsers(t *testing.T) {
	h := setupTodos(t)

	for _, user := range []string{"ann", "bob"} {
		rec := httptest.NewRecorder()
		h.Create(rec, asUser(user, "POST", "/todos", `{"title":"buy milk"}`))

		if rec.Code != http.StatusCreated {
			t.Errorf("user %s: expected 201, got %d", user, rec.Code)
		}
	}
}

func TestListTodos_Empty(t *testing.T) {
	h := setupTodos(t)

	rec := httptest.NewRecorder()
	h.List(rec, asUser("ann", "GET", "/todos", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Errorf("expected [], got %s", got)
	}
}

func TestListTodos_ScopedToUser(t *testing.T) {
	h := setupTodos(t)
	for user, title := range map[string]string{"ann": "ann's task", "bob": "bob's task"} {
		if _, err := h.DB.Exec(
			"INSERT INTO todos (username, title) VALUES ($1, $2)", user, title); err != nil {
			t.Fatal(err)
		}
	}

	rec := httptest.NewRecorder()
	h.List(rec, asUser("ann", "GET", "/todos", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var todos []Todo
	if err := json.NewDecoder(rec.Body).Decode(&todos); err != nil {
		t.Fatal(err)
	}
	if len(todos) != 1 || todos[0].Title != "ann's task" {
		t.Errorf("expected only ann's task, got %+v", todos)
	}
}

func TestDeleteTodo(t *testing.T) {
	h := setupTodos(t)
	var id int64
	if err := h.DB.QueryRow(
		"INSERT INTO todos (username, title) VALUES ('ann', 'buy milk') RETURNING id",
	).Scan(&id); err != nil {
		t.Fatal(err)
	}

	req := asUser("ann", "DELETE", "/todos/1", "")
	req.SetPathValue("id", strconv.FormatInt(id, 10))
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	var count int
	if err := h.DB.QueryRow("SELECT count(*) FROM todos").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected todo deleted, %d rows remain", count)
	}
}

func TestDeleteTodo_OtherUsers(t *testing.T) {
	h := setupTodos(t)
	var id int64
	if err := h.DB.QueryRow(
		"INSERT INTO todos (username, title) VALUES ('ann', 'buy milk') RETURNING id",
	).Scan(&id); err != nil {
		t.Fatal(err)
	}

	req := asUser("bob", "DELETE", "/todos/1", "")
	req.SetPathValue("id", strconv.FormatInt(id, 10))
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	var count int
	if err := h.DB.QueryRow("SELECT count(*) FROM todos").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected ann's todo to survive, %d rows remain", count)
	}
}

func TestDeleteTodo_BadID(t *testing.T) {
	h := setupTodos(t)

	req := asUser("ann", "DELETE", "/todos/abc", "")
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

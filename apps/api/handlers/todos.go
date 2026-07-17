package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"example.com/hello-go/middleware"
	"github.com/jackc/pgx/v5/pgconn"
)

type Todo struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

type Todos struct {
	DB *sql.DB
}

func (h *Todos) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.Username(r)
	todos := []Todo{} // non-nil so an empty list encodes as [], not null
	rows, err := h.DB.Query(
		"SELECT id, title FROM todos WHERE username = $1 ORDER BY id", user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.Title); err != nil {
			writeError(w, http.StatusInternalServerError, "scan failed")
			return
		}
		todos = append(todos, t)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(todos)
}

func (h *Todos) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.Username(r)
	var t Todo
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	t.Title = strings.TrimSpace(t.Title)
	if t.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if len(t.Title) > 200 {
		writeError(w, http.StatusBadRequest, "title must be 200 characters or fewer")
		return
	}
	err := h.DB.QueryRow(
		"INSERT INTO todos (username, title) VALUES ($1, $2) RETURNING id",
		user, t.Title).Scan(&t.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			writeError(w, http.StatusConflict, "todo already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create todo")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(t)
}

func (h *Todos) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.Username(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "todo not found")
		return
	}
	// scoping the DELETE by username means you can never delete someone else's todo
	res, err := h.DB.Exec(
		"DELETE FROM todos WHERE id = $1 AND username = $2", id, user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete todo")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeError(w, http.StatusNotFound, "todo not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgconn"
)

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Users struct {
	DB *sql.DB
}

func (h *Users) Get(w http.ResponseWriter, r *http.Request) {
	var u User
	err := h.DB.QueryRow("SELECT id, name FROM users WHERE id = $1", r.PathValue("id")).
		Scan(&u.ID, &u.Name)
	if err == sql.ErrNoRows {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(u)
}

func (h *Users) Create(w http.ResponseWriter, r *http.Request) {
	var u User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if msg := u.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	_, err := h.DB.Exec("INSERT INTO users (id, name) VALUES ($1, $2)", u.ID, u.Name)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			writeError(w, http.StatusConflict, "user already exists") // 409!
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create user")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(u)
}

func (h *Users) List(w http.ResponseWriter, r *http.Request) {
	users := []User{} // non-nil so an empty list encodes as [], not null
	rows, err := h.DB.Query("SELECT id, name FROM users")
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name); err != nil {
			http.Error(w, "scan failed", http.StatusInternalServerError)
			return
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func (h *Users) Update(w http.ResponseWriter, r *http.Request) {
	var u User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if _, err := h.DB.Exec("UPDATE users SET name = $1 WHERE id = $2", u.Name, r.PathValue("id")); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	u.ID = r.PathValue("id")
	json.NewEncoder(w).Encode(u)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (u *User) validate() string {
	if strings.TrimSpace(u.ID) == "" {
		return "id is required"
	}
	if strings.TrimSpace(u.Name) == "" {
		return "name is required"
	}
	if len(u.Name) > 100 {
		return "name must be 100 characters or fewer"
	}
	return ""
}

func (h *Users) Stats(w http.ResponseWriter, r *http.Request) {
	var users, accounts, sessions int
	var wg sync.WaitGroup

	count := func(table string, dest *int) {
		defer wg.Done()
		h.DB.QueryRow("SELECT count(*) FROM " + table).Scan(dest)
	}

	wg.Add(3)
	go count("users", &users)
	go count("accounts", &accounts)
	go count("sessions", &sessions)
	wg.Wait()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"users": users, "accounts": accounts, "sessions": sessions,
	})
}

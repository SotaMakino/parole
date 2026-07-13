package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
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
	err := h.DB.QueryRow("SELECT id, name FROM users WHERE id = ?", r.PathValue("id")).
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
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if _, err := h.DB.Exec("INSERT INTO users (id, name) VALUES (?, ?)", u.ID, u.Name); err != nil {
		http.Error(w, "could not create user", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(u)
}

func (h *Users) List(w http.ResponseWriter, r *http.Request) {
	var users []User
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
	if _, err := h.DB.Exec("UPDATE users SET name = ? WHERE id = ?", u.Name, r.PathValue("id")); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	u.ID = r.PathValue("id")
	json.NewEncoder(w).Encode(u)
}

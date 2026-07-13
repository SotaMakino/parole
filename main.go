package main

import (
	"encoding/json"
	"net/http"
	"sync"
)

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var (
	users = map[string]User{}
	mu    sync.Mutex
)

func getUser(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	u, ok := users[r.PathValue("id")]
	mu.Unlock()
	if !ok {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(u)
}

func createUser(w http.ResponseWriter, r *http.Request) {
	var u User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	mu.Lock()
	users[u.ID] = u
	mu.Unlock()
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(u)
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	delete(users, r.PathValue("id"))
	mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/{id}", getUser)
	mux.HandleFunc("POST /users", createUser)
	mux.HandleFunc("DELETE /users/{id}", deleteUser)
	http.ListenAndServe(":8080", mux)
}

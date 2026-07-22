package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"example.com/le-cinque/middleware"
	"golang.org/x/crypto/bcrypt"
)

type Auth struct {
	DB *sql.DB
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *Auth) Signup(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil ||
		c.Username == "" || len(c.Password) < 8 {
		writeError(w, http.StatusBadRequest, "username and password (min 8 chars) required")
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
	_, err := a.DB.Exec("INSERT INTO accounts (username, password_hash) VALUES ($1, $2)",
		c.Username, string(hash))
	if err != nil {
		writeError(w, http.StatusConflict, "username already taken")
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (a *Auth) Login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	json.NewDecoder(r.Body).Decode(&c)

	var hash string
	err := a.DB.QueryRow("SELECT password_hash FROM accounts WHERE username = $1",
		c.Username).Scan(&hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(c.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials") // same msg for both cases!
		return
	}

	// random 32-byte session token
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)

	a.DB.Exec("INSERT INTO sessions (token, username, expires_at) VALUES ($1, $2, $3)",
		token, c.Username, time.Now().Add(24*time.Hour))

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true, // JS cannot read it — XSS protection
		Secure:   true,
		SameSite: http.SameSiteLaxMode, // first-party: Pages proxies /api to this backend
		MaxAge:   86400,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		a.DB.Exec("DELETE FROM sessions WHERE token = $1", cookie.Value) // kill server side
	}
	http.SetCookie(w, &http.Cookie{
		Name: "session", Value: "", Path: "/",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: -1, // tells the browser: delete this cookie
	})
	w.WriteHeader(http.StatusNoContent)
}

// DeleteAccount permanently removes the signed-in account and everything tied to
// it — game history, sessions, and the account row itself — honouring the "email
// us to delete your data" promise in the Privacy Policy as a self-service button.
// The route requires a valid session (see middleware.Auth), so the user to delete
// is whoever the cookie identifies.
func (a *Auth) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	user := middleware.Username(r)

	// one transaction so a half-deleted account can never be left behind. Guesses
	// clear on their own: the games -> guesses foreign key is ON DELETE CASCADE.
	tx, err := a.DB.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete account")
		return
	}
	defer tx.Rollback()
	for _, q := range []string{
		"DELETE FROM games WHERE username = $1",
		"DELETE FROM sessions WHERE username = $1",
		"DELETE FROM accounts WHERE username = $1",
	} {
		if _, err := tx.Exec(q, user); err != nil {
			writeError(w, http.StatusInternalServerError, "could not delete account")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete account")
		return
	}

	// clear the now-orphaned session cookie, same attributes as Logout
	http.SetCookie(w, &http.Cookie{
		Name: "session", Value: "", Path: "/",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

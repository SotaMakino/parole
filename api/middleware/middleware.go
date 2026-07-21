package middleware

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

type contextKey struct{}
type authKey struct{}

// WithUser returns a context carrying the current player's identity (a signed-in
// username or an anonymous guest id).
func WithUser(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, contextKey{}, username)
}

// Username returns the player identity set by Auth/Player, or "" if absent.
func Username(r *http.Request) string {
	u, _ := r.Context().Value(contextKey{}).(string)
	return u
}

// WithAuth records whether the current player is a signed-in account (true) or
// an anonymous guest (false).
func WithAuth(ctx context.Context, authenticated bool) context.Context {
	return context.WithValue(ctx, authKey{}, authenticated)
}

// Authenticated reports whether the request belongs to a signed-in account.
func Authenticated(r *http.Request) bool {
	a, _ := r.Context().Value(authKey{}).(bool)
	return a
}

// sessionUser resolves a valid "session" cookie to its username. ok is false
// when there is no cookie, no matching session, or the session has expired.
func sessionUser(db *sql.DB, r *http.Request) (username string, ok bool) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return "", false
	}
	var expires time.Time
	err = db.QueryRow(
		"SELECT username, expires_at FROM sessions WHERE token = $1",
		cookie.Value).Scan(&username, &expires)
	if err != nil || time.Now().After(expires) {
		return "", false
	}
	return username, true
}

func Auth(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, ok := sessionUser(db, r)
		if !ok {
			unauthorized(w)
			return
		}
		ctx := WithAuth(WithUser(r.Context(), username), true)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Player identifies the current player for the always-open game routes: a
// signed-in account when a valid session cookie is present, otherwise an
// anonymous guest (see Guest). Signing in switches a browser from its guest
// history to the account's own history and unlocks the vocabulary count.
func Player(db *sql.DB, next http.Handler) http.Handler {
	guest := Guest(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if username, ok := sessionUser(db, r); ok {
			ctx := WithAuth(WithUser(r.Context(), username), true)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		guest.ServeHTTP(w, r)
	})
}

// Guest lets anyone play without signing in. It identifies a player by an
// anonymous "player" cookie, minting one on first visit, so each browser keeps
// its own game history (the spaced-repetition logic still works per browser).
func Guest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id string
		if cookie, err := r.Cookie("player"); err == nil && cookie.Value != "" {
			id = cookie.Value
		} else {
			b := make([]byte, 16)
			rand.Read(b)
			id = "guest_" + hex.EncodeToString(b)
			http.SetCookie(w, &http.Cookie{
				Name:     "player",
				Value:    id,
				Path:     "/",
				HttpOnly: true,
				Secure:   true,
				SameSite: http.SameSiteLaxMode, // first-party: Pages proxies /api to this backend
				MaxAge:   60 * 60 * 24 * 365,   // one year
			})
		}
		ctx := WithAuth(WithUser(r.Context(), id), false)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": "authentication required"})
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r) // call the actual handler
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start))
	})
}

// CORS reflects the request's Origin when it is in the allowed list. Because
// the app sends credentials, "*" is not permitted — a specific origin must be
// echoed back, so multiple front-ends are supported via a comma-separated list.
func CORS(allowedOrigins []string, next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range allowedOrigins {
		if o = strings.TrimSpace(o); o != "" {
			allowed[o] = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions { // preflight request
			w.WriteHeader(http.StatusNoContent)
			return // don't call the handler
		}
		next.ServeHTTP(w, r)
	})
}

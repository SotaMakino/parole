package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type contextKey struct{}

// WithUser returns a context carrying the authenticated username.
func WithUser(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, contextKey{}, username)
}

// Username returns the authenticated username set by Auth, or "" if absent.
func Username(r *http.Request) string {
	u, _ := r.Context().Value(contextKey{}).(string)
	return u
}

func Auth(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil {
			unauthorized(w)
			return
		}

		var username string
		var expires time.Time
		err = db.QueryRow(
			"SELECT username, expires_at FROM sessions WHERE token = $1",
			cookie.Value).Scan(&username, &expires)
		if err != nil || time.Now().After(expires) {
			unauthorized(w)
			return
		}

		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), username)))
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

func CORS(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
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

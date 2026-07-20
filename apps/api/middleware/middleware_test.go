package middleware

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"example.com/parole/store"
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
	if _, err := db.Exec("TRUNCATE games, guesses, accounts, sessions"); err != nil {
		t.Fatal(err)
	}
	return db
}

func authRequest(db *sql.DB, cookie *http.Cookie) *httptest.ResponseRecorder {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest("GET", "/game", nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	Auth(db, next).ServeHTTP(rec, req)
	return rec
}

func TestAuth_NoCookie(t *testing.T) {
	db := setupDB(t)

	rec := authRequest(db, nil)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	db := setupDB(t)

	rec := authRequest(db, &http.Cookie{Name: "session", Value: "no-such-token"})

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_ExpiredSession(t *testing.T) {
	db := setupDB(t)
	if _, err := db.Exec("INSERT INTO sessions (token, username, expires_at) VALUES ($1, $2, $3)",
		"expired-token", "ann", time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	rec := authRequest(db, &http.Cookie{Name: "session", Value: "expired-token"})

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_ValidSession(t *testing.T) {
	db := setupDB(t)
	if _, err := db.Exec("INSERT INTO sessions (token, username, expires_at) VALUES ($1, $2, $3)",
		"valid-token", "ann", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	rec := authRequest(db, &http.Cookie{Name: "session", Value: "valid-token"})

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/le-cinque/middleware"
)

func setupAuth(t *testing.T) *Auth {
	return &Auth{DB: setupDB(t)}
}

func signup(t *testing.T, a *Auth, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/signup", strings.NewReader(body))
	rec := httptest.NewRecorder()
	a.Signup(rec, req)
	return rec
}

func login(t *testing.T, a *Auth, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
	rec := httptest.NewRecorder()
	a.Login(rec, req)
	return rec
}

func TestSignup(t *testing.T) {
	a := setupAuth(t)

	rec := signup(t, a, `{"username":"ann","password":"secret123"}`)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
}

func TestSignup_ShortPassword(t *testing.T) {
	a := setupAuth(t)

	rec := signup(t, a, `{"username":"ann","password":"short"}`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestSignup_EmptyUsername(t *testing.T) {
	a := setupAuth(t)

	rec := signup(t, a, `{"username":"","password":"secret123"}`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestSignup_DuplicateUsername(t *testing.T) {
	a := setupAuth(t)

	signup(t, a, `{"username":"ann","password":"secret123"}`)
	rec := signup(t, a, `{"username":"ann","password":"secret456"}`)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestLogin(t *testing.T) {
	a := setupAuth(t)
	signup(t, a, `{"username":"ann","password":"secret123"}`)

	rec := login(t, a, `{"username":"ann","password":"secret123"}`)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "session" || cookies[0].Value == "" {
		t.Errorf("expected a session cookie, got %v", cookies)
	}

	var count int
	if err := a.DB.QueryRow("SELECT COUNT(*) FROM sessions WHERE username = $1", "ann").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 session in DB, got %d", count)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	a := setupAuth(t)
	signup(t, a, `{"username":"ann","password":"secret123"}`)

	rec := login(t, a, `{"username":"ann","password":"wrongpass"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestLogin_UnknownUser(t *testing.T) {
	a := setupAuth(t)

	rec := login(t, a, `{"username":"nobody","password":"secret123"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestLogout(t *testing.T) {
	a := setupAuth(t)
	signup(t, a, `{"username":"ann","password":"secret123"}`)
	loginRec := login(t, a, `{"username":"ann","password":"secret123"}`)
	token := loginRec.Result().Cookies()[0].Value

	req := httptest.NewRequest("POST", "/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rec := httptest.NewRecorder()

	a.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge != -1 {
		t.Errorf("expected cookie deletion (MaxAge -1), got %v", cookies)
	}

	var count int
	if err := a.DB.QueryRow("SELECT COUNT(*) FROM sessions WHERE token = $1", token).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected session deleted from DB, found %d", count)
	}
}

func TestDeleteAccount(t *testing.T) {
	a := setupAuth(t)
	signup(t, a, `{"username":"ann","password":"secret123"}`)
	login(t, a, `{"username":"ann","password":"secret123"}`)

	// give ann a round with a guess, so cascade deletion is exercised too
	var gameID int64
	if err := a.DB.QueryRow(
		"INSERT INTO games (username, word, status) VALUES ('ann','TRENO','won') RETURNING id",
	).Scan(&gameID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec("INSERT INTO guesses (game_id, guess) VALUES ($1, 'T:0:0')", gameID); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("DELETE", "/me", nil)
	req = req.WithContext(middleware.WithUser(req.Context(), "ann"))
	rec := httptest.NewRecorder()

	a.DeleteAccount(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "session" || cookies[0].MaxAge != -1 {
		t.Errorf("expected session cookie deletion (MaxAge -1), got %v", cookies)
	}

	// account, sessions, and games are all gone
	for _, q := range []string{
		"SELECT COUNT(*) FROM accounts WHERE username = 'ann'",
		"SELECT COUNT(*) FROM sessions WHERE username = 'ann'",
		"SELECT COUNT(*) FROM games WHERE username = 'ann'",
	} {
		var n int
		if err := a.DB.QueryRow(q).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("expected 0 rows for %q, got %d", q, n)
		}
	}
	// guesses go with their game via ON DELETE CASCADE
	var guesses int
	if err := a.DB.QueryRow("SELECT COUNT(*) FROM guesses WHERE game_id = $1", gameID).Scan(&guesses); err != nil {
		t.Fatal(err)
	}
	if guesses != 0 {
		t.Errorf("expected guesses cascade-deleted, got %d", guesses)
	}
}

func TestLogout_NoCookie(t *testing.T) {
	a := setupAuth(t)

	req := httptest.NewRequest("POST", "/logout", nil)
	rec := httptest.NewRecorder()

	a.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
}

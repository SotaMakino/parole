package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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
	if _, err := db.Exec("TRUNCATE games, guesses, accounts, sessions"); err != nil {
		t.Fatal(err)
	}
	return db
}

func setupGames(t *testing.T) *Games {
	return &Games{DB: setupDB(t)}
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

// startGame inserts a game with a known answer so tests can guess against it.
func startGame(t *testing.T, h *Games, user, word string) int64 {
	t.Helper()
	var id int64
	if err := h.DB.QueryRow(
		"INSERT INTO games (username, word, status) VALUES ($1, $2, 'playing') RETURNING id",
		user, word).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func guessWord(h *Games, user, word string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.Guess(rec, asUser(user, "POST", "/game/guess", `{"guess":"`+word+`"}`))
	return rec
}

func decodeState(t *testing.T, rec *httptest.ResponseRecorder) gameState {
	t.Helper()
	var s gameState
	if err := json.NewDecoder(rec.Body).Decode(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestWords_AllFiveUppercaseLetters(t *testing.T) {
	seen := map[string]bool{}
	for _, w := range words {
		if len(w) != WordLength {
			t.Errorf("%q is not %d letters", w, WordLength)
		}
		for _, r := range w {
			if r < 'A' || r > 'Z' {
				t.Errorf("%q contains non A-Z letter %q", w, r)
			}
		}
		if seen[w] {
			t.Errorf("%q appears twice", w)
		}
		seen[w] = true
	}
}

func TestScore(t *testing.T) {
	cases := []struct {
		word, guess string
		want        []string
	}{
		{"FIORE", "FIORE", []string{"correct", "correct", "correct", "correct", "correct"}},
		{"FIORE", "ZUCCA", []string{"absent", "absent", "absent", "absent", "absent"}},
		// duplicate letters: only one S left after the two exact matches
		{"SASSO", "SPOSA", []string{"correct", "absent", "present", "correct", "present"}},
		// guessed letter repeated more often than it occurs in the word
		{"AMORE", "EEEEE", []string{"absent", "absent", "absent", "absent", "correct"}},
	}
	for _, c := range cases {
		got := score(c.word, c.guess)
		for i := range c.want {
			if got[i] != c.want[i] {
				t.Errorf("score(%s, %s) = %v, want %v", c.word, c.guess, got, c.want)
				break
			}
		}
	}
}

func TestCurrentGame_CreatesOne(t *testing.T) {
	h := setupGames(t)

	rec := httptest.NewRecorder()
	h.Current(rec, asUser("ann", "GET", "/game", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	s := decodeState(t, rec)
	if s.ID == 0 || s.Status != "playing" || len(s.Guesses) != 0 {
		t.Errorf("expected a fresh playing game, got %+v", s)
	}
	if s.Word != "" {
		t.Errorf("answer leaked in an unfinished game: %+v", s)
	}
}

func TestCurrentGame_ScopedToUser(t *testing.T) {
	h := setupGames(t)
	annID := startGame(t, h, "ann", "FIORE")

	rec := httptest.NewRecorder()
	h.Current(rec, asUser("bob", "GET", "/game", ""))

	s := decodeState(t, rec)
	if s.ID == annID {
		t.Errorf("bob got ann's game: %+v", s)
	}
}

func TestGuess_Win(t *testing.T) {
	h := setupGames(t)
	startGame(t, h, "ann", "FIORE")

	rec := guessWord(h, "ann", "fiore") // lowercase input is normalized

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	s := decodeState(t, rec)
	if s.Status != "won" || s.Word != "FIORE" {
		t.Errorf("expected a won game revealing the word, got %+v", s)
	}
	if len(s.Guesses) != 1 || s.Guesses[0].Result[0] != "correct" {
		t.Errorf("expected one all-correct guess, got %+v", s.Guesses)
	}
}

func TestGuess_WrongKeepsPlaying(t *testing.T) {
	h := setupGames(t)
	startGame(t, h, "ann", "FIORE")

	s := decodeState(t, guessWord(h, "ann", "ZUCCA"))

	if s.Status != "playing" || len(s.Guesses) != 1 {
		t.Errorf("expected game still playing with 1 guess, got %+v", s)
	}
	if s.Word != "" {
		t.Errorf("answer leaked in an unfinished game: %+v", s)
	}
}

func TestGuess_SixWrongLoses(t *testing.T) {
	h := setupGames(t)
	startGame(t, h, "ann", "FIORE")

	wrong := []string{"ZUCCA", "PIZZA", "GATTO", "AMORE", "LATTE", "MONDO"}
	var last *httptest.ResponseRecorder
	for _, w := range wrong {
		last = guessWord(h, "ann", w)
	}

	s := decodeState(t, last)
	if s.Status != "lost" || s.Word != "FIORE" {
		t.Errorf("expected a lost game revealing the word, got %+v", s)
	}
	if len(s.Guesses) != MaxGuesses {
		t.Errorf("expected %d guesses, got %d", MaxGuesses, len(s.Guesses))
	}
}

func TestGuess_NotInWordList(t *testing.T) {
	h := setupGames(t)
	startGame(t, h, "ann", "FIORE")

	if rec := guessWord(h, "ann", "QQQQQ"); rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestGuess_WrongLength(t *testing.T) {
	h := setupGames(t)
	startGame(t, h, "ann", "FIORE")

	for _, g := range []string{"", "AMO", "AMOREVOLE", "CAFFÈ"} {
		rec := httptest.NewRecorder()
		h.Guess(rec, asUser("ann", "POST", "/game/guess", `{"guess":"`+g+`"}`))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("guess %q: expected 400, got %d", g, rec.Code)
		}
	}
}

func TestGuess_AfterGameOver(t *testing.T) {
	h := setupGames(t)
	startGame(t, h, "ann", "FIORE")
	guessWord(h, "ann", "FIORE") // win it

	if rec := guessWord(h, "ann", "ZUCCA"); rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestNewGame_WhilePlaying(t *testing.T) {
	h := setupGames(t)
	startGame(t, h, "ann", "FIORE")

	rec := httptest.NewRecorder()
	h.New(rec, asUser("ann", "POST", "/game", ""))

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestNewGame_AfterFinished(t *testing.T) {
	h := setupGames(t)
	old := startGame(t, h, "ann", "FIORE")
	guessWord(h, "ann", "FIORE") // win it

	rec := httptest.NewRecorder()
	h.New(rec, asUser("ann", "POST", "/game", ""))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	s := decodeState(t, rec)
	if s.ID == old || s.Status != "playing" || len(s.Guesses) != 0 {
		t.Errorf("expected a fresh game, got %+v", s)
	}
}

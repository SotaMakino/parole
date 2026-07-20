package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"example.com/parole/middleware"
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

// fixed round used by most tests; English: TRAIN, BANK, MUSIC, LION, PARK.
var testRound = []string{"TRENO", "BANCA", "MUSICA", "LEONE", "PARCO"}

// startRound inserts a playing round with known words.
func startRound(t *testing.T, h *Games, user string, ws []string) int64 {
	t.Helper()
	var id int64
	if err := h.DB.QueryRow(
		"INSERT INTO games (username, word, status) VALUES ($1, $2, 'playing') RETURNING id",
		user, strings.Join(ws, ",")).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// finishRound records a completed round directly, for curriculum tests.
func finishRound(t *testing.T, h *Games, user string, ws []string, status string) {
	t.Helper()
	if _, err := h.DB.Exec(
		"INSERT INTO games (username, word, status) VALUES ($1, $2, $3)",
		user, strings.Join(ws, ","), status); err != nil {
		t.Fatal(err)
	}
}

func guessLetter(h *Games, user, letter string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.Guess(rec, asUser(user, "POST", "/game/guess", `{"guess":"`+letter+`"}`))
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

func curriculum(i int) string { return words[i].Italian }

func firstN(n int) []string {
	ws := make([]string, n)
	for i := range ws {
		ws[i] = curriculum(i)
	}
	return ws
}

func TestWords_UppercaseAndUnique(t *testing.T) {
	if len(words) < 500 {
		t.Errorf("expected at least 500 words, got %d", len(words))
	}
	seen := map[string]bool{}
	for _, v := range words {
		for _, w := range []string{v.Italian, v.English} {
			if w == "" {
				t.Errorf("entry %+v has an empty side", v)
			}
			for _, r := range w {
				if r < 'A' || r > 'Z' {
					t.Errorf("%q contains non A-Z letter %q", w, r)
				}
			}
		}
		if seen[v.Italian] {
			t.Errorf("%q appears twice", v.Italian)
		}
		seen[v.Italian] = true
	}
}

func TestCurrentGame_CreatesRandomRound(t *testing.T) {
	h := setupGames(t)

	rec := httptest.NewRecorder()
	h.Current(rec, asUser("ann", "GET", "/game", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	s := decodeState(t, rec)
	if s.ID == 0 || s.Status != "playing" || len(s.Guessed) != 0 {
		t.Errorf("expected a fresh playing round, got %+v", s)
	}
	if len(s.Pairs) != WordsPerRound {
		t.Fatalf("expected %d pairs, got %d", WordsPerRound, len(s.Pairs))
	}
	seen := map[string]bool{}
	for i, p := range s.Pairs {
		if english[p.Italian] == "" {
			t.Errorf("pair %d: %q is not in the word list", i, p.Italian)
		}
		if seen[p.Italian] {
			t.Errorf("%q served twice in one round", p.Italian)
		}
		seen[p.Italian] = true
		if len(p.English) != len(english[p.Italian]) {
			t.Errorf("pair %d: expected %d blanks, got %d",
				i, len(english[p.Italian]), len(p.English))
		}
		for _, l := range p.English {
			if l != "" {
				t.Errorf("letter leaked in a fresh round: %+v", p)
			}
		}
	}
}

func TestGuess_HitRevealsEveryOccurrence(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)

	rec := guessLetter(h, "ann", "a") // lowercase input is normalized

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	s := decodeState(t, rec)
	if len(s.Wrong) != 0 {
		t.Errorf("a hit must not count as a miss: %+v", s)
	}
	// TRAIN -> _ _ A _ _ and PARK -> _ A _ _
	if s.Pairs[0].English[2] != "A" || s.Pairs[4].English[1] != "A" {
		t.Errorf("expected every A revealed, got %+v", s.Pairs)
	}
	if s.Pairs[0].English[0] != "" {
		t.Errorf("unguessed letter revealed: %+v", s.Pairs[0])
	}
}

func TestGuess_MissIsRecordedButNeverEndsTheRound(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)

	var last *httptest.ResponseRecorder
	// far more misses than the review threshold — the round must keep going
	for _, l := range []string{"Z", "Q", "J", "X", "V", "W", "F", "G", "D", "E"} {
		last = guessLetter(h, "ann", l)
	}

	s := decodeState(t, last)
	if s.Status != "playing" {
		t.Errorf("misses must never end the round, got %+v", s)
	}
	// none of the ten letters occur in TRAIN BANK MUSIC LION PARK
	if len(s.Wrong) != 10 || s.Wrong[0] != "Z" {
		t.Errorf("expected ten recorded misses, got %+v", s.Wrong)
	}
}

func TestGuess_AllLettersWin(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)

	var last *httptest.ResponseRecorder
	// every distinct letter of TRAIN BANK MUSIC LION PARK
	for _, l := range strings.Split("TRAINBKMUSCLOP", "") {
		last = guessLetter(h, "ann", l)
	}

	s := decodeState(t, last)
	if s.Status != "won" || len(s.Wrong) != 0 {
		t.Errorf("expected a clean win, got %+v", s)
	}
}

func TestGuess_ManyMissesFlagsForReview(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)

	// six misses — one over the review threshold
	for _, l := range []string{"Z", "Q", "J", "X", "V", "W"} {
		guessLetter(h, "ann", l)
	}
	var last *httptest.ResponseRecorder
	for _, l := range strings.Split("TRAINBKMUSCLOP", "") {
		last = guessLetter(h, "ann", l)
	}

	s := decodeState(t, last)
	if s.Status != "lost" {
		t.Errorf("expected the round flagged for review, got %+v", s)
	}
	if strings.Join(s.Pairs[0].English, "") != "TRAIN" {
		t.Errorf("expected TRAIN fully revealed, got %+v", s.Pairs[0])
	}
}

func TestGuess_RepeatLetterRejected(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)
	guessLetter(h, "ann", "A")

	if rec := guessLetter(h, "ann", "A"); rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for a repeated letter, got %d", rec.Code)
	}
}

func TestGuess_InvalidInput(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)

	for _, g := range []string{"", "AB", "1", "È"} {
		rec := httptest.NewRecorder()
		h.Guess(rec, asUser("ann", "POST", "/game/guess", `{"guess":"`+g+`"}`))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("guess %q: expected 400, got %d", g, rec.Code)
		}
	}
}

func TestGuess_AfterGameOver(t *testing.T) {
	h := setupGames(t)
	finishRound(t, h, "ann", testRound, "won")

	if rec := guessLetter(h, "ann", "A"); rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestRetry_RepeatsTheSameWords(t *testing.T) {
	h := setupGames(t)
	old := startRound(t, h, "ann", testRound)
	if _, err := h.DB.Exec("UPDATE games SET status = 'lost' WHERE id = $1", old); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.Retry(rec, asUser("ann", "POST", "/game/retry", ""))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	s := decodeState(t, rec)
	if s.ID == old || s.Status != "playing" || len(s.Guessed) != 0 {
		t.Errorf("expected a fresh retry round, got %+v", s)
	}
	for i, p := range s.Pairs {
		if p.Italian != testRound[i] {
			t.Errorf("pair %d: expected %q, got %q", i, testRound[i], p.Italian)
		}
		for _, l := range p.English {
			if l != "" {
				t.Errorf("letter leaked in a retry round: %+v", p)
			}
		}
	}
}

func TestRetry_WhilePlaying(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)

	rec := httptest.NewRecorder()
	h.Retry(rec, asUser("ann", "POST", "/game/retry", ""))

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestNewGame_WhilePlaying(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)

	rec := httptest.NewRecorder()
	h.New(rec, asUser("ann", "POST", "/game", ""))

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestCurrentGame_ScopedToUser(t *testing.T) {
	h := setupGames(t)
	annID := startRound(t, h, "ann", testRound)

	rec := httptest.NewRecorder()
	h.Current(rec, asUser("bob", "GET", "/game", ""))

	if s := decodeState(t, rec); s.ID == annID {
		t.Errorf("bob got ann's round: %+v", s)
	}
}

func TestNextWords_RandomButNeverRepeatsPlayedWords(t *testing.T) {
	h := setupGames(t)
	finishRound(t, h, "ann", firstN(5), "won")

	ws, err := h.nextWords("ann")
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != WordsPerRound {
		t.Fatalf("expected %d words, got %v", WordsPerRound, ws)
	}
	played := map[string]bool{}
	for _, w := range firstN(5) {
		played[w] = true
	}
	seen := map[string]bool{}
	for _, w := range ws {
		if english[w] == "" {
			t.Errorf("%q is not in the word list", w)
		}
		if played[w] {
			t.Errorf("%q was already won and must not repeat yet", w)
		}
		if seen[w] {
			t.Errorf("%q served twice in one round", w)
		}
		seen[w] = true
	}
}

func TestNextWords_MissedRoundComesBackAfterGap(t *testing.T) {
	h := setupGames(t)
	finishRound(t, h, "ann", firstN(5), "lost")
	for i := 0; i < ReviewGap; i++ {
		finishRound(t, h, "ann", firstN(5*(i+2))[5*(i+1):], "won")
	}

	ws, err := h.nextWords("ann")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(ws, ",") != strings.Join(firstN(5), ",") {
		t.Errorf("expected the missed words %v to return, got %v", firstN(5), ws)
	}
}

func TestNextWords_MissedRoundNotDueYet(t *testing.T) {
	h := setupGames(t)
	finishRound(t, h, "ann", firstN(5), "lost")
	finishRound(t, h, "ann", firstN(10)[5:], "won")

	ws, err := h.nextWords("ann")
	if err != nil {
		t.Fatal(err)
	}
	// the loss is only one round old, so none of the ten played words may
	// appear yet — the round must be filled from unseen words instead
	played := map[string]bool{}
	for _, w := range firstN(10) {
		played[w] = true
	}
	for _, w := range ws {
		if played[w] {
			t.Errorf("%q is not due yet but was served: %v", w, ws)
		}
	}
}

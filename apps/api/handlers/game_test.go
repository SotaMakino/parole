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

func TestCurrentGame_CreatesFirstCurriculumRound(t *testing.T) {
	h := setupGames(t)

	rec := httptest.NewRecorder()
	h.Current(rec, asUser("ann", "GET", "/game", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	s := decodeState(t, rec)
	if s.ID == 0 || s.Status != "playing" || s.TriesLeft != MaxTries {
		t.Errorf("expected a fresh playing round, got %+v", s)
	}
	if len(s.Pairs) != WordsPerRound {
		t.Fatalf("expected %d pairs, got %d", WordsPerRound, len(s.Pairs))
	}
	for i, p := range s.Pairs {
		if p.Italian != curriculum(i) {
			t.Errorf("pair %d: expected %q, got %q", i, curriculum(i), p.Italian)
		}
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
	if s.TriesLeft != MaxTries || len(s.Wrong) != 0 {
		t.Errorf("a hit must not cost a try: %+v", s)
	}
	// TRAIN -> _ _ A _ _ and PARK -> _ A _ _
	if s.Pairs[0].English[2] != "A" || s.Pairs[4].English[1] != "A" {
		t.Errorf("expected every A revealed, got %+v", s.Pairs)
	}
	if s.Pairs[0].English[0] != "" {
		t.Errorf("unguessed letter revealed: %+v", s.Pairs[0])
	}
}

func TestGuess_MissCostsATry(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)

	s := decodeState(t, guessLetter(h, "ann", "Z"))

	if s.Status != "playing" || s.TriesLeft != MaxTries-1 {
		t.Errorf("expected one try spent, got %+v", s)
	}
	if len(s.Wrong) != 1 || s.Wrong[0] != "Z" {
		t.Errorf("expected Z recorded as wrong, got %+v", s.Wrong)
	}
}

func TestGuess_FiveMissesLose(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)

	var last *httptest.ResponseRecorder
	for _, l := range []string{"Z", "Q", "J", "X", "V"} { // none occur in the round
		last = guessLetter(h, "ann", l)
	}

	s := decodeState(t, last)
	if s.Status != "lost" || s.TriesLeft != 0 {
		t.Errorf("expected a lost round, got %+v", s)
	}
	// the answers are revealed on loss
	if strings.Join(s.Pairs[0].English, "") != "TRAIN" {
		t.Errorf("expected TRAIN revealed, got %+v", s.Pairs[0])
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
	if s.Status != "won" || s.TriesLeft != MaxTries {
		t.Errorf("expected a clean win, got %+v", s)
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

func TestNextWords_AdvancesThroughCurriculum(t *testing.T) {
	h := setupGames(t)
	finishRound(t, h, "ann", firstN(5), "won")

	ws, err := h.nextWords("ann")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{curriculum(5), curriculum(6), curriculum(7), curriculum(8), curriculum(9)}
	if strings.Join(ws, ",") != strings.Join(want, ",") {
		t.Errorf("expected %v, got %v", want, ws)
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
	want := firstN(15)[10:]
	if strings.Join(ws, ",") != strings.Join(want, ",") {
		t.Errorf("expected the next unseen words %v, got %v", want, ws)
	}
}

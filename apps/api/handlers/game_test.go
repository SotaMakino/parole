package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
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

// place attempts one letter on one tile.
func place(h *Games, user, letter string, word, pos int) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	body := fmt.Sprintf(`{"guess":%q,"word":%d,"position":%d}`, letter, word, pos)
	h.Guess(rec, asUser(user, "POST", "/game/guess", body))
	return rec
}

// solveRound places one correct tile per distinct letter; each placement
// reveals that letter everywhere, which completes the round.
func solveRound(h *Games, user string, ws []string) *httptest.ResponseRecorder {
	var last *httptest.ResponseRecorder
	placed := map[string]bool{}
	for wi, w := range ws {
		e := english[w]
		for i := 0; i < len(e); i++ {
			l := string(e[i])
			if !placed[l] {
				placed[l] = true
				last = place(h, user, l, wi, i)
			}
		}
	}
	return last
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

func TestGuess_CorrectPlacementRevealsEveryOccurrence(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)

	rec := place(h, "ann", "a", 0, 2) // lowercase input is normalized; TRAIN has A at 2

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	s := decodeState(t, rec)
	if len(s.Wrong) != 0 {
		t.Errorf("a correct placement must not count as a miss: %+v", s)
	}
	// one correct A opens the A in TRAIN, BANK, and PARK alike
	if s.Pairs[0].English[2] != "A" || s.Pairs[1].English[1] != "A" || s.Pairs[4].English[1] != "A" {
		t.Errorf("expected every A revealed, got %+v", s.Pairs)
	}
	if s.Pairs[0].English[0] != "" {
		t.Errorf("other letters must stay hidden, got %+v", s.Pairs[0])
	}
}

func TestGuess_WrongTileIsAMiss(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)

	// A exists in TRAIN, but not at position 0
	s := decodeState(t, place(h, "ann", "A", 0, 0))

	if s.Status != "playing" {
		t.Errorf("a miss must not end the round, got %+v", s)
	}
	if len(s.Wrong) != 1 || s.Wrong[0] != "A" {
		t.Errorf("expected one recorded miss, got %+v", s.Wrong)
	}
	if s.Pairs[0].English[2] != "" {
		t.Errorf("a wrong placement must not reveal anything, got %+v", s.Pairs[0])
	}
}

func TestGuess_FourMissesKeepPlaying(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)

	var last *httptest.ResponseRecorder
	// Z occurs nowhere in the round: four misses on four different tiles
	for i := 0; i < 4; i++ {
		last = place(h, "ann", "Z", 0, i)
	}

	s := decodeState(t, last)
	if s.Status != "playing" {
		t.Errorf("four misses must not end the round, got %+v", s)
	}
	if len(s.Wrong) != 4 || s.MaxMisses != MaxMisses {
		t.Errorf("expected four recorded misses out of %d, got %+v", MaxMisses, s)
	}
}

func TestGuess_FifthMissLoses(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)

	var last *httptest.ResponseRecorder
	for i := 0; i < 5; i++ {
		last = place(h, "ann", "Z", 0, i)
	}

	s := decodeState(t, last)
	if s.Status != "lost" {
		t.Errorf("expected the fifth miss to end the round, got %+v", s)
	}
	// the answers are revealed on loss
	if strings.Join(s.Pairs[0].English, "") != "TRAIN" {
		t.Errorf("expected TRAIN revealed, got %+v", s.Pairs[0])
	}
}

func TestGuess_FillingEveryTileWins(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)

	s := decodeState(t, solveRound(h, "ann", testRound))

	if s.Status != "won" || len(s.Wrong) != 0 {
		t.Errorf("expected a clean win, got %+v", s)
	}
	if strings.Join(s.Pairs[0].English, "") != "TRAIN" {
		t.Errorf("expected TRAIN fully revealed, got %+v", s.Pairs[0])
	}
}

func TestGuess_WinDespiteSomeMisses(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)

	// four misses, then a full solve: still a win
	for i := 0; i < 4; i++ {
		place(h, "ann", "Z", 0, i)
	}
	s := decodeState(t, solveRound(h, "ann", testRound))

	if s.Status != "won" || len(s.Wrong) != 4 {
		t.Errorf("expected a win with four misses, got %+v", s)
	}
}

func TestGuess_CorrectPlacementUsesUpTheLetter(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)

	// one correct A reveals every A, so the letter is spent immediately
	s := decodeState(t, place(h, "ann", "A", 0, 2))

	found := false
	for _, l := range s.UsedUp {
		found = found || l == "A"
	}
	if !found {
		t.Errorf("every A is revealed, expected it used up: %+v", s.UsedUp)
	}
	for _, l := range s.UsedUp {
		if l == "T" {
			t.Errorf("T was never placed and must not be used up: %+v", s.UsedUp)
		}
	}
}

func TestGuess_RevealedTileRejected(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)
	place(h, "ann", "A", 0, 2)

	if rec := place(h, "ann", "X", 0, 2); rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for a revealed tile, got %d", rec.Code)
	}
}

func TestGuess_RepeatedMissRejected(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)
	place(h, "ann", "Z", 0, 0)

	if rec := place(h, "ann", "Z", 0, 0); rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for the same miss twice, got %d", rec.Code)
	}
	// a different letter on that tile is a fresh attempt
	if rec := place(h, "ann", "Q", 0, 0); rec.Code != http.StatusOK {
		t.Errorf("expected 200 for a new letter on the tile, got %d", rec.Code)
	}
}

func TestGuess_InvalidInput(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)

	for _, g := range []string{"", "AB", "1", "È"} {
		rec := place(h, "ann", g, 0, 0)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("guess %q: expected 400, got %d", g, rec.Code)
		}
	}
	for _, tile := range [][2]int{{-1, 0}, {5, 0}, {0, -1}, {0, 5}} {
		rec := place(h, "ann", "A", tile[0], tile[1])
		if rec.Code != http.StatusBadRequest {
			t.Errorf("tile %v: expected 400, got %d", tile, rec.Code)
		}
	}
}

func TestGuess_AfterGameOver(t *testing.T) {
	h := setupGames(t)
	finishRound(t, h, "ann", testRound, "won")

	if rec := place(h, "ann", "A", 0, 2); rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestCurrentGame_RevisitDealsAFreshRound(t *testing.T) {
	h := setupGames(t)
	id := startRound(t, h, "ann", testRound)
	place(h, "ann", "A", 0, 2)
	place(h, "ann", "Z", 0, 0)

	rec := httptest.NewRecorder()
	h.Current(rec, asUser("ann", "GET", "/game", ""))

	s := decodeState(t, rec)
	if s.ID == id || s.Status != "playing" {
		t.Errorf("expected a brand-new round, got %+v", s)
	}
	if len(s.Guessed) != 0 || len(s.Wrong) != 0 {
		t.Errorf("expected a fresh round with no guesses, got %+v", s)
	}
	for _, p := range s.Pairs {
		for _, l := range p.English {
			if l != "" {
				t.Errorf("letter leaked in a fresh round: %+v", p)
			}
		}
	}
}

func TestCurrentGame_RevisitKeepsAFinishedRound(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)
	solveRound(h, "ann", testRound)

	rec := httptest.NewRecorder()
	h.Current(rec, asUser("ann", "GET", "/game", ""))

	s := decodeState(t, rec)
	if s.Status != "won" || len(s.Guessed) == 0 {
		t.Errorf("expected the finished round untouched, got %+v", s)
	}
}

func TestReset_ClearsGuessesMidRound(t *testing.T) {
	h := setupGames(t)
	id := startRound(t, h, "ann", testRound)
	place(h, "ann", "A", 0, 2)
	place(h, "ann", "Z", 0, 0)

	rec := httptest.NewRecorder()
	h.Reset(rec, asUser("ann", "POST", "/game/reset", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	s := decodeState(t, rec)
	if s.ID != id || s.Status != "playing" || len(s.Guessed) != 0 {
		t.Errorf("expected the same round wiped clean, got %+v", s)
	}
}

func TestReset_AfterGameOver(t *testing.T) {
	h := setupGames(t)
	finishRound(t, h, "ann", testRound, "won")

	rec := httptest.NewRecorder()
	h.Reset(rec, asUser("ann", "POST", "/game/reset", ""))

	if rec.Code != http.StatusConflict {
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

func TestMe_CountsDistinctLearnedWords(t *testing.T) {
	h := setupGames(t)
	// two won rounds share "TRENO", plus a lost round that must not count
	finishRound(t, h, "ann", []string{"TRENO", "BANCA", "MUSICA", "LEONE", "PARCO"}, "won")
	finishRound(t, h, "ann", []string{"TRENO", "GATTO", "CANE", "SOLE", "LUNA"}, "won")
	finishRound(t, h, "ann", []string{"MARE", "FIUME", "LAGO", "CIELO", "VENTO"}, "lost")

	rec := httptest.NewRecorder()
	h.Me(rec, asUser("ann", "GET", "/me", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		Username string `json:"username"`
		Learned  int    `json:"learned"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	// 9 distinct words won (TRENO counted once), lost round excluded
	if body.Username != "ann" || body.Learned != 9 {
		t.Errorf("expected ann with 9 learned, got %+v", body)
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

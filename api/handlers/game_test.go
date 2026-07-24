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
	"time"

	"example.com/le-cinque/middleware"
	"example.com/le-cinque/store"
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
	if _, err := db.Exec("TRUNCATE games, guesses, accounts, sessions, word_reviews, study_days"); err != nil {
		t.Fatal(err)
	}
	return db
}

// freezeClock pins the scheduling clock so tests can travel in time instead of
// waiting for real days to pass.
func freezeClock(t *testing.T, at time.Time) {
	t.Helper()
	prev := now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = prev })
}

// setReview writes a word's review record directly, standing in for rounds
// already played.
func setReview(t *testing.T, h *Games, user, word string, dueAt, lastSeen time.Time, streak int) {
	t.Helper()
	if _, err := h.DB.Exec(
		`INSERT INTO word_reviews (username, word, due_at, last_seen, streak, learned)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (username, word) DO UPDATE SET
		   due_at = EXCLUDED.due_at, last_seen = EXCLUDED.last_seen, streak = EXCLUDED.streak,
		   learned = word_reviews.learned OR EXCLUDED.learned`,
		user, word, dueAt, lastSeen, streak, streak > 0); err != nil {
		t.Fatal(err)
	}
}

// readReview returns one word's review record.
func readReview(t *testing.T, h *Games, user, word string) (review, bool) {
	t.Helper()
	var r review
	err := h.DB.QueryRow(
		"SELECT due_at, last_seen, streak FROM word_reviews WHERE username = $1 AND word = $2",
		user, word).Scan(&r.dueAt, &r.lastSeen, &r.streak)
	if err == sql.ErrNoRows {
		return review{}, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return r, true
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

// startRoundDir inserts a playing round with a chosen guessing direction.
func startRoundDir(t *testing.T, h *Games, user string, ws []string, dir string) int64 {
	t.Helper()
	var id int64
	if err := h.DB.QueryRow(
		"INSERT INTO games (username, word, status, direction) VALUES ($1, $2, 'playing', $3) RETURNING id",
		user, strings.Join(ws, ","), dir).Scan(&id); err != nil {
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

// entry looks a curriculum entry up by its Italian word.
func entry(t *testing.T, italian string) vocab {
	t.Helper()
	for _, v := range words {
		if v.Italian == italian {
			return v
		}
	}
	t.Fatalf("%q is not in the curriculum", italian)
	return vocab{}
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
		if english[p.Prompt] == "" {
			t.Errorf("pair %d: %q is not in the word list", i, p.Prompt)
		}
		if seen[p.Prompt] {
			t.Errorf("%q served twice in one round", p.Prompt)
		}
		seen[p.Prompt] = true
		if len(p.Tiles) != len(english[p.Prompt]) {
			t.Errorf("pair %d: expected %d blanks, got %d",
				i, len(english[p.Prompt]), len(p.Tiles))
		}
		for _, l := range p.Tiles {
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
	if s.Pairs[0].Tiles[2] != "A" || s.Pairs[1].Tiles[1] != "A" || s.Pairs[4].Tiles[1] != "A" {
		t.Errorf("expected every A revealed, got %+v", s.Pairs)
	}
	if s.Pairs[0].Tiles[0] != "" {
		t.Errorf("other letters must stay hidden, got %+v", s.Pairs[0])
	}
}

func TestGuess_ReverseDirectionSpellsItalian(t *testing.T) {
	h := setupGames(t)
	startRoundDir(t, h, "ann", testRound, "en")

	rec := httptest.NewRecorder()
	h.Current(rec, asUser("ann", "GET", "/game", ""))
	s := decodeState(t, rec)
	if s.Direction != "en" {
		t.Fatalf("expected direction en, got %q", s.Direction)
	}
	// the English word is shown as the clue; the Italian word is spelled out
	if s.Pairs[0].Prompt != "TRAIN" {
		t.Errorf("expected English clue TRAIN, got %q", s.Pairs[0].Prompt)
	}
	if len(s.Pairs[0].Tiles) != len("TRENO") {
		t.Errorf("expected %d tiles for TRENO, got %d", len("TRENO"), len(s.Pairs[0].Tiles))
	}
	// placing T on the first tile is correct against TRENO, not TRAIN
	s = decodeState(t, place(h, "ann", "T", 0, 0))
	if s.Pairs[0].Tiles[0] != "T" || len(s.Wrong) != 0 {
		t.Errorf("expected T revealed on the Italian word, got %+v", s.Pairs[0])
	}
}

func TestSetDirection_FlipsAndDealsFreshWords(t *testing.T) {
	h := setupGames(t)
	id := startRound(t, h, "ann", testRound) // default direction "it"

	rec := httptest.NewRecorder()
	h.SetDirection(rec, asUser("ann", "POST", "/game/direction", `{"direction":"en"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	s := decodeState(t, rec)
	// same untouched row, now in the English direction with a fresh, blank round
	if s.ID != id || s.Direction != "en" || len(s.Pairs) != WordsPerRound || len(s.Guessed) != 0 {
		t.Errorf("expected the same round flipped to a fresh en round, got %+v", s)
	}
	for i, p := range s.Pairs {
		// in the en direction the prompt is an English word from the curriculum
		found := false
		for _, v := range words {
			found = found || v.English == p.Prompt
		}
		if !found {
			t.Errorf("pair %d: %q is not an English curriculum word", i, p.Prompt)
		}
		for _, l := range p.Tiles {
			if l != "" {
				t.Errorf("letter leaked after a flip: %+v", p)
			}
		}
	}
}

func TestSetDirection_RejectedAfterFirstGuess(t *testing.T) {
	h := setupGames(t)
	startRound(t, h, "ann", testRound)
	place(h, "ann", "A", 0, 2) // a correct placement — the round is now underway

	rec := httptest.NewRecorder()
	h.SetDirection(rec, asUser("ann", "POST", "/game/direction", `{"direction":"en"}`))
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 once a letter is placed, got %d", rec.Code)
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
	if s.Pairs[0].Tiles[2] != "" {
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
	if strings.Join(s.Pairs[0].Tiles, "") != "TRAIN" {
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
	if strings.Join(s.Pairs[0].Tiles, "") != "TRAIN" {
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

func TestCurrentGame_RevisitResumesRound(t *testing.T) {
	h := setupGames(t)
	id := startRound(t, h, "ann", testRound)
	place(h, "ann", "A", 0, 2)
	place(h, "ann", "Z", 0, 0)

	rec := httptest.NewRecorder()
	h.Current(rec, asUser("ann", "GET", "/game", ""))

	// refreshing resumes the same in-progress round rather than dealing a new
	// one, so the id, the words, and the placements so far all survive
	s := decodeState(t, rec)
	if s.ID != id || s.Status != "playing" {
		t.Errorf("expected the in-progress round resumed, got %+v", s)
	}
	if len(s.Guessed) != 2 {
		t.Errorf("expected the two placements preserved, got %+v", s.Guessed)
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

func TestMe_CountsRetrievedWordsOnly(t *testing.T) {
	h := setupGames(t)
	at := time.Now()
	// three words the player produced themselves, plus one that a round revealed
	// through another word's letters — the latter must not count as learned
	setReview(t, h, "ann", "TRENO", at, at, 1)
	setReview(t, h, "ann", "BANCA", at, at, 3)
	setReview(t, h, "ann", "MUSICA", at, at, 2)
	setReview(t, h, "ann", "LEONE", at, at, 0)

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
	if body.Username != "ann" || body.Learned != 3 {
		t.Errorf("expected ann with 3 learned, got %+v", body)
	}
}

// A word the player retrieved once stays in the vocabulary tally even after a
// later round reveals it for free and resets its streak to 0. Without the sticky
// "learned" flag the count would churn instead of only growing.
func TestMe_KeepsLearnedWordAfterStreakReset(t *testing.T) {
	h := setupGames(t)
	at := time.Now()
	// retrieved once (streak 1), then revealed for free in a later round: the
	// scheduler drops the streak back to 0 but the word is still learned
	setReview(t, h, "ann", "TRENO", at, at, 1)
	setReview(t, h, "ann", "TRENO", at, at, 0)

	rec := httptest.NewRecorder()
	h.Me(rec, asUser("ann", "GET", "/me", ""))

	var body struct {
		Learned int `json:"learned"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Learned != 1 {
		t.Errorf("expected 1 learned after streak reset, got %d", body.Learned)
	}
}

func TestMe_ActivityCalendarCountsRetrievalsPerDay(t *testing.T) {
	h := setupGames(t)
	at := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	freezeClock(t, at)

	// three retrievals today, two five days ago; everything else untouched
	if _, err := h.DB.Exec(
		`INSERT INTO study_days (username, day, count) VALUES ('ann', $1::date, 3), ('ann', $2::date, 2)`,
		at, at.AddDate(0, 0, -5)); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.Me(rec, asUser("ann", "GET", "/me", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		Activity []int `json:"activity"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	// the dense window runs from a Sunday through today, so its length is fixed
	// by today's weekday plus twelve whole weeks
	today := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, at.Location())
	wantLen := int(today.Weekday()) + 7*12 + 1
	if len(body.Activity) != wantLen {
		t.Fatalf("expected %d days, got %d", wantLen, len(body.Activity))
	}
	// counting from the end: today is last, five days back is five cells earlier,
	// and any untouched day reads zero
	if last := body.Activity[len(body.Activity)-1]; last != 3 {
		t.Errorf("expected today's count 3, got %d", last)
	}
	if five := body.Activity[len(body.Activity)-6]; five != 2 {
		t.Errorf("expected count 2 five days ago, got %d", five)
	}
	if idle := body.Activity[len(body.Activity)-3]; idle != 0 {
		t.Errorf("expected 0 for an unstudied day, got %d", idle)
	}
}

func TestRecordReviews_LeakedWordEarnsNothing(t *testing.T) {
	h := setupGames(t)
	at := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	freezeClock(t, at)

	// LEMON and MELON are anagrams, so spelling LEMON reveals MELON outright:
	// solveRound never places a letter on the second word
	round := []string{"LIMONE", "MELONE", "TRENO", "BANCA", "PARCO"}
	startRound(t, h, "ann", round)
	if s := decodeState(t, solveRound(h, "ann", round)); s.Status != "won" {
		t.Fatalf("expected the round won, got %+v", s)
	}

	// the four words the player spelled advance to a streak of one, due tomorrow
	for _, w := range []string{"LIMONE", "TRENO", "BANCA", "PARCO"} {
		r, ok := readReview(t, h, "ann", w)
		if !ok {
			t.Fatalf("%q has no review record", w)
		}
		if r.streak != 1 {
			t.Errorf("%q: expected streak 1, got %d", w, r.streak)
		}
		if !r.dueAt.Equal(at.AddDate(0, 0, reviewDays[0])) {
			t.Errorf("%q: expected due in %d day(s), got %v", w, reviewDays[0], r.dueAt)
		}
	}
	// MELONE was never recalled — it only appeared. No credit, due again next
	// session rather than in a day.
	r, ok := readReview(t, h, "ann", "MELONE")
	if !ok {
		t.Fatal("MELONE has no review record")
	}
	if r.streak != 0 {
		t.Errorf("MELONE was revealed by LIMONE's letters, expected streak 0, got %d", r.streak)
	}
	if r.dueAt.After(at) {
		t.Errorf("expected MELONE due immediately, got %v", r.dueAt)
	}
}

// Winning credits every word toward the vocabulary tally, including one that
// filled in entirely from another word's letters: the player produced all of
// its letters, so they can spell it even though its streak stays 0 (see
// TestRecordReviews_LeakedWordEarnsNothing for the scheduling side).
func TestMe_CreditsAutoRevealedWordOnWin(t *testing.T) {
	h := setupGames(t)
	at := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	freezeClock(t, at)

	round := []string{"LIMONE", "MELONE", "TRENO", "BANCA", "PARCO"}
	startRound(t, h, "ann", round)
	if s := decodeState(t, solveRound(h, "ann", round)); s.Status != "won" {
		t.Fatalf("expected the round won, got %+v", s)
	}

	rec := httptest.NewRecorder()
	h.Me(rec, asUser("ann", "GET", "/me", ""))
	var body struct {
		Learned int `json:"learned"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Learned != 5 {
		t.Errorf("expected all 5 words learned on a win, got %d", body.Learned)
	}
}

func TestRecordReviews_LadderExpandsAndResets(t *testing.T) {
	h := setupGames(t)
	at := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	freezeClock(t, at)

	// a word already retrieved twice running steps to the third rung
	setReview(t, h, "ann", "TRENO", at, at.AddDate(0, 0, -3), 2)
	startRound(t, h, "ann", testRound)
	solveRound(h, "ann", testRound)

	r, _ := readReview(t, h, "ann", "TRENO")
	if r.streak != 3 {
		t.Fatalf("expected streak 3, got %d", r.streak)
	}
	if !r.dueAt.Equal(at.AddDate(0, 0, reviewDays[2])) {
		t.Errorf("expected due in %d days, got %v", reviewDays[2], r.dueAt)
	}

	// losing the round without touching a word drops it back to the bottom
	later := at.AddDate(0, 0, 7)
	freezeClock(t, later)
	startRound(t, h, "ann", testRound)
	for i := 0; i < MaxMisses; i++ {
		place(h, "ann", "Z", 0, i)
	}
	r, _ = readReview(t, h, "ann", "TRENO")
	if r.streak != 0 {
		t.Errorf("expected the streak reset after a lost round, got %d", r.streak)
	}
}

func TestRecordReviews_TalliesTheStudyDay(t *testing.T) {
	h := setupGames(t)
	at := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	freezeClock(t, at)

	// solving a round retrieves every word; the day's tally should equal how
	// many distinct words were genuinely produced
	startRound(t, h, "ann", testRound)
	solveRound(h, "ann", testRound)

	var count int
	if err := h.DB.QueryRow(
		"SELECT count FROM study_days WHERE username = 'ann' AND day = $1::date", at).Scan(&count); err != nil {
		t.Fatalf("no study_days row for today: %v", err)
	}
	if count != len(testRound) {
		t.Errorf("expected %d retrievals tallied, got %d", len(testRound), count)
	}

	// and it surfaces on the activity calendar's final (today) cell, with the
	// window starting on a Sunday
	start, cal, err := h.activityCalendar("ann")
	if err != nil {
		t.Fatal(err)
	}
	if start.Weekday() != time.Sunday {
		t.Errorf("expected the window to start on a Sunday, got %v", start.Weekday())
	}
	if last := cal[len(cal)-1]; last != len(testRound) {
		t.Errorf("expected today's cell to read %d, got %d", len(testRound), last)
	}
}

func TestRecordReviews_LostRoundWithNoRetrievalLeavesNoMark(t *testing.T) {
	h := setupGames(t)
	at := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	freezeClock(t, at)

	// five misses without ever placing a correct letter: nothing retrieved, so
	// the day earns no entry on the calendar
	startRound(t, h, "ann", testRound)
	for i := 0; i < MaxMisses; i++ {
		place(h, "ann", "Z", 0, i)
	}

	var n int
	if err := h.DB.QueryRow(
		"SELECT COUNT(*) FROM study_days WHERE username = 'ann'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected no study_days rows after a blank round, got %d", n)
	}
}

func TestDueAfter_ClampsToTheLastRung(t *testing.T) {
	at := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	last := reviewDays[len(reviewDays)-1]
	for _, streak := range []int{len(reviewDays), len(reviewDays) + 5, 99} {
		if got := dueAfter(at, streak); !got.Equal(at.AddDate(0, 0, last)) {
			t.Errorf("streak %d: expected the ladder capped at %d days, got %v", streak, last, got)
		}
	}
	// a zero or negative streak still schedules a review rather than panicking
	if got := dueAfter(at, 0); !got.Equal(at.AddDate(0, 0, reviewDays[0])) {
		t.Errorf("streak 0: expected %d day(s), got %v", reviewDays[0], got)
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

func TestNextWords_NewPlayerGetsOneWordPerTheme(t *testing.T) {
	h := setupGames(t)

	ws, err := h.nextWords("ann", "it")
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != WordsPerRound {
		t.Fatalf("expected %d words, got %v", WordsPerRound, ws)
	}
	// five colors or five numbers in one round would have the words interfere
	// with each other, so the curriculum is dealt across its themes
	themes := map[string]bool{}
	for _, w := range ws {
		v := entry(t, w)
		if themes[v.Theme] {
			t.Errorf("two words from theme %q in one round: %v", v.Theme, ws)
		}
		themes[v.Theme] = true
	}
}

func TestNextWords_EnDirectionSkipsAmbiguousPrompts(t *testing.T) {
	h := setupGames(t)

	// in this direction the English word is the clue, so a translation shared by
	// two Italian words (PUSH is both SPINGERE and SPINTA) is unanswerable
	for i := 0; i < 40; i++ {
		ws, err := h.nextWords("ann", "en")
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range ws {
			if ambiguousEnglish[english[w]] {
				t.Fatalf("%q has the ambiguous prompt %q", w, english[w])
			}
		}
		finishRound(t, h, "ann", ws, "won")
		for _, w := range ws {
			setReview(t, h, "ann", w, now().AddDate(0, 0, 30), now(), 1)
		}
	}
}

func TestNextWords_DueWordsLeadTheRound(t *testing.T) {
	h := setupGames(t)
	at := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	freezeClock(t, at)

	// a word retrieved three days ago and due yesterday — won, not missed, and
	// still scheduled: retrieval is what keeps it, so it comes back
	setReview(t, h, "ann", "GATTO", at.AddDate(0, 0, -1), at.AddDate(0, 0, -3), 2)
	// and one whose next review is still a fortnight out
	setReview(t, h, "ann", "CANE", at.AddDate(0, 0, 14), at.AddDate(0, 0, -7), 3)

	ws, err := h.nextWords("ann", "it")
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) == 0 || ws[0] != "GATTO" {
		t.Errorf("expected the overdue GATTO to lead the round, got %v", ws)
	}
	for _, w := range ws {
		if w == "CANE" {
			t.Errorf("CANE is not due for two weeks but was served: %v", ws)
		}
	}
}

func TestNextWords_HeldBackWithinTheSession(t *testing.T) {
	h := setupGames(t)
	at := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	freezeClock(t, at)

	// due, but only just seen: a run of rounds in one sitting must not stack a
	// review minutes behind its first encounter
	setReview(t, h, "ann", "GATTO", at.Add(-time.Minute), at.Add(-time.Minute), 0)

	ws, err := h.nextWords("ann", "it")
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range ws {
		if w == "GATTO" {
			t.Errorf("GATTO was seen a minute ago and must wait: %v", ws)
		}
	}

	// the next session picks it up
	freezeClock(t, at.Add(SessionGap+time.Minute))
	ws, err = h.nextWords("ann", "it")
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) == 0 || ws[0] != "GATTO" {
		t.Errorf("expected GATTO once the session gap passed, got %v", ws)
	}
}

func TestNextWords_FillsTheRoundWhenGuardsCollide(t *testing.T) {
	h := setupGames(t)
	at := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	freezeClock(t, at)

	// every word seen moments ago: the guards would starve the round, so they
	// relax rather than deal fewer than five words
	for _, v := range words {
		setReview(t, h, "ann", v.Italian, at, at, 1)
	}

	ws, err := h.nextWords("ann", "it")
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != WordsPerRound {
		t.Errorf("expected a full round even under the guards, got %v", ws)
	}
}

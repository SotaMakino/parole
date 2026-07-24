package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"example.com/le-cinque/middleware"
)

const (
	WordsPerRound = 5
	// the fifth wrong placement ends the round as lost
	MaxMisses = 5
	// a word just seen is held back this long, so a run of rounds in one sitting
	// cannot bring a review back minutes after its first encounter
	SessionGap = 2 * time.Hour
)

// reviewDays is the gap before a retrieved word comes back, in days, indexed by
// how many times in a row the player has produced it. Gradually widening gaps
// beat equal ones only slightly, and the amount of spacing matters far more than
// the shape of the schedule, so the ladder stays coarse on purpose.
var reviewDays = []int{1, 3, 7, 21, 60}

// now is the clock scheduling reads. Tests replace it to travel in time.
var now = time.Now

// dueAfter returns when a word retrieved for the streak-th time running is due.
func dueAfter(at time.Time, streak int) time.Time {
	i := streak - 1
	if i < 0 {
		i = 0
	}
	if i >= len(reviewDays) {
		i = len(reviewDays) - 1
	}
	return at.AddDate(0, 0, reviewDays[i])
}

type Games struct {
	DB *sql.DB
}

// game rows store the round's Italian words comma-joined in the word column,
// so the schema is the same as a single-word game.
type game struct {
	id     int64
	words  []string
	status string // playing | won | lost
	// "it" (default): the Italian word is the clue, the English word is spelled.
	// "en": flipped — the English word is the clue, the Italian word is spelled.
	direction string
}

type pair struct {
	Prompt string   `json:"prompt"` // the word shown in full as the clue
	Tiles  []string `json:"tiles"`  // one entry per letter of the answer: revealed letter or ""
}

type gameState struct {
	ID        int64    `json:"id"`
	Status    string   `json:"status"`    // "lost" = completed, flagged for review
	Direction string   `json:"direction"` // "it" | "en" — which word is spelled
	Pairs     []pair   `json:"pairs"`
	Guessed   []string `json:"guessed"`   // the letter of every placement tried, in order
	Results   []bool   `json:"results"`   // parallel to guessed: true = correct placement
	Wrong     []string `json:"wrong"`     // the letters of failed placements, in order
	UsedUp    []string `json:"usedUp"`    // letters whose every occurrence is revealed
	MaxMisses int      `json:"maxMisses"` // wrong placements allowed before losing
}

// answer is the word the player spells out on the tiles; clue is the word shown
// in full. The direction decides which side of the pair is which.
func (g *game) answer(w string) string {
	if g.direction == "en" {
		return w
	}
	return english[w]
}

func (g *game) clue(w string) string {
	if g.direction == "en" {
		return english[w]
	}
	return w
}

// normalizeDirection coerces client input to a known value, defaulting to "it".
func normalizeDirection(d string) string {
	if d == "en" {
		return "en"
	}
	return "it"
}

func (h *Games) latest(user string) (*game, error) {
	g := &game{}
	var joined string
	err := h.DB.QueryRow(
		"SELECT id, word, status, direction FROM games WHERE username = $1 ORDER BY id DESC LIMIT 1",
		user).Scan(&g.id, &joined, &g.status, &g.direction)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	g.words = strings.Split(joined, ",")
	return g, err
}

// review is a player's record for one word: when it is due again, when they
// last saw it, and how many times running they have retrieved it.
type review struct {
	dueAt    time.Time
	lastSeen time.Time
	streak   int
}

func (h *Games) reviews(user string) (map[string]review, error) {
	rows, err := h.DB.Query(
		"SELECT word, due_at, last_seen, streak FROM word_reviews WHERE username = $1", user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]review{}
	for rows.Next() {
		var w string
		var r review
		if err := rows.Scan(&w, &r.dueAt, &r.lastSeen, &r.streak); err != nil {
			return nil, err
		}
		out[w] = r
	}
	return out, rows.Err()
}

// recordReviews writes one row per word of a finished round. Only a word the
// player actually produced advances its ladder: a correct placement opens that
// letter across every word on the board, so a word can finish fully revealed
// without ever having been recalled. Those reset to a streak of zero and come
// due again at the next session.
func (h *Games) recordReviews(user string, g *game, attempts []attempt) error {
	revs, err := h.reviews(user)
	if err != nil {
		return err
	}
	retrieved := map[int]bool{}
	for _, a := range attempts {
		// Guess turns away a placement on an already revealed tile, so every
		// correct attempt is a letter the player produced for that word
		if g.correct(a) {
			retrieved[a.word] = true
		}
	}
	at := now()
	for wi, w := range g.words {
		streak, due := 0, at
		if retrieved[wi] {
			streak = revs[w].streak + 1
			due = dueAfter(at, streak)
		}
		if _, err := h.DB.Exec(
			`INSERT INTO word_reviews (username, word, due_at, last_seen, streak)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (username, word) DO UPDATE SET
			   due_at = EXCLUDED.due_at,
			   last_seen = EXCLUDED.last_seen,
			   streak = EXCLUDED.streak`,
			user, w, due, at, streak); err != nil {
			return err
		}
	}
	// tally the day's genuine retrievals for the activity calendar; a round in
	// which the player recalled nothing leaves no mark on the grid
	if len(retrieved) > 0 {
		if _, err := h.DB.Exec(
			`INSERT INTO study_days (username, day, count) VALUES ($1, $2::date, $3)
			 ON CONFLICT (username, day) DO UPDATE SET count = study_days.count + EXCLUDED.count`,
			user, at, len(retrieved)); err != nil {
			return err
		}
	}
	return nil
}

// activityCalendar returns daily retrieval counts for a GitHub-style heatmap
// along with the window's start day. The window begins on the Sunday on or
// before 12 weeks ago and runs through today (13 columns once chunked into
// 7-day weeks), so the dense slice the client receives aligns cleanly into
// weekday rows; the start day lets the client date each cell. No-study days
// are 0.
func (h *Games) activityCalendar(user string) (time.Time, []int, error) {
	at := now()
	today := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, at.Location())
	start := today.AddDate(0, 0, -int(today.Weekday())-7*12)
	rows, err := h.DB.Query(
		"SELECT day, count FROM study_days WHERE username = $1 AND day >= $2::date", user, start)
	if err != nil {
		return start, nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var day time.Time
		var c int
		if err := rows.Scan(&day, &c); err != nil {
			return start, nil, err
		}
		counts[day.Format("2006-01-02")] = c
	}
	if err := rows.Err(); err != nil {
		return start, nil, err
	}
	out := []int{}
	for d := start; !d.After(today); d = d.AddDate(0, 0, 1) {
		out = append(out, counts[d.Format("2006-01-02")])
	}
	return start, out, nil
}

// nextWords picks a round's words. Priority:
//  1. review — words whose due date has arrived, most overdue first. Words the
//     player got right are scheduled too, not only the ones they missed:
//     retrieving a word is what fixes it in memory, so a word that stops being
//     tested is a word being forgotten.
//  2. words the player has never met, in curriculum order — words.tsv is
//     ordered most-essential first, so beginners meet the core words soonest
//  3. filler, once the curriculum runs out: whatever was seen longest ago
//
// Two guards apply to every tier. No two words in a round share a theme, because
// meeting a whole semantic set at once — five colors, five numbers — makes the
// words interfere with one another. And a word seen within SessionGap is held
// back, so a long sitting cannot stack a review right behind its first
// encounter. Both relax on a second pass rather than deal a short round.
func (h *Games) nextWords(user, direction string) ([]string, error) {
	revs, err := h.reviews(user)
	if err != nil {
		return nil, err
	}
	at := now()

	picked := []string{}
	taken := map[string]bool{}
	usedTheme := map[string]bool{}
	relaxed := false
	take := func(v vocab) {
		if len(picked) >= WordsPerRound || taken[v.Italian] {
			return
		}
		// here the English word is the prompt, so it has to name exactly one
		// Italian word for the tiles to be answerable
		if direction == "en" && ambiguousEnglish[v.English] {
			return
		}
		if !relaxed {
			if usedTheme[v.Theme] {
				return
			}
			if r, ok := revs[v.Italian]; ok && at.Sub(r.lastSeen) < SessionGap {
				return
			}
		}
		taken[v.Italian] = true
		usedTheme[v.Theme] = true
		picked = append(picked, v.Italian)
	}

	due := []vocab{}
	seen := []vocab{}
	for _, v := range words {
		r, ok := revs[v.Italian]
		if !ok {
			continue
		}
		seen = append(seen, v)
		if !r.dueAt.After(at) {
			due = append(due, v)
		}
	}
	sort.SliceStable(due, func(i, j int) bool {
		return revs[due[i].Italian].dueAt.Before(revs[due[j].Italian].dueAt)
	})
	sort.SliceStable(seen, func(i, j int) bool {
		return revs[seen[i].Italian].lastSeen.Before(revs[seen[j].Italian].lastSeen)
	})

	fill := func() {
		for _, v := range due {
			take(v)
		}
		for _, v := range words {
			if _, met := revs[v.Italian]; !met {
				take(v)
			}
		}
		for _, v := range seen {
			take(v)
		}
	}
	fill()
	relaxed = true
	fill()
	return picked, nil
}

func (h *Games) create(user, direction string) (*game, error) {
	picked, err := h.nextWords(user, direction)
	if err != nil {
		return nil, err
	}
	g := &game{words: picked, status: "playing", direction: direction}
	err = h.DB.QueryRow(
		"INSERT INTO games (username, word, status, direction) VALUES ($1, $2, 'playing', $3) RETURNING id",
		user, strings.Join(picked, ","), direction).Scan(&g.id)
	return g, err
}

// attempt is one letter placed on one tile, stored in guesses.guess as
// "L:word:pos" — e.g. "T:0:3" puts a T on the fourth tile of the first word.
type attempt struct {
	letter string
	word   int
	pos    int
}

func (a attempt) encode() string {
	return fmt.Sprintf("%s:%d:%d", a.letter, a.word, a.pos)
}

func decodeAttempt(s string) attempt {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return attempt{word: -1, pos: -1}
	}
	w, _ := strconv.Atoi(parts[1])
	p, _ := strconv.Atoi(parts[2])
	return attempt{letter: parts[0], word: w, pos: p}
}

// correct reports whether the attempt's letter really sits on that tile.
func (g *game) correct(a attempt) bool {
	if a.word < 0 || a.word >= len(g.words) {
		return false
	}
	e := g.answer(g.words[a.word])
	return a.pos >= 0 && a.pos < len(e) && string(e[a.pos]) == a.letter
}

func (h *Games) attempts(g *game) ([]attempt, error) {
	rows, err := h.DB.Query("SELECT guess FROM guesses WHERE game_id = $1 ORDER BY id", g.id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	attempts := []attempt{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		attempts = append(attempts, decodeAttempt(s))
	}
	return attempts, rows.Err()
}

type tileKey struct{ word, pos int }

func revealedTiles(g *game, attempts []attempt) map[tileKey]bool {
	revealed := map[tileKey]bool{}
	for _, a := range attempts {
		if !g.correct(a) {
			continue
		}
		// a correct placement opens every occurrence of that letter, so the
		// player never has to place the same character twice
		for wi, w := range g.words {
			for i, r := range g.answer(w) {
				if string(r) == a.letter {
					revealed[tileKey{wi, i}] = true
				}
			}
		}
	}
	return revealed
}

func (h *Games) state(g *game) (gameState, error) {
	attempts, err := h.attempts(g)
	if err != nil {
		return gameState{}, err
	}
	s := gameState{
		ID:        g.id,
		Status:    g.status,
		Direction: g.direction,
		Pairs:     []pair{},
		Guessed:   []string{},
		Results:   []bool{},
		Wrong:     []string{},
		UsedUp:    []string{},
		MaxMisses: MaxMisses,
	}
	revealed := revealedTiles(g, attempts)
	for _, a := range attempts {
		s.Guessed = append(s.Guessed, a.letter)
		s.Results = append(s.Results, g.correct(a))
		if !g.correct(a) {
			s.Wrong = append(s.Wrong, a.letter)
		}
	}
	counts := map[string]int{} // occurrences of each letter in the round
	found := map[string]int{}  // revealed occurrences of each letter
	for wi, w := range g.words {
		e := g.answer(w)
		out := make([]string, len(e))
		for i, r := range e {
			counts[string(r)]++
			if revealed[tileKey{wi, i}] {
				found[string(r)]++
			}
			// a finished round shows everything
			if revealed[tileKey{wi, i}] || g.status != "playing" {
				out[i] = string(r)
			}
		}
		s.Pairs = append(s.Pairs, pair{Prompt: g.clue(w), Tiles: out})
	}
	for l, c := range counts {
		if found[l] == c {
			s.UsedUp = append(s.UsedUp, l)
		}
	}
	sort.Strings(s.UsedUp)
	return s, nil
}

func (h *Games) writeState(w http.ResponseWriter, code int, g *game) {
	s, err := h.state(g)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(s)
}

// Me returns the signed-in user's name and how many distinct words they have
// learned — a word counts once the player has retrieved it themselves, so the
// ones a round revealed for free through another word's letters do not inflate
// the tally.
func (h *Games) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.Username(r)
	authed := middleware.Authenticated(r)
	var learned int
	err := h.DB.QueryRow(
		"SELECT COUNT(*) FROM word_reviews WHERE username = $1 AND streak > 0", user).Scan(&learned)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	// the masthead's issue number is this player's own tally of rounds dealt,
	// counting up as they play (separate per account and per guest browser)
	var plays int
	if err := h.DB.QueryRow("SELECT COUNT(*) FROM games WHERE username = $1", user).Scan(&plays); err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	// a GitHub-style activity calendar: how many words the player genuinely
	// retrieved on each of the recent days. The window starts on a Sunday so the
	// client can chunk the dense array into weekday-aligned columns; days with no
	// study come back as 0. daysBack covers 13 weeks plus the current partial week.
	start, activity, err := h.activityCalendar(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	// guest players play anonymously; only signed-in accounts show a name and
	// a persisted vocabulary count in the UI
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"username":      user,
		"learned":       learned,
		"guest":         !authed,
		"plays":         plays,
		"activity":      activity,
		"activityStart": start.Format("2006-01-02"),
	})
}

// Current returns the player's latest round, dealing a fresh one only when they
// have never played. Refreshing resumes an in-progress round with the same
// words rather than dealing a new one, so revisiting neither changes the board
// nor inflates the global play tally. A finished round stays visible (with its
// results) until the player starts a new one.
func (h *Games) Current(w http.ResponseWriter, r *http.Request) {
	user := middleware.Username(r)
	g, err := h.latest(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if g == nil {
		if g, err = h.create(user, "it"); err != nil {
			writeError(w, http.StatusInternalServerError, "could not start a game")
			return
		}
	}
	h.writeState(w, http.StatusOK, g)
}

// New starts a fresh round once the current one is finished.
func (h *Games) New(w http.ResponseWriter, r *http.Request) {
	user := middleware.Username(r)
	g, err := h.latest(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if g != nil && g.status == "playing" {
		writeError(w, http.StatusConflict, "finish the current game first")
		return
	}
	// carry the player's chosen direction into the next round
	dir := "it"
	if g != nil {
		dir = g.direction
	}
	g, err = h.create(user, dir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not start a game")
		return
	}
	h.writeState(w, http.StatusCreated, g)
}

// Reset wipes the in-progress round's guesses so it starts over with the
// same five words.
func (h *Games) Reset(w http.ResponseWriter, r *http.Request) {
	user := middleware.Username(r)
	g, err := h.latest(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if g == nil || g.status != "playing" {
		writeError(w, http.StatusConflict, "no game in progress")
		return
	}
	if _, err := h.DB.Exec("DELETE FROM guesses WHERE game_id = $1", g.id); err != nil {
		writeError(w, http.StatusInternalServerError, "could not reset the game")
		return
	}
	h.writeState(w, http.StatusOK, g)
}

// SetDirection flips the current round between guessing the English word and
// guessing the Italian one. It is allowed only before the player has placed any
// letter — the UI disables the language flags once a round is underway. Changing
// direction also deals a fresh set of words, reusing the untouched round's row so
// the play tally is not inflated.
func (h *Games) SetDirection(w http.ResponseWriter, r *http.Request) {
	user := middleware.Username(r)
	var body struct {
		Direction string `json:"direction"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	dir := normalizeDirection(body.Direction)

	g, err := h.latest(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if g == nil || g.status != "playing" {
		writeError(w, http.StatusConflict, "no game in progress")
		return
	}
	attempts, err := h.attempts(g)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if len(attempts) > 0 {
		writeError(w, http.StatusConflict, "round already started")
		return
	}
	if g.direction != dir {
		picked, err := h.nextWords(user, dir)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		if _, err := h.DB.Exec(
			"UPDATE games SET word = $1, direction = $2 WHERE id = $3",
			strings.Join(picked, ","), dir, g.id); err != nil {
			writeError(w, http.StatusInternalServerError, "could not switch direction")
			return
		}
		g.words = picked
		g.direction = dir
	}
	h.writeState(w, http.StatusOK, g)
}

// Guess places one letter on one tile of the current round.
func (h *Games) Guess(w http.ResponseWriter, r *http.Request) {
	user := middleware.Username(r)
	var body struct {
		Guess    string `json:"guess"`
		Word     int    `json:"word"`     // 0-based pair index
		Position int    `json:"position"` // 0-based tile index within the word
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	letter := strings.ToUpper(strings.TrimSpace(body.Guess))
	if len(letter) != 1 || letter[0] < 'A' || letter[0] > 'Z' {
		writeError(w, http.StatusBadRequest, "guess one letter (A-Z)")
		return
	}

	g, err := h.latest(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if g == nil || g.status != "playing" {
		writeError(w, http.StatusConflict, "no game in progress")
		return
	}
	if body.Word < 0 || body.Word >= len(g.words) ||
		body.Position < 0 || body.Position >= len(g.answer(g.words[body.Word])) {
		writeError(w, http.StatusBadRequest, "no such tile")
		return
	}
	a := attempt{letter: letter, word: body.Word, pos: body.Position}

	attempts, err := h.attempts(g)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	revealed := revealedTiles(g, attempts)
	if revealed[tileKey{a.word, a.pos}] {
		writeError(w, http.StatusBadRequest, "tile already revealed")
		return
	}
	wrong := 0
	for _, prev := range attempts {
		if !g.correct(prev) {
			wrong++
		}
		if prev == a {
			writeError(w, http.StatusBadRequest, "already tried that letter there")
			return
		}
	}

	if _, err := h.DB.Exec(
		"INSERT INTO guesses (game_id, guess) VALUES ($1, $2)", g.id, a.encode()); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save guess")
		return
	}

	if g.correct(a) {
		revealed = revealedTiles(g, append(attempts, a))
	} else {
		wrong++
	}
	total := 0
	for _, w := range g.words {
		total += len(g.answer(w))
	}
	switch {
	case wrong >= MaxMisses:
		g.status = "lost" // the fifth miss ends the round
	case len(revealed) == total:
		g.status = "won"
	}
	if g.status != "playing" {
		if _, err := h.DB.Exec("UPDATE games SET status = $1 WHERE id = $2", g.status, g.id); err != nil {
			writeError(w, http.StatusInternalServerError, "could not update game")
			return
		}
		// schedule each word of the round on its own, by what the player
		// actually retrieved rather than by how the round as a whole ended
		if err := h.recordReviews(user, g, append(attempts, a)); err != nil {
			writeError(w, http.StatusInternalServerError, "could not record the round")
			return
		}
	}
	h.writeState(w, http.StatusOK, g)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

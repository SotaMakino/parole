package handlers

import (
	"database/sql"
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"strings"

	"example.com/parole/middleware"
)

const (
	WordsPerRound = 5
	// misses are unlimited, but finishing with more than this many flags the
	// round's words for review ("lost")
	ReviewMisses = 5
	// a flagged round's words return once this many later rounds have been played
	ReviewGap = 3
)

type Games struct {
	DB *sql.DB
}

// game rows store the round's Italian words comma-joined in the word column,
// so the schema is the same as a single-word game.
type game struct {
	id     int64
	words  []string
	status string // playing | won | lost
}

type pair struct {
	Italian string   `json:"italian"`
	English []string `json:"english"` // one entry per letter: revealed letter or ""
}

type gameState struct {
	ID      int64    `json:"id"`
	Status  string   `json:"status"` // "lost" = completed, flagged for review
	Pairs   []pair   `json:"pairs"`
	Guessed []string `json:"guessed"` // every letter tried, in order
	Wrong   []string `json:"wrong"`   // the tried letters that hit nothing
}

func (h *Games) latest(user string) (*game, error) {
	g := &game{}
	var joined string
	err := h.DB.QueryRow(
		"SELECT id, word, status FROM games WHERE username = $1 ORDER BY id DESC LIMIT 1",
		user).Scan(&g.id, &joined, &g.status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	g.words = strings.Split(joined, ",")
	return g, err
}

// history replays a user's finished rounds: every word of a round shares the
// round's outcome, and later rounds overwrite earlier ones.
type outcome struct {
	round int // 1-based index of the user's finished rounds
	won   bool
}

func (h *Games) history(user string) (map[string]outcome, int, error) {
	rows, err := h.DB.Query(
		"SELECT word, status FROM games WHERE username = $1 AND status <> 'playing' ORDER BY id",
		user)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	last := map[string]outcome{}
	round := 0
	for rows.Next() {
		var joined, status string
		if err := rows.Scan(&joined, &status); err != nil {
			return nil, 0, err
		}
		round++
		for _, w := range strings.Split(joined, ",") {
			last[w] = outcome{round, status == "won"}
		}
	}
	return last, round, rows.Err()
}

// nextWords picks a round's words. Priority:
//  1. spaced repetition — words lost at least ReviewGap finished rounds ago
//     and not won since (oldest miss first)
//  2. words the user has never played, drawn at random
//  3. not-yet-won words played longest ago (losses not due yet)
//  4. everything is won: recycle the words won longest ago
func (h *Games) nextWords(user string) ([]string, error) {
	last, round, err := h.history(user)
	if err != nil {
		return nil, err
	}

	picked := []string{}
	taken := map[string]bool{}
	take := func(w string) {
		if !taken[w] && len(picked) < WordsPerRound {
			taken[w] = true
			picked = append(picked, w)
		}
	}
	// repeatedly take the oldest-round word matching, until none match
	takeOldest := func(match func(outcome) bool) {
		for len(picked) < WordsPerRound {
			best, bestRound := "", round+1
			for _, v := range words {
				o, seen := last[v.Italian]
				if seen && !taken[v.Italian] && match(o) && o.round < bestRound {
					best, bestRound = v.Italian, o.round
				}
			}
			if best == "" {
				return
			}
			take(best)
		}
	}

	takeOldest(func(o outcome) bool { return !o.won && round-o.round >= ReviewGap })
	unseen := []string{}
	for _, v := range words {
		if _, seen := last[v.Italian]; !seen {
			unseen = append(unseen, v.Italian)
		}
	}
	rand.Shuffle(len(unseen), func(i, j int) { unseen[i], unseen[j] = unseen[j], unseen[i] })
	for _, w := range unseen {
		take(w)
	}
	takeOldest(func(o outcome) bool { return !o.won })
	takeOldest(func(o outcome) bool { return true })
	return picked, nil
}

func (h *Games) create(user string) (*game, error) {
	picked, err := h.nextWords(user)
	if err != nil {
		return nil, err
	}
	g := &game{words: picked, status: "playing"}
	err = h.DB.QueryRow(
		"INSERT INTO games (username, word, status) VALUES ($1, $2, 'playing') RETURNING id",
		user, strings.Join(picked, ",")).Scan(&g.id)
	return g, err
}

func (h *Games) guessed(g *game) ([]string, error) {
	rows, err := h.DB.Query("SELECT guess FROM guesses WHERE game_id = $1 ORDER BY id", g.id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	letters := []string{}
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return nil, err
		}
		letters = append(letters, l)
	}
	return letters, rows.Err()
}

// inAnyWord reports whether the letter occurs in any of the round's
// English translations.
func (g *game) inAnyWord(letter string) bool {
	for _, w := range g.words {
		if strings.Contains(english[w], letter) {
			return true
		}
	}
	return false
}

// solved reports whether every letter of every English word has been guessed.
func (g *game) solved(guessedSet map[string]bool) bool {
	for _, w := range g.words {
		for _, r := range english[w] {
			if !guessedSet[string(r)] {
				return false
			}
		}
	}
	return true
}

func (h *Games) state(g *game) (gameState, error) {
	letters, err := h.guessed(g)
	if err != nil {
		return gameState{}, err
	}
	guessedSet := map[string]bool{}
	for _, l := range letters {
		guessedSet[l] = true
	}
	s := gameState{
		ID:      g.id,
		Status:  g.status,
		Pairs:   []pair{},
		Guessed: letters,
		Wrong:   []string{},
	}
	for _, l := range letters {
		if !g.inAnyWord(l) {
			s.Wrong = append(s.Wrong, l)
		}
	}
	for _, w := range g.words {
		e := english[w]
		revealed := make([]string, len(e))
		for i, r := range e {
			// a finished round shows everything
			if guessedSet[string(r)] || g.status != "playing" {
				revealed[i] = string(r)
			}
		}
		s.Pairs = append(s.Pairs, pair{Italian: w, English: revealed})
	}
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

// Current returns the user's latest round, starting one if they have none
// yet. Revisiting mid-round starts it over: an in-progress round's guesses
// are wiped so the page always loads a clean board.
func (h *Games) Current(w http.ResponseWriter, r *http.Request) {
	user := middleware.Username(r)
	g, err := h.latest(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	switch {
	case g == nil:
		if g, err = h.create(user); err != nil {
			writeError(w, http.StatusInternalServerError, "could not start a game")
			return
		}
	case g.status == "playing":
		if _, err := h.DB.Exec("DELETE FROM guesses WHERE game_id = $1", g.id); err != nil {
			writeError(w, http.StatusInternalServerError, "could not reset the game")
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
	g, err = h.create(user)
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

// Retry restarts the just-finished round with the same five words.
func (h *Games) Retry(w http.ResponseWriter, r *http.Request) {
	user := middleware.Username(r)
	g, err := h.latest(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if g == nil || g.status == "playing" {
		writeError(w, http.StatusConflict, "no finished game to retry")
		return
	}
	fresh := &game{words: g.words, status: "playing"}
	if err := h.DB.QueryRow(
		"INSERT INTO games (username, word, status) VALUES ($1, $2, 'playing') RETURNING id",
		user, strings.Join(g.words, ",")).Scan(&fresh.id); err != nil {
		writeError(w, http.StatusInternalServerError, "could not start a game")
		return
	}
	h.writeState(w, http.StatusCreated, fresh)
}

// Guess submits one letter for the current round.
func (h *Games) Guess(w http.ResponseWriter, r *http.Request) {
	user := middleware.Username(r)
	var body struct {
		Guess string `json:"guess"`
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

	letters, err := h.guessed(g)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	guessedSet := map[string]bool{}
	wrong := 0
	for _, l := range letters {
		guessedSet[l] = true
		if !g.inAnyWord(l) {
			wrong++
		}
	}
	if guessedSet[letter] {
		writeError(w, http.StatusBadRequest, "letter already tried")
		return
	}

	if _, err := h.DB.Exec(
		"INSERT INTO guesses (game_id, guess) VALUES ($1, $2)", g.id, letter); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save guess")
		return
	}
	guessedSet[letter] = true

	if !g.inAnyWord(letter) {
		wrong++
	}
	// misses never end the round; the outcome is decided once everything is
	// revealed — too many misses flags the words for review
	if g.solved(guessedSet) {
		if wrong > ReviewMisses {
			g.status = "lost"
		} else {
			g.status = "won"
		}
	}
	if g.status != "playing" {
		if _, err := h.DB.Exec("UPDATE games SET status = $1 WHERE id = $2", g.status, g.id); err != nil {
			writeError(w, http.StatusInternalServerError, "could not update game")
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

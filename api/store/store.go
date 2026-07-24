package store

import (
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Open(url string) (*sql.DB, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS accounts (
		username TEXT PRIMARY KEY,
		password_hash TEXT NOT NULL
	)`)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		expires_at TIMESTAMPTZ NOT NULL
	)`)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS games (
		id BIGSERIAL PRIMARY KEY,
		username TEXT NOT NULL,
		word TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'playing'
	)`)
	// "it" spells the English word (default); "en" spells the Italian one
	_, err = db.Exec(`ALTER TABLE games ADD COLUMN IF NOT EXISTS direction TEXT NOT NULL DEFAULT 'it'`)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS guesses (
		id BIGSERIAL PRIMARY KEY,
		game_id BIGINT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
		guess TEXT NOT NULL
	)`)
	// one row per word the player has met, so review scheduling is per word and
	// measured in days rather than in rounds played
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS word_reviews (
		username TEXT NOT NULL,
		word TEXT NOT NULL,
		due_at TIMESTAMPTZ NOT NULL,
		last_seen TIMESTAMPTZ NOT NULL,
		streak INT NOT NULL DEFAULT 0,
		PRIMARY KEY (username, word)
	)`)
	// "learned" is sticky: once the player retrieves a word themselves it stays
	// true forever, so the vocabulary tally only grows. streak, by contrast,
	// resets to 0 when a word is later revealed for free — that governs review
	// scheduling, not whether the word has ever been learned.
	_, err = db.Exec(`ALTER TABLE word_reviews ADD COLUMN IF NOT EXISTS learned BOOLEAN NOT NULL DEFAULT false`)
	// backfill: any word currently on a positive streak has been retrieved at
	// least once. Sets true only; never unsets, so it is safe to run each start.
	_, err = db.Exec(`UPDATE word_reviews SET learned = true WHERE streak > 0 AND NOT learned`)
	// one row per day the player studied, holding how many words they genuinely
	// retrieved that day — the source for the GitHub-style activity calendar
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS study_days (
		username TEXT NOT NULL,
		day DATE NOT NULL,
		count INT NOT NULL DEFAULT 0,
		PRIMARY KEY (username, day)
	)`)
	// Rounds played before word_reviews existed only recorded a win or a loss
	// per round. Seed those wins so returning players keep their vocabulary
	// count and their words re-enter the review rotation instead of being dealt
	// as if never met. ON CONFLICT keeps this idempotent: a word that already
	// has a record is left on whatever rung it has reached.
	_, err = db.Exec(`INSERT INTO word_reviews (username, word, due_at, last_seen, streak, learned)
		SELECT username, w, now(), now(), 1, true FROM (
			SELECT username, unnest(string_to_array(word, ',')) AS w
			FROM games WHERE status = 'won'
		) t
		ON CONFLICT (username, word) DO NOTHING`)
	return db, err
}

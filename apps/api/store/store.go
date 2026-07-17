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
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS todos (
		id BIGSERIAL PRIMARY KEY,
		username TEXT NOT NULL,
		title TEXT NOT NULL,
		UNIQUE (username, title)
	)`)
	return db, err
}

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"example.com/le-cinque/handlers"
	"example.com/le-cinque/middleware"
	"example.com/le-cinque/store"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	port := env("PORT", "8080")
	dbURL := env("DATABASE_URL", "postgres://localhost:5432/hellodb")

	db, err := store.Open(dbURL)
	if err != nil {
		log.Fatal(err)
	}

	h := &handlers.Games{DB: db}
	a := &handlers.Auth{DB: db}
	// GOOGLE_TTS_CREDENTIALS holds a service-account key JSON. If unset or bad,
	// TTS stays disabled and the frontend falls back to browser speech, so the
	// game still runs.
	t, err := handlers.NewTTS(context.Background(), env("GOOGLE_TTS_CREDENTIALS", ""))
	if err != nil {
		log.Printf("TTS disabled: %v", err)
		t = &handlers.TTS{}
	}
	mux := http.NewServeMux()

	// public
	mux.HandleFunc("POST /signup", a.Signup)
	mux.HandleFunc("POST /login", a.Login)
	mux.HandleFunc("POST /logout", a.Logout)

	// no login required — an anonymous "player" cookie identifies each browser
	game := http.NewServeMux()
	game.HandleFunc("GET /game", h.Current)
	game.HandleFunc("POST /game", h.New)
	game.HandleFunc("POST /game/retry", h.Retry)
	game.HandleFunc("POST /game/reset", h.Reset)
	game.HandleFunc("POST /game/direction", h.SetDirection)
	game.HandleFunc("POST /game/guess", h.Guess)
	mux.Handle("/game", middleware.Player(db, game))
	mux.Handle("/game/", middleware.Player(db, game))
	mux.Handle("GET /me", middleware.Player(db, http.HandlerFunc(h.Me)))
	// deleting an account needs a real session, not just a guest cookie
	mux.Handle("DELETE /me", middleware.Auth(db, http.HandlerFunc(a.DeleteAccount)))
	// Cloud TTS is for signed-in accounts only; guests fall back to the browser's
	// built-in speechSynthesis client-side. Gating it keeps synthesis cost tied to
	// real accounts rather than anonymous traffic.
	mux.Handle("GET /tts", middleware.Auth(db, http.HandlerFunc(t.Speak))) // Italian word pronunciation (Google Cloud TTS)

	allowedOrigins := strings.Split(env("ALLOWED_ORIGIN", "http://localhost:5173"), ",")
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: middleware.Logging(middleware.CORS(allowedOrigins, mux)),
	}

	go func() {
		log.Printf("listening on :%s", port)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt) // catch Ctrl+C
	<-stop                            // block here until it happens

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx) // finish active requests, refuse new ones
	db.Close()
}

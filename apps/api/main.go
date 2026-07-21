package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"example.com/parole/handlers"
	"example.com/parole/middleware"
	"example.com/parole/store"
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
	mux := http.NewServeMux()

	// public
	mux.HandleFunc("POST /signup", a.Signup)
	mux.HandleFunc("POST /login", a.Login)
	mux.HandleFunc("POST /logout", a.Logout)

	// protected — wrap a sub-mux with Auth
	game := http.NewServeMux()
	game.HandleFunc("GET /game", h.Current)
	game.HandleFunc("POST /game", h.New)
	game.HandleFunc("POST /game/retry", h.Retry)
	game.HandleFunc("POST /game/reset", h.Reset)
	game.HandleFunc("POST /game/guess", h.Guess)
	mux.Handle("/game", middleware.Auth(db, game))
	mux.Handle("/game/", middleware.Auth(db, game))
	mux.Handle("/me", middleware.Auth(db, http.HandlerFunc(h.Me)))

	allowedOrigin := env("ALLOWED_ORIGIN", "http://localhost:5173")
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: middleware.Logging(middleware.CORS(allowedOrigin, mux)),
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

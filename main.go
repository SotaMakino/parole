package main

import (
	"log"
	"net/http"

	"example.com/hello-go/handlers"
	"example.com/hello-go/middleware"
	"example.com/hello-go/store"
)

func main() {
	db, err := store.Open("app.db")
	if err != nil {
		log.Fatal(err)
	}

	h := &handlers.Users{DB: db}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /users", h.List)
	mux.HandleFunc("GET /users/{id}", h.Get)
	mux.HandleFunc("POST /users", h.Create)
	mux.HandleFunc("PUT /users/{id}", h.Update)

	handler := middleware.Logging(middleware.CORS(mux))
	log.Fatal(http.ListenAndServe(":8080", handler))
}

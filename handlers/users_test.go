package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/hello-go/store"
)

func setup(t *testing.T) *Users {
	db, err := store.Open(":memory:") // SQLite in RAM — fresh DB per test!
	if err != nil {
		t.Fatal(err)
	}
	return &Users{DB: db}
}

func TestCreateUser(t *testing.T) {
	h := setup(t)

	req := httptest.NewRequest("POST", "/users",
		strings.NewReader(`{"id":"1","name":"Ann"}`))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
}

func TestCreateUser_EmptyName(t *testing.T) {
	h := setup(t)

	req := httptest.NewRequest("POST", "/users",
		strings.NewReader(`{"id":"1","name":""}`))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

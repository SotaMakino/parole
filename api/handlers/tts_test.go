package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// These tests cover the request-validation and caching paths of tts.go — none
// of them reach Google, so they always run. The actual synthesis call is left
// to a live key.

// configuredTTS is a TTS with a non-nil client so validation and cache paths
// run — none of these tests actually reach Google (they return before any call).
func configuredTTS() *TTS { return &TTS{client: &http.Client{}} }

func TestTTS_UnconfiguredReturns503(t *testing.T) {
	tts := &TTS{} // nil client
	rr := httptest.NewRecorder()
	tts.Speak(rr, httptest.NewRequest(http.MethodGet, "/tts?word=cane", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when unconfigured, got %d", rr.Code)
	}
}

func TestTTS_RejectsBadInput(t *testing.T) {
	tts := configuredTTS() // so validation runs before any call
	cases := []struct {
		name, query string
		want        int
	}{
		{"empty word", "word=", http.StatusBadRequest},
		{"digits", "word=cane123", http.StatusBadRequest},
		{"too long", "word=" + strings.Repeat("a", 65), http.StatusBadRequest},
		{"unsupported lang", "word=cane&lang=de-DE", http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			tts.Speak(rr, httptest.NewRequest(http.MethodGet, "/tts?"+c.query, nil))
			if rr.Code != c.want {
				t.Errorf("query %q: expected %d, got %d", c.query, c.want, rr.Code)
			}
		})
	}
}

func TestTTS_ServesFromCache(t *testing.T) {
	// A cache hit is served without an API call, so an empty key still returns
	// the stored MP3 rather than the 503 an uncached request would give.
	tts := configuredTTS()
	tts.store("it-IT|cane", []byte("fake-mp3"))

	rr := httptest.NewRecorder()
	tts.Speak(rr, httptest.NewRequest(http.MethodGet, "/tts?word=cane&lang=it-IT", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from cache, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "audio/mpeg" {
		t.Errorf("expected audio/mpeg, got %q", ct)
	}
	if rr.Body.String() != "fake-mp3" {
		t.Errorf("expected cached bytes, got %q", rr.Body.String())
	}
}

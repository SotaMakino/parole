package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// TTS turns curriculum words into natural speech via Google Cloud
// Text-to-Speech. Browser speechSynthesis stays good enough (and free) for
// English, but Italian sounds robotic in browsers — Firefox only exposes the
// low-quality "compact" voice and can't reach the enhanced one — so Italian is
// served here instead. Synthesised MP3s are cached in memory: the vocabulary
// is small and fixed, so each word is generated at most once per process, well
// inside Google's free tier.
type TTS struct {
	// client carries the service-account OAuth token on every request; nil when
	// no credentials are configured, in which case Speak returns 503 and the
	// frontend falls back to browser speech.
	client *http.Client

	mu    sync.RWMutex
	cache map[string][]byte
}

// NewTTS builds a TTS from a Google service-account key (the JSON file's
// contents, e.g. from an env var). An empty string yields an unconfigured TTS
// that always 503s — handy for local dev without a key.
func NewTTS(ctx context.Context, credentialsJSON string) (*TTS, error) {
	if strings.TrimSpace(credentialsJSON) == "" {
		return &TTS{}, nil
	}
	creds, err := google.CredentialsFromJSON(
		ctx, []byte(credentialsJSON), "https://www.googleapis.com/auth/cloud-platform",
	)
	if err != nil {
		return nil, fmt.Errorf("parse TTS credentials: %w", err)
	}
	// oauth2's client refreshes and caches the access token automatically
	return &TTS{client: oauth2.NewClient(ctx, creds.TokenSource)}, nil
}

const ttsEndpoint = "https://texttospeech.googleapis.com/v1/text:synthesize"

// the natural Neural2 voice used per supported language
var ttsVoices = map[string]string{
	"it-IT": "it-IT-Neural2-C", // male
	"en-US": "en-US-Neural2-C",
}

// ttsSpeakingRate matches the old browser rate: a little slow so each syllable
// is easy to catch. Baked into the audio, so playback stays at normal speed.
const ttsSpeakingRate = 0.8

// wordRE guards the endpoint: curriculum words are short and letters-only, so
// anything else is rejected before it can reach (and bill) Google.
var wordRE = regexp.MustCompile(`^[a-z' -]{1,64}$`)

// Speak returns the MP3 pronunciation of ?word= in ?lang= (default it-IT).
func (t *TTS) Speak(w http.ResponseWriter, r *http.Request) {
	if t.client == nil {
		// the frontend falls back to browser speech on any error response
		writeError(w, http.StatusServiceUnavailable, "TTS is not configured")
		return
	}

	word := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("word")))
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "it-IT"
	}
	voice, ok := ttsVoices[lang]
	if !ok {
		writeError(w, http.StatusBadRequest, "unsupported lang")
		return
	}
	if !wordRE.MatchString(word) {
		writeError(w, http.StatusBadRequest, "invalid word")
		return
	}

	key := lang + "|" + word
	if mp3 := t.cached(key); mp3 != nil {
		writeMP3(w, mp3)
		return
	}

	mp3, err := t.synthesize(r.Context(), word, lang, voice)
	if err != nil {
		log.Printf("tts: %v", err)
		writeError(w, http.StatusBadGateway, "speech synthesis failed")
		return
	}
	t.store(key, mp3)
	writeMP3(w, mp3)
}

func (t *TTS) cached(key string) []byte {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.cache[key]
}

func (t *TTS) store(key string, mp3 []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cache == nil {
		t.cache = make(map[string][]byte)
	}
	t.cache[key] = mp3
}

// Google's REST request/response shapes (only the fields we use).
type ttsRequest struct {
	Input struct {
		Text string `json:"text"`
	} `json:"input"`
	Voice struct {
		LanguageCode string `json:"languageCode"`
		Name         string `json:"name"`
	} `json:"voice"`
	AudioConfig struct {
		AudioEncoding string  `json:"audioEncoding"`
		SpeakingRate  float64 `json:"speakingRate"`
	} `json:"audioConfig"`
}

type ttsResponse struct {
	AudioContent string `json:"audioContent"` // base64 MP3
}

func (t *TTS) synthesize(ctx context.Context, word, lang, voice string) ([]byte, error) {
	var reqBody ttsRequest
	reqBody.Input.Text = word
	reqBody.Voice.LanguageCode = lang
	reqBody.Voice.Name = voice
	reqBody.AudioConfig.AudioEncoding = "MP3"
	reqBody.AudioConfig.SpeakingRate = ttsSpeakingRate

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ttsEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("google tts %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var out ttsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	mp3, err := base64.StdEncoding.DecodeString(out.AudioContent)
	if err != nil {
		return nil, err
	}
	if len(mp3) == 0 {
		return nil, fmt.Errorf("google tts returned empty audio")
	}
	return mp3, nil
}

func writeMP3(w http.ResponseWriter, mp3 []byte) {
	w.Header().Set("Content-Type", "audio/mpeg")
	// the audio for a word never changes, so let the browser/CDN cache it hard
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Length", strconv.Itoa(len(mp3)))
	w.Write(mp3)
}

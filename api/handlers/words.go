package handlers

import (
	_ "embed"
	"strings"
)

// vocab is one curriculum entry: an essential Italian word and the English
// word with the same meaning. Both uppercase A–Z; lengths vary freely.
type vocab struct {
	Italian string
	English string
}

// wordsTSV is the curriculum, kept as data rather than Go literals: one
// "ITALIAN<tab>ENGLISH" pair per line, grouped by "#" theme comments. It holds
// 1500+ essential beginner words. Accented words (caffè, città, lunedì, …) are
// excluded so the game stays on plain A–Z.
//
//go:embed words.tsv
var wordsTSV string

// words is the parsed curriculum. Rounds draw five at random (review words
// first — see nextWords).
var words = parseVocab(wordsTSV)

// parseVocab reads the embedded TSV, skipping blank lines and "#" comments.
func parseVocab(data string) []vocab {
	lines := strings.Split(data, "\n")
	out := make([]vocab, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		it, en, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		out = append(out, vocab{Italian: strings.TrimSpace(it), English: strings.TrimSpace(en)})
	}
	return out
}

// english maps an Italian curriculum word to its translation.
var english = func() map[string]string {
	m := make(map[string]string, len(words))
	for _, v := range words {
		m[v.Italian] = v.English
	}
	return m
}()

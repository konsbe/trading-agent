package momentumapi

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// CorrelationText is shared/content/correlation_sentences.json: each aligned /
// divergent sentence fundamental analysis stores (its format string, as in
// internal/fundamental/analyze.go) and the text shown for it. The stored
// sentences state implications the comparison behind them does not test
// ("cascade to insolvency risk", "compounding machine"); the shown text says
// which comparison was met. analyst-bot reads the same file for Discord, so
// neither surface keeps its own copy.
//
// A number the stored sentence carries (%.1f) is copied into the shown text as
// rendered (%s, in the same order). TestCorrelationSentencesCoverAnalyzer fails
// when analyze.go gains, loses or edits a sentence the file does not match.
type CorrelationText struct {
	sentences []correlationSentence
	rules     []sentenceRule
}

type correlationSentence struct {
	Stored  string `json:"stored"`
	Display string `json:"display"`
}

type sentenceRule struct {
	match   *regexp.Regexp
	display string
}

// LoadCorrelationText reads the shared sentences. Like the caveats there is no
// fallback copy: a missing or empty file stops momentum-api at start.
func LoadCorrelationText(path string) (CorrelationText, error) {
	var t CorrelationText
	b, err := os.ReadFile(path)
	if err != nil {
		return t, fmt.Errorf("correlation sentences: %w", err)
	}
	var f struct {
		Sentences []correlationSentence `json:"sentences"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return t, fmt.Errorf("correlation sentences %s: %w", path, err)
	}
	if len(f.Sentences) == 0 {
		return t, fmt.Errorf("correlation sentences %s: no sentences", path)
	}
	for _, s := range f.Sentences {
		if s.Stored == "" || s.Display == "" {
			return t, fmt.Errorf("correlation sentences %s: an entry has an empty stored or display text", path)
		}
		// %.1f captures the rendered number, %% is a literal percent sign.
		pattern := regexp.QuoteMeta(s.Stored)
		pattern = strings.ReplaceAll(pattern, `%\.1f`, `(-?[0-9]+(?:\.[0-9]+)?)`)
		pattern = strings.ReplaceAll(pattern, `%%`, `%`)
		t.rules = append(t.rules, sentenceRule{match: regexp.MustCompile(`^` + pattern + `$`), display: s.Display})
	}
	t.sentences = f.Sentences
	return t, nil
}

// Display is the shown text for one stored sentence. A sentence the file does
// not know is served as stored, so nothing is dropped.
func (t CorrelationText) Display(stored string) string {
	for _, r := range t.rules {
		m := r.match.FindStringSubmatch(stored)
		if m == nil {
			continue
		}
		args := make([]any, len(m)-1)
		for i, v := range m[1:] {
			args[i] = v
		}
		return fmt.Sprintf(r.display, args...)
	}
	return stored
}

// Displays maps Display over a list.
func (t CorrelationText) Displays(stored []string) []string {
	out := make([]string, len(stored))
	for i, s := range stored {
		out[i] = t.Display(s)
	}
	return out
}

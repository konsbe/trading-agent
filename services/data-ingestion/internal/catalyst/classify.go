// Package catalyst implements §3.11's keyword classifier for company news.
//
// §3.11 is explicit that Phase 1 uses a KEYWORD CLASSIFIER, not an LLM: "The
// point of Phase 1 is to discover which keywords actually correlate with
// runners; paying for LLM classification before knowing that is premature."
//
// Two design rules follow from that, and both are load-bearing rather than
// stylistic:
//
//  1. Keyword lists live in CONFIGURATION, not code, so they can be tuned
//     without a rebuild. A classifier whose vocabulary requires a deploy to
//     change cannot be iterated on at the speed this phase needs.
//
//  2. EVERY match is retained, not just the winning tier. §3.11: "Phase 2's most
//     valuable analysis is which specific keywords preceded real runners, and
//     that requires the raw matches retained." Storing only the tier throws away
//     the only data that can answer the question the phase exists to ask.
package catalyst

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"
)

// Tier is §3.11's classification.
type Tier string

const (
	TierA    Tier = "A"
	TierB    Tier = "B"
	TierNone Tier = "none"
)

// Rank orders tiers so the highest match wins. A > B > none.
func (t Tier) Rank() int {
	switch t {
	case TierA:
		return 2
	case TierB:
		return 1
	default:
		return 0
	}
}

// Config holds the tunable keyword vocabulary.
type Config struct {
	// TierA and TierB are phrase lists, matched case-insensitively on word
	// boundaries. Phrases may contain spaces.
	TierA []string `json:"tier_a"`
	TierB []string `json:"tier_b"`

	// BOnAnyNews decides whether a headline matching NO keyword still counts as
	// tier B.
	//
	// §3.11's tier-B row ends with "any other company news", so the default is
	// true: the mere existence of coverage in the window is the weak signal. Made
	// configurable because it is also the single assumption most likely to be
	// wrong — if it is, tier B collapses to "we fetched news at all", which would
	// make the 8-point award meaningless.
	BOnAnyNews bool `json:"b_on_any_news"`
}

// DefaultConfig returns §3.11's table verbatim.
//
// These are a STARTING vocabulary, not a validated one. Which of these actually
// precede runners is the open question Phase 1 exists to answer, which is why
// every individual match is stored rather than only the tier.
func DefaultConfig() Config {
	return Config{
		TierA: []string{
			// Regulatory
			"fda approval", "fda approves", "fda clearance", "fda cleared",
			"breakthrough designation", "breakthrough therapy",
			"orphan drug", "priority review", "510(k)", "ce mark",
			// Corporate action
			"acquisition", "acquire", "acquired", "merger", "merge with",
			"buyout", "takeover", "definitive agreement", "to be acquired",
			"tender offer", "going private",
			// Contracts and trials
			"contract award", "awarded a contract", "government contract",
			"defense contract", "phase 3 results", "phase iii results",
			"topline results", "primary endpoint",
			// Financial
			"earnings beat", "beats estimates", "raised guidance",
			"raises guidance", "raised outlook", "record revenue",
			// Listing
			"uplisting", "uplist", "nasdaq listing approval",
		},
		TierB: []string{
			"analyst upgrade", "upgraded to buy", "price target raised",
			"raises price target", "initiated coverage", "outperform rating",
			"partnership", "collaboration", "strategic alliance",
			"letter of intent", "memorandum of understanding",
			"product launch", "launches", "unveils", "introduces",
			"conference presentation", "to present at", "investor day",
			"appoints", "names ceo", "board of directors",
		},
		BOnAnyNews: true,
	}
}

// LoadConfig reads a JSON keyword file, falling back to DefaultConfig when the
// path is empty.
//
// A malformed or missing file is an ERROR rather than a silent fallback: the
// whole point of external configuration is that someone tuned it, and quietly
// reverting to defaults would make a typo look like a vocabulary that simply
// stopped matching.
func LoadConfig(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		return DefaultConfig(), nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("catalyst config %s: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return Config{}, fmt.Errorf("catalyst config %s: %w", path, err)
	}
	if len(c.TierA) == 0 && len(c.TierB) == 0 {
		return Config{}, fmt.Errorf("catalyst config %s: no keywords in either tier", path)
	}
	return c, nil
}

// Match is one keyword hit on one headline.
type Match struct {
	Tier    Tier
	Keyword string
}

// Classification is the result for a single headline.
type Classification struct {
	// Tier is the HIGHEST tier matched.
	Tier Tier

	// Matches lists every keyword hit, across all tiers, so Phase 2 can ask
	// which specific phrases preceded runners.
	Matches []Match

	// MatchedNothing is true when no keyword hit at all. Distinguished from
	// TierNone because with BOnAnyNews the tier can be B while nothing matched,
	// and conflating those would hide how much of tier B is "real keyword" versus
	// "any coverage exists".
	MatchedNothing bool
}

// ClassifyHeadline applies §3.11's tiers to one headline.
//
// Matching is case-insensitive and boundary-aware: a phrase only matches when it
// is not embedded inside a larger word. Without that, "launches" matches
// nothing useful in "relaunches" and "acquire" fires on "acquirer" — the sort of
// false positive that would quietly inflate tier A and make the whole
// correlation exercise measure the wrong thing.
func ClassifyHeadline(headline string, cfg Config) Classification {
	h := strings.ToLower(headline)
	var matches []Match

	for _, kw := range cfg.TierA {
		if containsPhrase(h, strings.ToLower(kw)) {
			matches = append(matches, Match{TierA, kw})
		}
	}
	for _, kw := range cfg.TierB {
		if containsPhrase(h, strings.ToLower(kw)) {
			matches = append(matches, Match{TierB, kw})
		}
	}

	c := Classification{Matches: matches, MatchedNothing: len(matches) == 0}
	c.Tier = TierNone
	for _, m := range matches {
		if m.Tier.Rank() > c.Tier.Rank() {
			c.Tier = m.Tier
		}
	}
	if c.MatchedNothing && cfg.BOnAnyNews && strings.TrimSpace(headline) != "" {
		// §3.11's tier B ends with "any other company news": the existence of
		// coverage is itself the weak signal.
		c.Tier = TierB
	}
	return c
}

// containsPhrase reports whether phrase occurs in s at word boundaries.
func containsPhrase(s, phrase string) bool {
	if phrase == "" {
		return false
	}
	from := 0
	for {
		i := strings.Index(s[from:], phrase)
		if i < 0 {
			return false
		}
		start := from + i
		end := start + len(phrase)
		if boundary(s, start-1) && boundary(s, end) {
			return true
		}
		from = start + 1
		if from >= len(s) {
			return false
		}
	}
}

// boundary reports whether position i is outside the string or a non-alphanumeric
// rune, i.e. a word edge. Digits count as word characters so "510(k)" does not
// match inside "1510(k)".
func boundary(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return true
	}
	r := rune(s[i])
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

// SymbolResult aggregates §3.11's 48-hour window for one symbol.
type SymbolResult struct {
	// Tier is the highest tier across every headline in the window — the value
	// that feeds §4.2's catalyst sub-score.
	Tier Tier

	// Headlines is how many were examined. Zero means no coverage in the window,
	// which is TierNone and is a finding rather than a gap.
	Headlines int

	// Matches is every keyword hit across every headline, retained per §3.11.
	Matches []Match

	// KeywordMatchedHeadlines counts headlines that hit at least one keyword, so
	// "tier B because a keyword matched" stays separable from "tier B because any
	// coverage exists".
	KeywordMatchedHeadlines int
}

// ClassifyWindow applies §3.11 to every headline in a symbol's window.
func ClassifyWindow(headlines []string, cfg Config) SymbolResult {
	res := SymbolResult{Tier: TierNone}
	for _, h := range headlines {
		if strings.TrimSpace(h) == "" {
			continue
		}
		res.Headlines++
		c := ClassifyHeadline(h, cfg)
		if !c.MatchedNothing {
			res.KeywordMatchedHeadlines++
		}
		res.Matches = append(res.Matches, c.Matches...)
		if c.Tier.Rank() > res.Tier.Rank() {
			res.Tier = c.Tier
		}
	}
	return res
}

// DistinctKeywords returns the matched keywords, deduplicated and sorted, for
// compact storage and reporting.
func DistinctKeywords(matches []Match) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if _, dup := seen[m.Keyword]; dup {
			continue
		}
		seen[m.Keyword] = struct{}{}
		out = append(out, m.Keyword)
	}
	sort.Strings(out)
	return out
}

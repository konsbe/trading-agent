package catalyst

import (
	"os"
	"path/filepath"
	"testing"
)

// ─── Tier precedence ──────────────────────────────────────────────────────────

// §3.11: "Classify each headline into the HIGHEST matching tier." A headline that
// hits both tiers is A, and getting that backwards would systematically
// understate the strongest catalysts — the ones the feature exists to find.
func TestClassifyHeadline_HighestTierWins(t *testing.T) {
	cfg := DefaultConfig()
	// "partnership" is tier B, "fda approval" is tier A.
	c := ClassifyHeadline("Acme announces partnership and receives FDA approval", cfg)
	if c.Tier != TierA {
		t.Errorf("Tier = %q, want A — the highest match must win", c.Tier)
	}
	// And both matches are retained, not just the winner.
	var sawA, sawB bool
	for _, m := range c.Matches {
		if m.Tier == TierA {
			sawA = true
		}
		if m.Tier == TierB {
			sawB = true
		}
	}
	if !sawA || !sawB {
		t.Errorf("matches = %+v; §3.11 requires EVERY match retained, not just the winning tier", c.Matches)
	}
}

func TestClassifyHeadline_TierAKeywords(t *testing.T) {
	cfg := DefaultConfig()
	for _, h := range []string{
		"XYZ Receives FDA Approval for Lead Candidate",
		"BigCo to acquire SmallCo in all-cash deal",
		"Company enters definitive agreement with acquirer",
		"Reports positive Phase 3 results",
		"Q3 earnings beat expectations",
		"Management raises guidance for full year",
		"Announces uplisting to Nasdaq",
	} {
		if got := ClassifyHeadline(h, cfg); got.Tier != TierA {
			t.Errorf("%q -> %q, want A (matches %+v)", h, got.Tier, got.Matches)
		}
	}
}

func TestClassifyHeadline_TierBKeywords(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BOnAnyNews = false // isolate real keyword matches from the catch-all
	for _, h := range []string{
		"Analyst upgrade lifts shares",
		"Broker raises price target to $40",
		"Announces strategic alliance with vendor",
		"Unveils next-generation product",
		"CEO to present at investor conference",
	} {
		if got := ClassifyHeadline(h, cfg); got.Tier != TierB {
			t.Errorf("%q -> %q, want B (matches %+v)", h, got.Tier, got.Matches)
		}
	}
}

// ─── Boundary matching: the false-positive guard ──────────────────────────────

// Substring matching would quietly inflate tier A, which is the worst direction
// to be wrong in: tier A is worth 15 points and the whole exercise is measuring
// which keywords precede runners. If "acquire" fires on "acquirer", the
// correlation being measured is partly noise.
func TestClassifyHeadline_DoesNotMatchInsideLargerWords(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BOnAnyNews = false

	cases := []struct {
		headline string
		why      string
	}{
		{"The acquirer declined to comment", "\"acquire\" must not match inside \"acquirer\""},
		{"Relaunches website after outage", "\"launches\" must not match inside \"relaunches\""},
		{"Discusses premerger antitrust review", "\"merger\" must not match inside \"premerger\""},
	}
	for _, c := range cases {
		got := ClassifyHeadline(c.headline, cfg)
		if !got.MatchedNothing {
			t.Errorf("%q matched %+v: %s", c.headline, got.Matches, c.why)
		}
	}

	// The converse: a real occurrence adjacent to punctuation MUST match, or the
	// boundary rule has been made too strict.
	for _, h := range []string{
		"Acme to acquire, pending approval",
		"FDA approval: what it means",
		"(acquisition) completed",
	} {
		if got := ClassifyHeadline(h, cfg); got.MatchedNothing {
			t.Errorf("%q matched nothing; punctuation is a word boundary, not a blocker", h)
		}
	}
}

// ─── The "any news is tier B" assumption ──────────────────────────────────────

// §3.11's tier-B row ends with "any other company news", so coverage existing is
// itself the weak signal. It is configurable because it is the assumption most
// likely to be wrong: if it is, tier B degenerates into "we fetched news at
// all", which would make its 8 points meaningless.
func TestClassifyHeadline_AnyNewsFallbackIsConfigurable(t *testing.T) {
	on := DefaultConfig()
	on.BOnAnyNews = true
	off := DefaultConfig()
	off.BOnAnyNews = false

	const bland = "Company files routine quarterly report"

	if got := ClassifyHeadline(bland, on); got.Tier != TierB {
		t.Errorf("with BOnAnyNews: %q -> %q, want B", bland, got.Tier)
	}
	if got := ClassifyHeadline(bland, off); got.Tier != TierNone {
		t.Errorf("without BOnAnyNews: %q -> %q, want none", bland, got.Tier)
	}

	// MatchedNothing must stay true in both cases, so "tier B by keyword" is
	// separable from "tier B by catch-all" in the stored record.
	if !ClassifyHeadline(bland, on).MatchedNothing {
		t.Error("MatchedNothing must remain true even when the catch-all assigns tier B")
	}
}

// An empty headline is not coverage, so the catch-all must not fire on it.
func TestClassifyHeadline_EmptyHeadlineIsNotCoverage(t *testing.T) {
	cfg := DefaultConfig()
	for _, h := range []string{"", "   ", "\t"} {
		if got := ClassifyHeadline(h, cfg); got.Tier != TierNone {
			t.Errorf("%q -> %q, want none", h, got.Tier)
		}
	}
}

// ─── Window aggregation ───────────────────────────────────────────────────────

func TestClassifyWindow_TakesTheHighestTierAcrossHeadlines(t *testing.T) {
	cfg := DefaultConfig()
	res := ClassifyWindow([]string{
		"Analyst upgrade on valuation",
		"Routine 8-K filing",
		"Announces acquisition of competitor",
	}, cfg)

	if res.Tier != TierA {
		t.Errorf("Tier = %q, want A", res.Tier)
	}
	if res.Headlines != 3 {
		t.Errorf("Headlines = %d, want 3", res.Headlines)
	}
	if res.KeywordMatchedHeadlines != 2 {
		t.Errorf("KeywordMatchedHeadlines = %d, want 2 (the upgrade and the acquisition)", res.KeywordMatchedHeadlines)
	}
	if len(res.Matches) < 2 {
		t.Errorf("matches = %+v, want every hit retained", res.Matches)
	}
}

// No coverage in the window is TierNone, and that is a FINDING rather than
// missing data: §3.11 defines "no company news in window" as the zero tier.
func TestClassifyWindow_NoCoverageIsTierNone(t *testing.T) {
	res := ClassifyWindow(nil, DefaultConfig())
	if res.Tier != TierNone {
		t.Errorf("Tier = %q, want none", res.Tier)
	}
	if res.Headlines != 0 {
		t.Errorf("Headlines = %d, want 0", res.Headlines)
	}
}

func TestDistinctKeywords_DeduplicatesAndSorts(t *testing.T) {
	got := DistinctKeywords([]Match{
		{TierA, "merger"}, {TierB, "partnership"}, {TierA, "merger"}, {TierA, "acquisition"},
	})
	if len(got) != 3 {
		t.Fatalf("got %v, want 3 distinct", got)
	}
	if got[0] != "acquisition" || got[1] != "merger" || got[2] != "partnership" {
		t.Errorf("got %v, want sorted", got)
	}
}

// ─── Configuration ────────────────────────────────────────────────────────────

// §3.11: "Keyword lists live in configuration ... so they can be tuned without a
// rebuild." A classifier needing a deploy to change its vocabulary cannot be
// iterated at the speed this phase requires.
func TestLoadConfig_ReadsAnExternalVocabulary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kw.json")
	if err := os.WriteFile(path, []byte(`{
      "tier_a": ["reverse merger"],
      "tier_b": ["webinar"],
      "b_on_any_news": false
    }`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := ClassifyHeadline("Completes reverse merger with shell", cfg); got.Tier != TierA {
		t.Errorf("custom tier A not applied: %q", got.Tier)
	}
	// And the built-in vocabulary must NOT leak in — the file replaces it.
	if got := ClassifyHeadline("Receives FDA approval", cfg); got.Tier == TierA {
		t.Error("default keywords leaked into a custom config; the file must replace, not extend")
	}
	if cfg.BOnAnyNews {
		t.Error("b_on_any_news from the file was ignored")
	}
}

// A missing or malformed file is an ERROR, not a silent fallback. The point of
// external config is that someone tuned it; quietly reverting to defaults would
// make a typo look like a vocabulary that just stopped matching.
func TestLoadConfig_BadFileIsAnErrorNotASilentFallback(t *testing.T) {
	if _, err := LoadConfig("/nonexistent/keywords.json"); err == nil {
		t.Error("a missing config file must error")
	}

	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte("{not json"), 0o600)
	if _, err := LoadConfig(bad); err == nil {
		t.Error("malformed JSON must error")
	}

	empty := filepath.Join(dir, "empty.json")
	os.WriteFile(empty, []byte(`{"tier_a":[],"tier_b":[]}`), 0o600)
	if _, err := LoadConfig(empty); err == nil {
		t.Error("a config with no keywords in either tier must error rather than silently classify nothing")
	}

	// An empty path is the documented "use defaults" case.
	if cfg, err := LoadConfig(""); err != nil || len(cfg.TierA) == 0 {
		t.Errorf("an empty path should yield DefaultConfig, got err=%v", err)
	}
}

func TestTierRank_OrdersAOverBOverNone(t *testing.T) {
	if !(TierA.Rank() > TierB.Rank() && TierB.Rank() > TierNone.Rank()) {
		t.Errorf("ranks out of order: A=%d B=%d none=%d", TierA.Rank(), TierB.Rank(), TierNone.Rank())
	}
}

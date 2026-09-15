package universe

import "testing"

func defaultRules() Rules { return NewRules(nil, nil, nil, false) }

func TestEvaluate_IncludesCommonStockOnAllowedVenues(t *testing.T) {
	r := defaultRules()
	cases := []struct {
		name     string
		rec      Record
		exchange string
	}{
		{"nasdaq composite", Record{Symbol: "AAPL", Type: "Common Stock", MIC: "XNAS"}, "NASDAQ"},
		{"nasdaq global select", Record{Symbol: "MSFT", Type: "Common Stock", MIC: "XNGS"}, "NASDAQ"},
		{"nasdaq capital market", Record{Symbol: "TINY", Type: "Common Stock", MIC: "XNCM"}, "NASDAQ"},
		{"nyse", Record{Symbol: "XOM", Type: "Common Stock", MIC: "XNYS"}, "NYSE"},
		{"nyse american", Record{Symbol: "UUUU", Type: "Common Stock", MIC: "XASE"}, "NYSE American"},
		{"lowercase type", Record{Symbol: "GE", Type: "common stock", MIC: "XNYS"}, "NYSE"},
		{"lowercase symbol normalised", Record{Symbol: "amd", Type: "Common Stock", MIC: "XNAS"}, "NASDAQ"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := r.Evaluate(c.rec)
			if !d.Eligible {
				t.Fatalf("expected eligible, got reason %q", d.Reason)
			}
			if d.Reason != "" {
				t.Errorf("eligible row must have empty reason, got %q", d.Reason)
			}
			if d.Exchange != c.exchange {
				t.Errorf("exchange = %q, want %q", d.Exchange, c.exchange)
			}
		})
	}
}

// §3.1 excludes ETFs, ETNs, closed-end funds, mutual funds, warrants, rights,
// units, preferred shares and SPAC warrant/unit classes by instrument type.
func TestEvaluate_ExcludesNonCommonTypes(t *testing.T) {
	r := defaultRules()
	for _, typ := range []string{
		"ETP", "ETF", "Closed-End Fund", "Mutual Fund", "Open-End Fund",
		"Warrant", "Equity WRT", "Right", "Unit", "Preferred Stock",
		"ADR", "REIT", "Depositary Receipt", "GDR", "Bond",
		"Convertible Bond", "MLP", "Ltd Part", "Index", "Structured Product",
		"Tracking Stock", "Misc.", "", "PUBLIC",
	} {
		d := r.Evaluate(Record{Symbol: "ZZZZ", Type: typ, MIC: "XNAS"})
		if d.Eligible {
			t.Errorf("type %q must be excluded but was eligible", typ)
		}
		if d.Reason != ReasonTypeNotCommonStock {
			t.Errorf("type %q: reason = %q, want %q", typ, d.Reason, ReasonTypeNotCommonStock)
		}
	}
}

func TestEvaluate_ExcludesDisallowedVenues(t *testing.T) {
	r := defaultRules()
	// ARCX is NYSE Arca (predominantly ETFs); the rest are OTC tiers and ATSs.
	for _, mic := range []string{"ARCX", "BATS", "IEXG", "OTCM", "OOTC", "PSGM", "PINX", "XLON", "XTSE"} {
		d := r.Evaluate(Record{Symbol: "ZZZZ", Type: "Common Stock", MIC: mic})
		if d.Eligible {
			t.Errorf("mic %q must be excluded but was eligible", mic)
		}
		if d.Reason != ReasonExchangeNotAllowed {
			t.Errorf("mic %q: reason = %q, want %q", mic, d.Reason, ReasonExchangeNotAllowed)
		}
	}
}

func TestEvaluate_EmptyMICRejectedByDefaultAndAllowedWhenConfigured(t *testing.T) {
	rec := Record{Symbol: "ZZZZ", Type: "Common Stock", MIC: ""}

	if d := defaultRules().Evaluate(rec); d.Eligible {
		t.Error("blank MIC must be excluded by default — an unverifiable venue cannot satisfy an allowlist")
	} else if d.Reason != ReasonExchangeNotAllowed {
		t.Errorf("reason = %q, want %q", d.Reason, ReasonExchangeNotAllowed)
	}

	lenient := NewRules(nil, nil, nil, true)
	if d := lenient.Evaluate(rec); !d.Eligible {
		t.Errorf("blank MIC must be eligible when AllowEmptyMIC is set, got %q", d.Reason)
	}
}

// §3.1: "Ticker contains no suffix indicating a non-common class
// (e.g. trailing .W, .U, .R, -WT, -UN, -P)."
func TestEvaluate_ExcludesNonCommonTickerSuffixes(t *testing.T) {
	r := defaultRules()
	for _, sym := range []string{
		"ABCD.W", "ABCD.WS", "ABCD.WT", // warrants
		"ABCD.U", "ABCD.UN", // units
		"ABCD.R", "ABCD.RT", // rights
		"ABCD-WT", "ABCD-UN", "ABCD-U", "ABCD-R", "ABCD-RT",
		"ABCD-P", "ABCD-PA", "ABCD-PB", "ABCD-PRA", // preferred series
		"ABCD.P", "ABCD.PB",
		"abcd.u", // case-insensitive
	} {
		d := r.Evaluate(Record{Symbol: sym, Type: "Common Stock", MIC: "XNYS"})
		if d.Eligible {
			t.Errorf("symbol %q must be excluded but was eligible", sym)
		}
		if d.Reason != ReasonTickerSuffix {
			t.Errorf("symbol %q: reason = %q, want %q", sym, d.Reason, ReasonTickerSuffix)
		}
	}
}

// Share-class letters are ordinary common stock and must survive the suffix
// filter. A rule that drops BRK.B is silently discarding real candidates.
func TestEvaluate_KeepsShareClassAndSingleLetterTickers(t *testing.T) {
	r := defaultRules()
	for _, sym := range []string{
		"BRK.B", "BRK.A", "GOOG.A", "HEI.A", "LEN.B", // share classes
		"U",                       // Unity Software — a bare "U" is not a unit
		"R",                       // Ryder System — a bare "R" is not a right
		"WS",                      // not a suffix without a separator
		"F", "T", "X", "PG", "PM", // "P"-leading tickers with no separator
	} {
		d := r.Evaluate(Record{Symbol: sym, Type: "Common Stock", MIC: "XNYS"})
		if !d.Eligible {
			t.Errorf("symbol %q must stay eligible, got reason %q", sym, d.Reason)
		}
	}
}

func TestEvaluate_RejectsMalformedSymbols(t *testing.T) {
	r := defaultRules()
	for _, sym := range []string{"", "   ", "AB CD", "AB/CD", "AB\tCD"} {
		d := r.Evaluate(Record{Symbol: sym, Type: "Common Stock", MIC: "XNAS"})
		if d.Eligible {
			t.Errorf("symbol %q must be excluded but was eligible", sym)
		}
		if d.Reason != ReasonSymbolMalformed {
			t.Errorf("symbol %q: reason = %q, want %q", sym, d.Reason, ReasonSymbolMalformed)
		}
	}
}

// Rule precedence is observable in excluded_reason, so pin it: venue is checked
// before type, and type before suffix. A record failing several rules reports
// the first.
func TestEvaluate_ReasonPrecedence(t *testing.T) {
	r := defaultRules()
	d := r.Evaluate(Record{Symbol: "ABCD.W", Type: "Warrant", MIC: "OTCM"})
	if d.Reason != ReasonExchangeNotAllowed {
		t.Errorf("reason = %q, want %q (venue checked first)", d.Reason, ReasonExchangeNotAllowed)
	}
	d = r.Evaluate(Record{Symbol: "ABCD.W", Type: "Warrant", MIC: "XNAS"})
	if d.Reason != ReasonTypeNotCommonStock {
		t.Errorf("reason = %q, want %q (type before suffix)", d.Reason, ReasonTypeNotCommonStock)
	}
}

// A blank env var must mean "spec default", never "allow nothing" — otherwise a
// missing config line empties the universe.
func TestNewRules_EmptyConfigFallsBackToDefaults(t *testing.T) {
	r := NewRules([]string{}, []string{}, []string{}, false)
	if d := r.Evaluate(Record{Symbol: "AAPL", Type: "Common Stock", MIC: "XNAS"}); !d.Eligible {
		t.Fatalf("empty config must fall back to defaults, got %q", d.Reason)
	}
	if d := r.Evaluate(Record{Symbol: "AAPL.W", Type: "Common Stock", MIC: "XNAS"}); d.Reason != ReasonTickerSuffix {
		t.Errorf("suffix defaults not applied: got %q", d.Reason)
	}
}

func TestNewRules_CustomTypesWidenTheAllowlist(t *testing.T) {
	r := NewRules([]string{"Common Stock", "ADR"}, nil, nil, false)
	if d := r.Evaluate(Record{Symbol: "TSM", Type: "ADR", MIC: "XNYS"}); !d.Eligible {
		t.Errorf("ADR must be eligible once allowlisted, got %q", d.Reason)
	}
	if d := r.Evaluate(Record{Symbol: "SPY", Type: "ETP", MIC: "XNAS"}); d.Eligible {
		t.Error("widening to ADR must not admit ETPs")
	}
}

func TestTally_CountsAndOrdersReasons(t *testing.T) {
	r := defaultRules()
	tl := NewTally()
	recs := []Record{
		{Symbol: "AAPL", Type: "Common Stock", MIC: "XNAS"},
		{Symbol: "MSFT", Type: "Common Stock", MIC: "XNAS"},
		{Symbol: "SPY", Type: "ETP", MIC: "ARCX"},            // exchange (checked first)
		{Symbol: "QQQ", Type: "ETP", MIC: "XNAS"},            // type
		{Symbol: "IWM", Type: "ETP", MIC: "XNAS"},            // type
		{Symbol: "ABC.U", Type: "Common Stock", MIC: "XNYS"}, // suffix
	}
	for _, rec := range recs {
		tl.Add(r.Evaluate(rec))
	}
	if tl.Total != 6 || tl.Eligible != 2 {
		t.Fatalf("total/eligible = %d/%d, want 6/2", tl.Total, tl.Eligible)
	}
	if got := tl.ByReason[ReasonTypeNotCommonStock]; got != 2 {
		t.Errorf("type exclusions = %d, want 2", got)
	}
	if got := tl.ByReason[ReasonExchangeNotAllowed]; got != 1 {
		t.Errorf("exchange exclusions = %d, want 1", got)
	}
	if got := tl.ByReason[ReasonTickerSuffix]; got != 1 {
		t.Errorf("suffix exclusions = %d, want 1", got)
	}
	// Most frequent reason first.
	if order := tl.ReasonsSorted(); order[0] != ReasonTypeNotCommonStock {
		t.Errorf("ReasonsSorted()[0] = %q, want %q", order[0], ReasonTypeNotCommonStock)
	}
}

// Eligibility must never consider bar history — that check needs bars, which
// the backfill produces by iterating the eligible set. Enforcing it here would
// be circular and leave the universe permanently empty.
func TestEvaluate_DoesNotConsiderBarHistory(t *testing.T) {
	r := defaultRules()
	d := r.Evaluate(Record{Symbol: "NEWIPO", Type: "Common Stock", MIC: "XNAS"})
	if !d.Eligible {
		t.Fatalf("a symbol with zero bars must still be eligible at list load, got %q", d.Reason)
	}
	if d.Reason == ReasonInsufficientHistory {
		t.Error("insufficient_history must not be produced by Evaluate")
	}
}

// BuildPlan must separate the two reasons a fetched record is not persisted:
// an excluded off-venue listing, and a deduplicated cross-tier duplicate.
// Conflating them makes the filter look broken when it is only deduplicating.
func TestBuildPlan_SeparatesOffVenueFromDuplicates(t *testing.T) {
	recs := []Record{
		{Symbol: "AAPL", Type: "Common Stock", MIC: "XNAS"},
		{Symbol: "AAPL", Type: "Common Stock", MIC: "XNMS"}, // same ticker, NASDAQ tier
		{Symbol: "MSFT", Type: "Common Stock", MIC: "XNGS"},
		{Symbol: "QQQ", Type: "ETP", MIC: "XNAS"},            // excluded, but keyable
		{Symbol: "SPY", Type: "ETP", MIC: "ARCX"},            // off-venue
		{Symbol: "PENNY", Type: "Common Stock", MIC: "OTCM"}, // off-venue
	}
	p := defaultRules().BuildPlan(recs)

	if p.Tally.Total != 6 {
		t.Errorf("tally total = %d, want 6 (every input counted)", p.Tally.Total)
	}
	if p.SkippedOffVenue != 2 {
		t.Errorf("SkippedOffVenue = %d, want 2", p.SkippedOffVenue)
	}
	if p.SkippedDuplicate != 1 {
		t.Errorf("SkippedDuplicate = %d, want 1", p.SkippedDuplicate)
	}
	// AAPL (once), MSFT, QQQ — exclusions stay persistable so they remain auditable.
	if len(p.Decisions) != 3 {
		t.Fatalf("decisions = %d, want 3", len(p.Decisions))
	}
	if p.Tally.Eligible != 3 {
		t.Errorf("eligible = %d, want 3 (both AAPL listings count as eligible inputs)", p.Tally.Eligible)
	}
}

// The batch is upserted against a (symbol, exchange) primary key, so it must
// never contain the same pair twice or the statement conflicts with itself.
func TestBuildPlan_DecisionsAreUniqueOnPrimaryKey(t *testing.T) {
	recs := []Record{
		{Symbol: "AAPL", Type: "Common Stock", MIC: "XNAS"},
		{Symbol: "AAPL", Type: "Common Stock", MIC: "XNGS"},
		{Symbol: "AAPL", Type: "Common Stock", MIC: "XNCM"},
		{Symbol: "aapl", Type: "Common Stock", MIC: "XNMS"}, // case variant
	}
	p := defaultRules().BuildPlan(recs)
	seen := map[string]bool{}
	for _, d := range p.Decisions {
		k := d.Symbol + "|" + d.Exchange
		if seen[k] {
			t.Fatalf("duplicate primary key in batch: %s", k)
		}
		seen[k] = true
	}
	if len(p.Decisions) != 1 {
		t.Errorf("decisions = %d, want 1 (all four are NASDAQ AAPL)", len(p.Decisions))
	}
}

// A symbol on genuinely different exchanges is not a duplicate.
func TestBuildPlan_SameTickerDifferentExchangeIsKept(t *testing.T) {
	p := defaultRules().BuildPlan([]Record{
		{Symbol: "ZZZZ", Type: "Common Stock", MIC: "XNAS"},
		{Symbol: "ZZZZ", Type: "Common Stock", MIC: "XNYS"},
	})
	if len(p.Decisions) != 2 {
		t.Errorf("decisions = %d, want 2 (NASDAQ and NYSE are distinct keys)", len(p.Decisions))
	}
	if p.SkippedDuplicate != 0 {
		t.Errorf("SkippedDuplicate = %d, want 0", p.SkippedDuplicate)
	}
}

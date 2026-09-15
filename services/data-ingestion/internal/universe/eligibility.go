// Package universe implements the momentum scanner's symbol-list eligibility
// rules (docs/MOMENTUM_SCANNER_PHASE1.md §3.1).
//
// Everything here is a pure function over a raw symbol-list record so the rules
// can be unit-tested without a network or database. The rules are the product:
// a wrong exclusion silently shrinks the scannable universe, and a wrong
// inclusion fills the penny bucket with warrants and units.
package universe

import (
	"sort"
	"strings"
)

// Exclusion reasons, stored verbatim in universe_symbols.excluded_reason.
const (
	ReasonTypeNotCommonStock = "type_not_common_stock"
	ReasonExchangeNotAllowed = "exchange_not_allowed"
	ReasonTickerSuffix       = "ticker_suffix_excluded"
	ReasonSymbolMalformed    = "symbol_malformed"

	// ReasonInsufficientHistory is NOT applied at symbol-list load: bars do not
	// exist until the §8.1.3 backfill has run. It is recorded later, once
	// bar_count is known, and the same 250-bar minimum is additionally enforced
	// as a hard gate (§3.2) at scan time.
	ReasonInsufficientHistory = "insufficient_history"
)

// DefaultAllowedTypes is the Finnhub `type` allowlist.
//
// §3.1 says "instrument type is common stock" and excludes ETFs, ETNs,
// closed-end funds, mutual funds, warrants, rights, units, preferred shares and
// SPAC warrant/unit classes. That is a strict reading: ADRs, REITs and
// tracking stocks are NOT included here even though they are arguably common
// equity, because the spec names only common stock. Widen via
// UNIVERSE_ALLOWED_TYPES rather than editing this list.
var DefaultAllowedTypes = []string{"Common Stock"}

// DefaultAllowedMICs are the ISO 10383 market identifiers for NASDAQ, NYSE and
// NYSE American, including the NASDAQ tier codes Finnhub sometimes returns
// instead of the composite XNAS.
//
//	XNAS  NASDAQ (composite)      XNGS  NASDAQ Global Select
//	XNMS  NASDAQ Global Market    XNCM  NASDAQ Capital Market
//	XNYS  NYSE                    XASE  NYSE American (ex-AMEX)
//
// Deliberately absent: ARCX (NYSE Arca — predominantly ETFs), BATS, IEXG, and
// every OTC venue (OTCM, OOTC, PSGM, PINX). §3.1 excludes OTC/pink sheets in
// Phase 1 because data quality is poor and free sources cover them badly.
var DefaultAllowedMICs = []string{"XNAS", "XNGS", "XNMS", "XNCM", "XNYS", "XASE"}

// DefaultExcludedSuffixes are ticker suffixes that mark a non-common share
// class. Matched against the portion of the ticker after a '.' or '-'
// separator, case-insensitively.
//
//	W, WS, WT   warrants        U, UN   units (SPAC common+warrant bundles)
//	R, RT       rights          P*      preferred series (e.g. BAC-PB, -PA)
//
// Share-class letters (BRK.B, GOOG.A) are NOT here and must stay includable —
// they are ordinary common stock.
var DefaultExcludedSuffixes = []string{"W", "WS", "WT", "U", "UN", "R", "RT"}

// preferredSuffixPrefix matches preferred series, which are lettered rather than
// fixed: -P, -PA, -PB, -PRA … all denote preferred, none denote common.
const preferredSuffixPrefix = "P"

// Rules is the configured eligibility policy. Zero value is not usable; build
// one with NewRules so the sets are normalised.
type Rules struct {
	allowedTypes     map[string]struct{}
	allowedMICs      map[string]struct{}
	excludedSuffixes map[string]struct{}

	// AllowEmptyMIC includes records whose mic field is blank. Finnhub
	// occasionally omits it. Default false: an unknown venue cannot be verified
	// as NASDAQ/NYSE/NYSE American, and §3.1 is an allowlist.
	AllowEmptyMIC bool
}

// NewRules normalises the three sets. Empty input falls back to the Default*
// values, so a blank env var means "spec default", not "allow nothing".
func NewRules(types, mics, suffixes []string, allowEmptyMIC bool) Rules {
	if len(types) == 0 {
		types = DefaultAllowedTypes
	}
	if len(mics) == 0 {
		mics = DefaultAllowedMICs
	}
	if len(suffixes) == 0 {
		suffixes = DefaultExcludedSuffixes
	}
	r := Rules{
		allowedTypes:     make(map[string]struct{}, len(types)),
		allowedMICs:      make(map[string]struct{}, len(mics)),
		excludedSuffixes: make(map[string]struct{}, len(suffixes)),
		AllowEmptyMIC:    allowEmptyMIC,
	}
	for _, t := range types {
		if t = strings.TrimSpace(t); t != "" {
			r.allowedTypes[strings.ToLower(t)] = struct{}{}
		}
	}
	for _, m := range mics {
		if m = strings.TrimSpace(m); m != "" {
			r.allowedMICs[strings.ToUpper(m)] = struct{}{}
		}
	}
	for _, s := range suffixes {
		if s = strings.TrimSpace(s); s != "" {
			r.excludedSuffixes[strings.ToUpper(s)] = struct{}{}
		}
	}
	return r
}

// Record is one raw entry from the provider's symbol list.
type Record struct {
	Symbol      string
	DisplayName string
	Type        string
	MIC         string
	Currency    string
	FIGI        string
}

// Decision is the outcome of applying §3.1 to a Record.
type Decision struct {
	Symbol   string
	Exchange string // resolved human-readable venue, or "" when the MIC is unknown
	Eligible bool
	Reason   string // "" iff Eligible
}

// micToExchange maps an allowed MIC to the exchange name stored in
// universe_symbols.exchange, which is part of that table's primary key.
var micToExchange = map[string]string{
	"XNAS": "NASDAQ",
	"XNGS": "NASDAQ",
	"XNMS": "NASDAQ",
	"XNCM": "NASDAQ",
	"XNYS": "NYSE",
	"XASE": "NYSE American",
}

// Evaluate applies §3.1's three statically-checkable rules: instrument type,
// exchange, and ticker suffix.
//
// The fourth rule — at least 250 daily bars of history — is deliberately NOT
// applied here. At symbol-list load no bars exist yet (they arrive with the
// §8.1.3 backfill, which itself iterates the eligible set), so enforcing it
// here would be circular and would leave the universe permanently empty. The
// bar minimum is enforced as a hard gate at scan time, where §3.2 also lists
// it, and bar_count is recorded on the row for auditability.
func (r Rules) Evaluate(rec Record) Decision {
	sym := strings.ToUpper(strings.TrimSpace(rec.Symbol))
	d := Decision{Symbol: sym}

	if sym == "" || strings.ContainsAny(sym, " \t/") {
		d.Reason = ReasonSymbolMalformed
		return d
	}

	mic := strings.ToUpper(strings.TrimSpace(rec.MIC))
	if mic == "" {
		if !r.AllowEmptyMIC {
			d.Reason = ReasonExchangeNotAllowed
			return d
		}
	} else if _, ok := r.allowedMICs[mic]; !ok {
		d.Reason = ReasonExchangeNotAllowed
		return d
	}
	d.Exchange = micToExchange[mic]

	if _, ok := r.allowedTypes[strings.ToLower(strings.TrimSpace(rec.Type))]; !ok {
		d.Reason = ReasonTypeNotCommonStock
		return d
	}

	if r.hasExcludedSuffix(sym) {
		d.Reason = ReasonTickerSuffix
		return d
	}

	d.Eligible = true
	return d
}

// hasExcludedSuffix reports whether the ticker carries a non-common-class
// suffix after a '.' or '-' separator.
//
// Only the final segment is examined, and only when a separator is present:
// that keeps plain tickers like "U" (Unity Software) and "R" (Ryder) eligible
// while excluding "ABC.U" and "ABC-R". Share-class letters such as BRK.B are
// not in the excluded set and stay eligible.
func (r Rules) hasExcludedSuffix(symbol string) bool {
	idx := strings.LastIndexAny(symbol, ".-")
	if idx < 0 || idx == len(symbol)-1 {
		return false
	}
	suffix := symbol[idx+1:]
	if _, ok := r.excludedSuffixes[suffix]; ok {
		return true
	}
	// Preferred series are lettered: P, PA, PB, PRA … Treat any suffix starting
	// with "P" as preferred. No common-stock class suffix begins with P.
	return strings.HasPrefix(suffix, preferredSuffixPrefix)
}

// Plan is the result of applying the rules across a whole symbol directory:
// the decisions that can be persisted, plus an account of everything that could
// not be, so no record silently disappears between fetch and upsert.
type Plan struct {
	// Decisions is one entry per persistable row, deduplicated on
	// (symbol, exchange) — that pair is universe_symbols' primary key.
	Decisions []Decision

	// Tally counts every input record by outcome, including records that were
	// not persisted.
	Tally *Tally

	// SkippedOffVenue counts records whose MIC resolves to no allowed exchange.
	// These cannot be keyed and are exclusively the OTC and foreign listings
	// §3.1 drops.
	SkippedOffVenue int

	// SkippedDuplicate counts records collapsed because the same ticker appears
	// on more than one venue code — the US directory lists a symbol on several
	// NASDAQ tiers. Counted separately from SkippedOffVenue: one is an exclusion
	// and the other is deduplication, and conflating them in a log makes the
	// filter look wrong.
	SkippedDuplicate int
}

// BuildPlan evaluates every record and assembles the persistable set.
//
// Order is preserved and the first record for a (symbol, exchange) pair wins,
// so a directory listing AAPL on both XNAS and XNMS yields one NASDAQ row and
// a batch that cannot self-conflict on its own primary key.
func (r Rules) BuildPlan(recs []Record) Plan {
	p := Plan{
		Decisions: make([]Decision, 0, len(recs)),
		Tally:     NewTally(),
	}
	seen := make(map[string]struct{}, len(recs))
	for _, rec := range recs {
		d := r.Evaluate(rec)
		p.Tally.Add(d)

		if d.Exchange == "" {
			p.SkippedOffVenue++
			continue
		}
		k := d.Symbol + "\x00" + d.Exchange
		if _, dup := seen[k]; dup {
			p.SkippedDuplicate++
			continue
		}
		seen[k] = struct{}{}
		p.Decisions = append(p.Decisions, d)
	}
	return p
}

// Tally counts decisions by reason, for the audit summary §10 step 2 asks for
// ("exclusions auditable").
type Tally struct {
	Total    int
	Eligible int
	ByReason map[string]int
}

func NewTally() *Tally { return &Tally{ByReason: map[string]int{}} }

func (t *Tally) Add(d Decision) {
	t.Total++
	if d.Eligible {
		t.Eligible++
		return
	}
	t.ByReason[d.Reason]++
}

// ReasonsSorted returns reasons ordered by descending count for stable logging.
func (t *Tally) ReasonsSorted() []string {
	out := make([]string, 0, len(t.ByReason))
	for k := range t.ByReason {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if t.ByReason[out[i]] != t.ByReason[out[j]] {
			return t.ByReason[out[i]] > t.ByReason[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}

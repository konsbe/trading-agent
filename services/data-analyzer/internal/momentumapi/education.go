package momentumapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/heuristics"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
)

// Education (docs/EDUCATION_SECTION_CONTENT_SPEC.md, docs/MOMENTUM_SCANNER_API.md
// "Education content").
//
//	GET /api/v1/education/handbook
//	GET /api/v1/education/masterclass
//	GET /api/v1/education/glossary
//
// Static, authored content read from shared/content/handbook.json and
// masterclass.json at startup — the Backtest Lab pattern: no fallback copy, a
// missing or malformed file fails startup, and every body is built once at
// load. Handbook caveat blocks carry only a key into momentum_caveats.json and
// are resolved here, so a Handbook page cannot drift from the caveat every
// other surface shows. The Glossary is extracted from both files' `terms`,
// never authored separately.

// Education holds the three precomputed response bodies.
type Education struct {
	handbook    []byte
	masterClass []byte
	glossary    []byte
}

type eduBlock struct {
	Type  string   `json:"type"`
	Text  string   `json:"text,omitempty"`
	Items []string `json:"items,omitempty"`
	Key   string   `json:"key,omitempty"`
}

type eduTerm struct {
	Term       string   `json:"term"`
	Synonyms   []string `json:"synonyms,omitempty"`
	Definition string   `json:"definition"`
}

type handbookEntry struct {
	ID     string     `json:"id"`
	Title  string     `json:"title"`
	Blocks []eduBlock `json:"blocks"`
	Terms  []eduTerm  `json:"terms"`
}

type handbookSection struct {
	ID      string          `json:"id"`
	SpecRef string          `json:"spec_ref,omitempty"`
	Title   string          `json:"title"`
	Intro   string          `json:"intro,omitempty"`
	Entries []handbookEntry `json:"entries"`
}

type handbookDoc struct {
	Version  string            `json:"version"`
	Status   string            `json:"status"`
	Notes    string            `json:"notes,omitempty"`
	Sections []handbookSection `json:"sections"`
}

type masterClassEntry struct {
	ID      string     `json:"id"`
	Title   string     `json:"title"`
	Summary string     `json:"summary"`
	Blocks  []eduBlock `json:"blocks"`
	Terms   []eduTerm  `json:"terms"`
}

type masterClassModule struct {
	ID      string             `json:"id"`
	Number  *int               `json:"number,omitempty"`
	Title   string             `json:"title"`
	Entries []masterClassEntry `json:"entries"`
}

type masterClassDoc struct {
	Version string              `json:"version"`
	Status  string              `json:"status"`
	Notes   string              `json:"notes,omitempty"`
	Modules []masterClassModule `json:"modules"`
}

// GlossaryTerm is one row of GET /api/v1/education/glossary.
type GlossaryTerm struct {
	Term       string   `json:"term"`
	Synonyms   []string `json:"synonyms"`
	Definition string   `json:"definition"`
	// Source is "handbook" or "masterclass"; EntryID and SectionOrModuleID
	// locate the fuller entry the Glossary points at.
	Source            string `json:"source"`
	EntryID           string `json:"entry_id"`
	SectionOrModuleID string `json:"section_or_module_id"`
}

type glossaryResponse struct {
	HandbookVersion    string         `json:"handbook_version"`
	MasterClassVersion string         `json:"masterclass_version"`
	Terms              []GlossaryTerm `json:"terms"`
}

// masterClassBannedTerms are this app's own field names and output labels,
// which MasterClass must never contain (spec §0, §5 step 6). Matched as whole
// tokens, case-sensitively: plain English ("heuristics", "time out") is fine,
// only the exact snake_case names and UPPERCASE labels are refused.
var masterClassBannedTerms = []string{
	// Score and its status.
	"momentum_score_100", "momentum_score", "score_attainable", "score_status",
	"sub_scores", "null_inputs", "penalty_rules", "penalty_total", "vol_accel",
	"catalyst_tier", "model_version",
	// Action labels (heuristic sweep rule and analyst-bot's actions engine).
	heuristics.ActionBuyWatch, heuristics.ActionTrimWatch, "PREPARE_LONG", "HOLD_WATCH",
	"WATCH_ACCUMULATE", "REVIEW_POSITION", "REDUCE_SIZE", "REDUCE_SIZE_WATCH_REVERSAL",
	// Shared caveats, by label and by key.
	"HEURISTIC_TA_CAVEAT", "EVIDENCE_CAVEAT", "RESEARCH_SCORE_CAVEAT",
	"heuristic_ta_caveat", "evidence_caveat", "research_score_caveat", "evidence_note",
	// Scanner features and gates.
	string(momentum.BreakoutFromConsolidation), "breakout_state", "was_consolidating",
	"gates_passed", "gate_failures", "rvol_20", "avg_volume_20", "dollar_volume",
	"change_pct", "pct_of_52w_high", "high_52w", "resistance_20", "vwap_20", "vwap_dist_pct",
	"is_candidate_today", "is_stale",
	momentum.GateNoClose, momentum.GateCloseBelowFloor, momentum.GateCloseAboveCeil,
	momentum.GateShortHistory, momentum.GateNoChangePct, momentum.GateChangeTooLow,
	momentum.GateChangeTooHigh, momentum.GateNoRVol, momentum.GateRVolTooLow,
	momentum.GateNoDollarVol, momentum.GateDollarVolTooLow, momentum.GateMarketCapNull,
	momentum.GateMarketCapUnavailable, momentum.GateMarketCapTooLow, momentum.GateMarketCapTooHigh,
	momentum.GateUnbucketable, momentum.GateMarketCapPITUnavailable,
	// Alerts, heuristic signals and the analysis response.
	"fired_alerts", "recent_alert", "heuristic_signals", "action_signal", "chart_patterns",
	"liq_sweeps", "fvgs_active", "obs_active", "master_signals", "aligned_signals",
	"context_vs_benchmark", "market_cycle_composite", "market_cycle_tone", "correlation_regime",
	"moat_proxy", "insider_signal", "rd_intensity", "eps_strength", "pe_vs_5y",
	// Tracked positions.
	"exit_reason", "exit_reason_note", "exit_reason_notes", "evaluation_behind",
	"reference_price", "unrealized_pct", "max_gain_pct", "drawdown_from_peak_pct",
	"sessions_elapsed", "tracker_completed", "scanner_completed",
	momentum.ExitBreakoutFailed, momentum.ExitLostVWAP, momentum.ExitMomentumStalled,
	momentum.ExitStopATR, momentum.ExitTimeout,
}

// masterClassBanned matches any banned term as a whole token: the characters
// either side must not be letters, digits or underscores.
var masterClassBanned = func() *regexp.Regexp {
	terms := append([]string(nil), masterClassBannedTerms...)
	sort.Slice(terms, func(i, j int) bool { return len(terms[i]) > len(terms[j]) })
	for i, t := range terms {
		terms[i] = regexp.QuoteMeta(t)
	}
	return regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(` + strings.Join(terms, "|") + `)(?:[^A-Za-z0-9_]|$)`)
}()

// LoadEducation loads both files, resolves Handbook caveats from the
// already-loaded caveats, and builds the Glossary.
func LoadEducation(handbookPath, masterClassPath string, caveats Caveats) (Education, error) {
	hb, hbBody, err := loadHandbook(handbookPath, caveats)
	if err != nil {
		return Education{}, err
	}
	mc, mcBody, err := loadMasterClass(masterClassPath)
	if err != nil {
		return Education{}, err
	}
	terms, err := buildGlossary(hb, mc)
	if err != nil {
		return Education{}, err
	}
	glossary, err := marshalContent(glossaryResponse{
		HandbookVersion: hb.Version, MasterClassVersion: mc.Version, Terms: terms,
	})
	if err != nil {
		return Education{}, fmt.Errorf("encode glossary: %w", err)
	}
	return Education{handbook: hbBody, masterClass: mcBody, glossary: glossary}, nil
}

func loadHandbook(path string, caveats Caveats) (handbookDoc, []byte, error) {
	var doc handbookDoc
	if _, err := decodeContent(path, "handbook", &doc); err != nil {
		return handbookDoc{}, nil, err
	}
	fail := func(format string, a ...any) (handbookDoc, []byte, error) {
		return handbookDoc{}, nil, fmt.Errorf("handbook %s: "+format, append([]any{path}, a...)...)
	}
	if doc.Version == "" || doc.Status == "" {
		return fail("version and status are required")
	}
	if len(doc.Sections) == 0 {
		return fail("sections is required")
	}
	sectionIDs, entryIDs := map[string]bool{}, map[string]bool{}
	for si := range doc.Sections {
		s := &doc.Sections[si]
		if s.ID == "" || s.Title == "" {
			return fail("sections[%d]: id and title are required", si)
		}
		if sectionIDs[s.ID] {
			return fail("duplicate section id %q", s.ID)
		}
		sectionIDs[s.ID] = true
		if s.Entries == nil {
			s.Entries = []handbookEntry{}
		}
		for ei := range s.Entries {
			e := &s.Entries[ei]
			if err := checkEntry(e.ID, e.Title, e.Blocks, entryIDs); err != nil {
				return fail("section %q: %v", s.ID, err)
			}
			for bi := range e.Blocks {
				b := &e.Blocks[bi]
				if err := checkBlock(*b, true); err != nil {
					return fail("entry %q blocks[%d]: %v", e.ID, bi, err)
				}
				if b.Type == "caveat" {
					text, ok := caveats.byKey(b.Key)
					if !ok {
						return fail("entry %q blocks[%d]: unknown caveat key %q", e.ID, bi, b.Key)
					}
					b.Text = text
				}
			}
			if err := checkTerms(e.Terms); err != nil {
				return fail("entry %q: %v", e.ID, err)
			}
			if e.Terms == nil {
				e.Terms = []eduTerm{}
			}
		}
	}
	body, err := marshalContent(doc)
	if err != nil {
		return fail("encode: %v", err)
	}
	return doc, body, nil
}

func loadMasterClass(path string) (masterClassDoc, []byte, error) {
	var doc masterClassDoc
	raw, err := decodeContent(path, "masterclass", &doc)
	if err != nil {
		return masterClassDoc{}, nil, err
	}
	fail := func(format string, a ...any) (masterClassDoc, []byte, error) {
		return masterClassDoc{}, nil, fmt.Errorf("masterclass %s: "+format, append([]any{path}, a...)...)
	}
	if doc.Version == "" || doc.Status == "" {
		return fail("version and status are required")
	}
	if len(doc.Modules) == 0 {
		return fail("modules is required")
	}
	moduleIDs, entryIDs := map[string]bool{}, map[string]bool{}
	for mi, m := range doc.Modules {
		if m.ID == "" || m.Title == "" {
			return fail("modules[%d]: id and title are required", mi)
		}
		if moduleIDs[m.ID] {
			return fail("duplicate module id %q", m.ID)
		}
		moduleIDs[m.ID] = true
		for _, e := range m.Entries {
			if err := checkEntry(e.ID, e.Title, e.Blocks, entryIDs); err != nil {
				return fail("module %q: %v", m.ID, err)
			}
			if strings.TrimSpace(e.Summary) == "" {
				return fail("entry %q: summary is required", e.ID)
			}
			for bi, b := range e.Blocks {
				if err := checkBlock(b, false); err != nil {
					return fail("entry %q blocks[%d]: %v", e.ID, bi, err)
				}
			}
			if err := checkTerms(e.Terms); err != nil {
				return fail("entry %q: %v", e.ID, err)
			}
		}
	}
	if err := checkMasterClassPurity(raw); err != nil {
		return fail("%v", err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return fail("compact: %v", err)
	}
	return doc, compact.Bytes(), nil
}

// decodeContent reads path into v, refusing unknown fields: a field the loader
// does not know would be silently dropped from the served Handbook.
func decodeContent(path, name string, v any) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return nil, fmt.Errorf("parse %s %s: %w", name, path, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("parse %s %s: trailing data after the document", name, path)
	}
	return raw, nil
}

func checkEntry(id, title string, blocks []eduBlock, seen map[string]bool) error {
	if id == "" || title == "" {
		return fmt.Errorf("every entry needs an id and a title")
	}
	if seen[id] {
		return fmt.Errorf("duplicate entry id %q", id)
	}
	seen[id] = true
	if len(blocks) == 0 {
		return fmt.Errorf("entry %q: blocks must not be empty", id)
	}
	return nil
}

func checkBlock(b eduBlock, handbook bool) error {
	switch b.Type {
	case "paragraph", "heading":
		if strings.TrimSpace(b.Text) == "" || b.Items != nil || b.Key != "" {
			return fmt.Errorf("%s block takes non-empty text only", b.Type)
		}
	case "list":
		if len(b.Items) == 0 || b.Text != "" || b.Key != "" {
			return fmt.Errorf("list block takes non-empty items only")
		}
		for _, it := range b.Items {
			if strings.TrimSpace(it) == "" {
				return fmt.Errorf("list block has an empty item")
			}
		}
	case "caveat":
		if !handbook {
			return fmt.Errorf("caveat blocks are Handbook-only")
		}
		if b.Key == "" || b.Text != "" || b.Items != nil {
			return fmt.Errorf("caveat block takes a key only (caveat text is never typed into the content file)")
		}
	default:
		return fmt.Errorf("unknown block type %q", b.Type)
	}
	return nil
}

func checkTerms(terms []eduTerm) error {
	for i, t := range terms {
		if strings.TrimSpace(t.Term) == "" || strings.TrimSpace(t.Definition) == "" {
			return fmt.Errorf("terms[%d]: term and definition are required", i)
		}
		for _, s := range t.Synonyms {
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("terms[%d] %q: empty synonym", i, t.Term)
			}
		}
	}
	return nil
}

// checkMasterClassPurity walks every string value in the file.
func checkMasterClassPurity(raw []byte) error {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	var walk func(path string, v any) error
	walk = func(path string, v any) error {
		switch x := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if err := walk(path+"."+k, x[k]); err != nil {
					return err
				}
			}
		case []any:
			for i, child := range x {
				if err := walk(fmt.Sprintf("%s[%d]", path, i), child); err != nil {
					return err
				}
			}
		case string:
			if m := masterClassBanned.FindStringSubmatch(x); m != nil {
				return fmt.Errorf("%s contains %q, one of this app's own field names or labels; MasterClass is general content only (spec §0)", path, m[1])
			}
		}
		return nil
	}
	return walk("$", v)
}

func buildGlossary(hb handbookDoc, mc masterClassDoc) ([]GlossaryTerm, error) {
	out := []GlossaryTerm{}
	add := func(t eduTerm, source, entryID, parentID string) {
		syn := t.Synonyms
		if syn == nil {
			syn = []string{}
		}
		out = append(out, GlossaryTerm{Term: t.Term, Synonyms: syn, Definition: t.Definition,
			Source: source, EntryID: entryID, SectionOrModuleID: parentID})
	}
	for _, s := range hb.Sections {
		for _, e := range s.Entries {
			for _, t := range e.Terms {
				add(t, "handbook", e.ID, s.ID)
			}
		}
	}
	for _, m := range mc.Modules {
		for _, e := range m.Entries {
			for _, t := range e.Terms {
				add(t, "masterclass", e.ID, m.ID)
			}
		}
	}
	seen := map[string]GlossaryTerm{}
	for _, g := range out {
		k := strings.ToLower(strings.TrimSpace(g.Term))
		if prev, dup := seen[k]; dup {
			return nil, fmt.Errorf("glossary: term %q is defined twice (%s/%s and %s/%s); define it once and point at it",
				g.Term, prev.Source, prev.EntryID, g.Source, g.EntryID)
		}
		seen[k] = g
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Term), strings.ToLower(out[j].Term)
		if a != b {
			return a < b
		}
		return out[i].Term < out[j].Term
	})
	return out, nil
}

// marshalContent encodes without HTML escaping, so "&" in authored text is
// served as "&", not "\u0026".
func marshalContent(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// educationMaxAge: authored content, changed only by a deliberate new version
// (same reasoning as backtestReportMaxAge).
const educationMaxAge = "public, max-age=86400"

func (s *Server) handleEducationHandbook(w http.ResponseWriter, _ *http.Request) {
	s.serveEducation(w, s.cfg.Education.handbook)
}

func (s *Server) handleEducationMasterClass(w http.ResponseWriter, _ *http.Request) {
	s.serveEducation(w, s.cfg.Education.masterClass)
}

func (s *Server) handleEducationGlossary(w http.ResponseWriter, _ *http.Request) {
	s.serveEducation(w, s.cfg.Education.glossary)
}

func (s *Server) serveEducation(w http.ResponseWriter, body []byte) {
	if len(body) == 0 {
		// main refuses to start without the content; only a miswired Server gets here.
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "education_content_not_loaded"})
		return
	}
	w.Header().Set("Cache-Control", educationMaxAge)
	writeBody(w, http.StatusOK, body)
}

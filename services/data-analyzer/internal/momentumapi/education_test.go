package momentumapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func sharedContentPath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "shared", "content", name)
}

func loadSharedEducation(t *testing.T) Education {
	t.Helper()
	e, err := LoadEducation(sharedContentPath(t, "handbook.json"), sharedContentPath(t, "masterclass.json"), loadSharedCaveats(t))
	if err != nil {
		t.Fatalf("load education content: %v", err)
	}
	return e
}

// readSharedJSON decodes a checked-in file directly, independently of the loader.
func readSharedJSON(t *testing.T, name string) (map[string]any, []byte) {
	t.Helper()
	raw, err := os.ReadFile(sharedContentPath(t, name))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m, raw
}

func writeFixture(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// Fixtures: one section/module with one entry. %s is the entry's blocks, then its terms.
const (
	handbookFixture    = `{"version":"1","status":"draft","sections":[{"id":"s1","title":"S","entries":[{"id":"e1","title":"E","blocks":[%s],"terms":[%s]}]}]}`
	masterClassFixture = `{"version":"1","status":"draft","modules":[{"id":"m1","title":"M","entries":[{"id":"r1","title":"R","summary":"short","blocks":[%s],"terms":[%s]}]}]}`
	paragraph          = `{"type":"paragraph","text":"p"}`
)

func loadFixtures(t *testing.T, handbook, masterClass string) (Education, error) {
	t.Helper()
	dir := t.TempDir()
	return LoadEducation(writeFixture(t, dir, "handbook.json", handbook),
		writeFixture(t, dir, "masterclass.json", masterClass), loadSharedCaveats(t))
}

func validHandbook() string    { return fmt.Sprintf(handbookFixture, paragraph, "") }
func validMasterClass() string { return fmt.Sprintf(masterClassFixture, paragraph, "") }

func TestLoadEducation_FixturesLoad(t *testing.T) {
	if _, err := loadFixtures(t, validHandbook(), validMasterClass()); err != nil {
		t.Fatalf("minimal fixtures must load: %v", err)
	}
}

func TestLoadEducation_RefusesMissingFiles(t *testing.T) {
	dir := t.TempDir()
	hb := writeFixture(t, dir, "handbook.json", validHandbook())
	mc := writeFixture(t, dir, "masterclass.json", validMasterClass())
	absent := filepath.Join(dir, "absent.json")
	if _, err := LoadEducation(absent, mc, loadSharedCaveats(t)); err == nil {
		t.Error("a missing handbook must fail startup")
	}
	if _, err := LoadEducation(hb, absent, loadSharedCaveats(t)); err == nil {
		t.Error("a missing masterclass must fail startup")
	}
}

func TestLoadEducation_RefusesMalformedHandbook(t *testing.T) {
	for name, body := range map[string]string{
		"not json":           `{`,
		"no version":         `{"status":"draft","sections":[{"id":"s1","title":"S","entries":[]}]}`,
		"no status":          `{"version":"1","sections":[{"id":"s1","title":"S","entries":[]}]}`,
		"no sections":        `{"version":"1","status":"draft","sections":[]}`,
		"unknown field":      `{"version":"1","status":"draft","extra":1,"sections":[{"id":"s1","title":"S","entries":[]}]}`,
		"section no title":   `{"version":"1","status":"draft","sections":[{"id":"s1","entries":[]}]}`,
		"duplicate section":  `{"version":"1","status":"draft","sections":[{"id":"s1","title":"S","entries":[]},{"id":"s1","title":"T","entries":[]}]}`,
		"entry no id":        `{"version":"1","status":"draft","sections":[{"id":"s1","title":"S","entries":[{"title":"E","blocks":[` + paragraph + `]}]}]}`,
		"entry no title":     `{"version":"1","status":"draft","sections":[{"id":"s1","title":"S","entries":[{"id":"e1","blocks":[` + paragraph + `]}]}]}`,
		"empty blocks":       fmt.Sprintf(handbookFixture, "", ""),
		"duplicate entry id": `{"version":"1","status":"draft","sections":[{"id":"s1","title":"S","entries":[{"id":"e1","title":"E","blocks":[` + paragraph + `]}]},{"id":"s2","title":"T","entries":[{"id":"e1","title":"F","blocks":[` + paragraph + `]}]}]}`,
		"unknown block type": fmt.Sprintf(handbookFixture, `{"type":"quote","text":"q"}`, ""),
		"empty paragraph":    fmt.Sprintf(handbookFixture, `{"type":"paragraph","text":" "}`, ""),
		"heading with items": fmt.Sprintf(handbookFixture, `{"type":"heading","text":"h","items":["x"]}`, ""),
		"empty list":         fmt.Sprintf(handbookFixture, `{"type":"list","items":[]}`, ""),
		"empty list item":    fmt.Sprintf(handbookFixture, `{"type":"list","items":["a",""]}`, ""),
		"unknown caveat":     fmt.Sprintf(handbookFixture, `{"type":"caveat","key":"made_up_caveat"}`, ""),
		"caveat no key":      fmt.Sprintf(handbookFixture, `{"type":"caveat"}`, ""),
		"caveat typed text":  fmt.Sprintf(handbookFixture, `{"type":"caveat","key":"heuristic_ta_caveat","text":"retyped"}`, ""),
		"term no definition": fmt.Sprintf(handbookFixture, paragraph, `{"term":"RVOL"}`),
		"term empty synonym": fmt.Sprintf(handbookFixture, paragraph, `{"term":"RVOL","synonyms":[""],"definition":"d"}`),
	} {
		if _, err := loadFixtures(t, body, validMasterClass()); err == nil {
			t.Errorf("handbook %s: loaded, want a startup error", name)
		}
	}
}

func TestLoadEducation_RefusesMalformedMasterClass(t *testing.T) {
	for name, body := range map[string]string{
		"not json":           `[]`,
		"no version":         `{"status":"draft","modules":[{"id":"m1","title":"M","entries":[]}]}`,
		"no modules":         `{"version":"1","status":"draft"}`,
		"unknown field":      `{"version":"1","status":"draft","modules":[{"id":"m1","title":"M","entries":[],"extra":true}]}`,
		"duplicate module":   `{"version":"1","status":"draft","modules":[{"id":"m1","title":"M","entries":[]},{"id":"m1","title":"N","entries":[]}]}`,
		"no summary":         `{"version":"1","status":"draft","modules":[{"id":"m1","title":"M","entries":[{"id":"r1","title":"R","blocks":[` + paragraph + `]}]}]}`,
		"blank summary":      `{"version":"1","status":"draft","modules":[{"id":"m1","title":"M","entries":[{"id":"r1","title":"R","summary":"  ","blocks":[` + paragraph + `]}]}]}`,
		"empty blocks":       fmt.Sprintf(masterClassFixture, "", ""),
		"duplicate entry id": `{"version":"1","status":"draft","modules":[{"id":"m1","title":"M","entries":[{"id":"r1","title":"R","summary":"s","blocks":[` + paragraph + `]},{"id":"r1","title":"S","summary":"s","blocks":[` + paragraph + `]}]}]}`,
		"caveat block":       fmt.Sprintf(masterClassFixture, `{"type":"caveat","key":"heuristic_ta_caveat"}`, ""),
		"unknown block type": fmt.Sprintf(masterClassFixture, `{"type":"image","text":"x"}`, ""),
		"bad term":           fmt.Sprintf(masterClassFixture, paragraph, `{"definition":"no term"}`),
	} {
		if _, err := loadFixtures(t, validHandbook(), body); err == nil {
			t.Errorf("masterclass %s: loaded, want a startup error", name)
		}
	}
}

// Spec §4: a Handbook caveat is the shared constant, read from the caveats
// file independently of LoadCaveats, byte for byte.
func TestEducation_HandbookCaveatIsTheSharedConstantByteForByte(t *testing.T) {
	file, _ := readSharedJSON(t, "momentum_caveats.json")
	want, _ := file["heuristic_ta_caveat"].(string)
	if want == "" {
		t.Fatal("shared file has no heuristic_ta_caveat")
	}
	rec := get(t, newTestServer(t, fixtureStore(), freshNow), "/api/v1/education/handbook")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body)
	}
	body := decode(t, rec)
	var found int
	for _, s := range body["sections"].([]any) {
		for _, e := range s.(map[string]any)["entries"].([]any) {
			for _, b := range e.(map[string]any)["blocks"].([]any) {
				blk := b.(map[string]any)
				if blk["type"] != "caveat" {
					continue
				}
				if blk["key"] == "heuristic_ta_caveat" {
					found++
					if blk["text"] != want {
						t.Errorf("caveat text = %q\nshared     = %q", blk["text"], want)
					}
				}
			}
		}
	}
	if found == 0 {
		t.Fatal("the served Handbook has no heuristic_ta_caveat block (Stock Detail §1.3 must carry one)")
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"text":`+mustJSON(t, want))) {
		t.Error("the caveat's encoded bytes differ from a plain encoding of the shared text")
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := marshalContent(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestEducation_EveryCaveatKeyResolves(t *testing.T) {
	c := loadSharedCaveats(t)
	for key, want := range map[string]string{
		"evidence_caveat": c.Evidence, "research_score_caveat": c.ResearchScore, "heuristic_ta_caveat": c.HeuristicTA,
	} {
		e, err := loadFixtures(t, fmt.Sprintf(handbookFixture, `{"type":"caveat","key":"`+key+`"}`, ""), validMasterClass())
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		var doc handbookDoc
		if err := json.Unmarshal(e.handbook, &doc); err != nil {
			t.Fatal(err)
		}
		if b := doc.Sections[0].Entries[0].Blocks[0]; b.Key != key || b.Text != want {
			t.Errorf("%s resolved to key=%q text=%q", key, b.Key, b.Text)
		}
	}
}

// The served Handbook is the file with each caveat block's text added —
// nothing else added, dropped or reordered in meaning.
func TestEducation_HandbookServedAsAuthoredWithCaveatsResolved(t *testing.T) {
	want, _ := readSharedJSON(t, "handbook.json")
	rec := get(t, newTestServer(t, fixtureStore(), freshNow), "/api/v1/education/handbook")
	got := decode(t, rec)
	for _, s := range got["sections"].([]any) {
		for _, e := range s.(map[string]any)["entries"].([]any) {
			for _, b := range e.(map[string]any)["blocks"].([]any) {
				if blk := b.(map[string]any); blk["type"] == "caveat" {
					delete(blk, "text")
				}
			}
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Error("served Handbook (caveat text removed) differs from shared/content/handbook.json")
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(`\u0026`)) {
		t.Error("authored \"&\" must be served unescaped")
	}
}

func TestEducation_MasterClassServedAsAuthored(t *testing.T) {
	_, raw := readSharedJSON(t, "masterclass.json")
	var want bytes.Buffer
	if err := json.Compact(&want, raw); err != nil {
		t.Fatal(err)
	}
	rec := get(t, newTestServer(t, fixtureStore(), freshNow), "/api/v1/education/masterclass")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body)
	}
	if !bytes.Equal(rec.Body.Bytes(), want.Bytes()) {
		t.Error("response body differs from the checked-in masterclass.json")
	}
}

func TestEducation_GlossaryOfTheSharedFilesIsEmptyToday(t *testing.T) {
	rec := get(t, newTestServer(t, fixtureStore(), freshNow), "/api/v1/education/glossary")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body)
	}
	if got := rec.Body.String(); got != `{"handbook_version":"0.1.0","masterclass_version":"0.1.0","terms":[]}` {
		t.Errorf("glossary = %s (every `terms` is empty today, so terms must be [] not null)", got)
	}
}

func TestEducation_HeadersAndNoWritePath(t *testing.T) {
	srv := newTestServer(t, fixtureStore(), freshNow)
	for _, path := range []string{"/api/v1/education/handbook", "/api/v1/education/masterclass", "/api/v1/education/glossary"} {
		rec := get(t, srv, path)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", path, rec.Code)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=86400" {
			t.Errorf("%s: Cache-Control = %q, want a 24h cache", path, cc)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("%s: Content-Type = %q", path, ct)
		}
		if !json.Valid(rec.Body.Bytes()) {
			t.Errorf("%s: body is not JSON", path)
		}
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
			if rec := request(t, srv, method, path); rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d, want 405", method, path, rec.Code)
			}
		}
	}
}

func TestEducation_NotLoadedIs500(t *testing.T) {
	srv := NewServer(Config{Store: fixtureStore(), Caveats: loadSharedCaveats(t),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if rec := get(t, srv, "/api/v1/education/handbook"); rec.Code != http.StatusInternalServerError {
		t.Errorf("status %d, want 500 on a miswired server", rec.Code)
	}
}

// Spec §0 / §5 step 6: the checked-in MasterClass contains none of this app's
// own field names or labels.
func TestMasterClassPurity_SharedFilePasses(t *testing.T) {
	_, raw := readSharedJSON(t, "masterclass.json")
	if err := checkMasterClassPurity(raw); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadMasterClass(sharedContentPath(t, "masterclass.json")); err != nil {
		t.Fatal(err)
	}
}

func TestMasterClassPurity_AppLabelFailsToLoad(t *testing.T) {
	body := fmt.Sprintf(masterClassFixture, `{"type":"paragraph","text":"A BUY_WATCH label means buyers stepped in."}`, "")
	_, err := loadFixtures(t, validHandbook(), body)
	if err == nil || !strings.Contains(err.Error(), "BUY_WATCH") {
		t.Fatalf("err = %v, want a startup error naming BUY_WATCH", err)
	}
}

func TestMasterClassPurity_EveryBannedTermIsCaughtAnywhere(t *testing.T) {
	for _, term := range masterClassBannedTerms {
		for where, body := range map[string]string{
			"paragraph": fmt.Sprintf(masterClassFixture, `{"type":"paragraph","text":"see (`+term+`), here"}`, ""),
			"list item": fmt.Sprintf(masterClassFixture, `{"type":"list","items":["ok",`+mustJSON(t, term+" at the start")+`]}`, ""),
			"summary":   strings.Replace(validMasterClass(), `"summary":"short"`, `"summary":"ends with `+term+`"`, 1),
			"term":      fmt.Sprintf(masterClassFixture, paragraph, `{"term":"`+term+`","definition":"d"}`),
		} {
			if _, _, err := loadMasterClass(writeFixture(t, t.TempDir(), "mc.json", body)); err == nil {
				t.Errorf("%q in a %s: loaded, want a startup error", term, where)
			}
		}
	}
}

// Plain English and look-alike tokens are not this app's labels.
func TestMasterClassPurity_PlainEnglishIsAllowed(t *testing.T) {
	for _, text := range []string{
		"Traders sometimes take a time out after a losing streak.",
		"Chart patterns are heuristics; a heuristic is a rule of thumb.",
		"Heuristic signals are debated.",
		"A momentum score, a stop loss, an exit reason, a breakout from consolidation.",
		"Buy, watch, trim: plain verbs.",
		"rvol_200 and my_stop_atr_x are not this app's names.",
		"The order timed out.",
	} {
		body := fmt.Sprintf(masterClassFixture, `{"type":"paragraph","text":`+mustJSON(t, text)+`}`, "")
		if _, _, err := loadMasterClass(writeFixture(t, t.TempDir(), "mc.json", body)); err != nil {
			t.Errorf("%q: %v", text, err)
		}
	}
}

// Handbook is platform-specific: the app's own labels are expected there.
func TestMasterClassPurity_DoesNotApplyToHandbook(t *testing.T) {
	body := fmt.Sprintf(handbookFixture, `{"type":"paragraph","text":"BUY_WATCH and momentum_score_100"}`, "")
	if _, err := loadFixtures(t, body, validMasterClass()); err != nil {
		t.Fatal(err)
	}
}

func TestGlossary_ExtractedFromBothFilesAndAlphabetised(t *testing.T) {
	hb := fmt.Sprintf(handbookFixture, paragraph,
		`{"term":"RVOL","synonyms":["relative volume"],"definition":"Volume vs its 20-day average."},{"term":"breakout state","definition":"Where price sits vs resistance."}`)
	mc := fmt.Sprintf(masterClassFixture, paragraph,
		`{"term":"P/E ratio","synonyms":["PE ratio","price to earnings"],"definition":"Price over earnings per share."},{"term":"ATR","definition":"Average true range."}`)
	e, err := loadFixtures(t, hb, mc)
	if err != nil {
		t.Fatal(err)
	}
	var got glossaryResponse
	if err := json.Unmarshal(e.glossary, &got); err != nil {
		t.Fatal(err)
	}
	want := glossaryResponse{HandbookVersion: "1", MasterClassVersion: "1", Terms: []GlossaryTerm{
		{Term: "ATR", Synonyms: []string{}, Definition: "Average true range.", Source: "masterclass", EntryID: "r1", SectionOrModuleID: "m1"},
		{Term: "breakout state", Synonyms: []string{}, Definition: "Where price sits vs resistance.", Source: "handbook", EntryID: "e1", SectionOrModuleID: "s1"},
		{Term: "P/E ratio", Synonyms: []string{"PE ratio", "price to earnings"}, Definition: "Price over earnings per share.", Source: "masterclass", EntryID: "r1", SectionOrModuleID: "m1"},
		{Term: "RVOL", Synonyms: []string{"relative volume"}, Definition: "Volume vs its 20-day average.", Source: "handbook", EntryID: "e1", SectionOrModuleID: "s1"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("glossary =\n%+v\nwant\n%+v", got, want)
	}
	if !bytes.Contains(e.glossary, []byte(`"synonyms":[]`)) {
		t.Error("a term without synonyms must carry synonyms: [], not null or absent")
	}
}

func TestGlossary_DuplicateTermsFailStartup(t *testing.T) {
	cases := map[string][2]string{
		"across files, case-insensitive": {
			fmt.Sprintf(handbookFixture, paragraph, `{"term":"RSI","definition":"this app's RSI"}`),
			fmt.Sprintf(masterClassFixture, paragraph, `{"term":"rsi","definition":"RSI in general"}`),
		},
		"within one entry": {
			fmt.Sprintf(handbookFixture, paragraph, `{"term":"ATR","definition":"a"},{"term":"ATR ","definition":"b"}`),
			validMasterClass(),
		},
	}
	for name, c := range cases {
		if _, err := loadFixtures(t, c[0], c[1]); err == nil || !strings.Contains(err.Error(), "defined twice") {
			t.Errorf("%s: err = %v, want a duplicate-term startup error", name, err)
		}
	}
}

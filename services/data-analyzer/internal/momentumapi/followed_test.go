package momentumapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

type fakeFollow struct {
	classes  map[string]store.SymbolClass
	followed []store.FollowedRow
	computed []store.ComputedRow
	requests []string
	stopped  []string
}

func (f *fakeFollow) ListFollowed(context.Context) ([]store.FollowedRow, error) {
	return f.followed, nil
}
func (f *fakeFollow) ClassifySymbol(_ context.Context, s string) (store.SymbolClass, bool, error) {
	c, ok := f.classes[s]
	if ok && c.AssetType == "other" {
		return c, true, store.ErrNotComputable
	}
	return c, ok, nil
}
func (f *fakeFollow) AddFollowed(_ context.Context, c store.SymbolClass) (bool, error) {
	for _, r := range f.followed {
		if r.Symbol == c.Symbol {
			return false, nil
		}
	}
	f.followed = append(f.followed, store.FollowedRow{Symbol: c.Symbol, AssetType: c.AssetType, Listing: c.Listing, Source: "user"})
	return true, nil
}
func (f *fakeFollow) RemoveFollowed(_ context.Context, s string) (bool, error) {
	for i, r := range f.followed {
		if r.Symbol == s {
			f.followed = append(f.followed[:i], f.followed[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}
func (f *fakeFollow) SearchDirectory(_ context.Context, q string, _ int) ([]store.DirectoryMatch, error) {
	return []store.DirectoryMatch{{Symbol: strings.ToUpper(q), AssetType: "etf", Source: "finnhub_us"}}, nil
}
func (f *fakeFollow) ComputedSymbols(context.Context) ([]store.ComputedRow, error) {
	return f.computed, nil
}
func (f *fakeFollow) RequestCompute(_ context.Context, c store.SymbolClass) (bool, error) {
	f.requests = append(f.requests, c.Symbol)
	return true, nil
}
func (f *fakeFollow) StopCompute(_ context.Context, s string) (bool, error) {
	f.stopped = append(f.stopped, s)
	return true, nil
}

func newFollowServer(t *testing.T, fs FollowStore, now time.Time) *Server {
	t.Helper()
	srv := newTestServer(t, &fakeStore{}, now)
	srv.cfg.Follow = fs
	srv.cfg.ComputeDataTimeout = 30 * time.Minute
	return srv
}

func do(t *testing.T, srv *Server, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestComputeState(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { v := now.Add(d); return &v }
	errText := func(s string) *string { return &s }
	cases := []struct {
		name string
		row  store.ComputedRow
		want string
	}{
		{"automatic, never computed", store.ComputedRow{AssetType: "equity"}, computeScheduled},
		{"automatic, computed", store.ComputedRow{AssetType: "equity", ComputedAt: at(-time.Hour)}, computeComputed},
		{"automatic, compute failed", store.ComputedRow{AssetType: "equity", LastError: errText("compute: boom")}, computeFailed},
		{"manual, just queued", store.ComputedRow{AssetType: "equity", QueuedSince: at(-5 * time.Minute)}, computeWaitingForData},
		{"manual, past timeout", store.ComputedRow{AssetType: "equity", QueuedSince: at(-31 * time.Minute)}, computeDataNotArrived},
		{"manual, old computation does not count", store.ComputedRow{AssetType: "equity", QueuedSince: at(-5 * time.Minute), ComputedAt: at(-time.Hour)}, computeWaitingForData},
		{"manual equity, bars only", store.ComputedRow{AssetType: "equity", QueuedSince: at(-5 * time.Minute), BarsFetchedAt: at(-time.Minute)}, computeWaitingForData},
		{"manual equity, data in", store.ComputedRow{AssetType: "equity", QueuedSince: at(-5 * time.Minute), BarsFetchedAt: at(-time.Minute), FundamentalsFetchedAt: at(-time.Minute)}, computeComputing},
		{"manual crypto, bars in", store.ComputedRow{AssetType: "crypto", QueuedSince: at(-5 * time.Minute), BarsFetchedAt: at(-time.Minute)}, computeComputing},
		{"manual, data in, compute failed", store.ComputedRow{AssetType: "crypto", QueuedSince: at(-5 * time.Minute), BarsFetchedAt: at(-time.Minute), LastError: errText("compute: boom")}, computeFailed},
		{"manual, fetch error is not a compute failure", store.ComputedRow{AssetType: "crypto", QueuedSince: at(-5 * time.Minute), BarsFetchedAt: at(-time.Minute), LastError: errText("bars: no data")}, computeComputing},
		{"watchlist addition, just queued", store.ComputedRow{AssetType: "equity", Reasons: []string{"watchlist"}, QueuedSince: at(-2 * time.Minute)}, computeWaitingForData},
		{"watchlist addition, computed", store.ComputedRow{AssetType: "equity", Reasons: []string{"watchlist"}, QueuedSince: at(-10 * time.Minute), ComputedAt: at(-time.Minute)}, computeComputed},
		{"manual, computed", store.ComputedRow{AssetType: "equity", QueuedSince: at(-5 * time.Minute), ComputedAt: at(-time.Minute)}, computeComputed},
	}
	for _, c := range cases {
		if got := computeState(c.row, now, 30*time.Minute); got != c.want {
			t.Errorf("%s: state = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestFollowedEndpoints(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	fs := &fakeFollow{classes: map[string]store.SymbolClass{
		"SPY":  {Symbol: "SPY", AssetType: "etf", Listing: "us"},
		"WARR": {Symbol: "WARR", AssetType: "other", Listing: "us"},
	}}
	srv := newFollowServer(t, fs, now)

	if rec := do(t, srv, http.MethodPut, "/api/v1/followed-symbols/spy"); rec.Code != http.StatusCreated {
		t.Fatalf("follow: %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, srv, http.MethodPut, "/api/v1/followed-symbols/SPY"); rec.Code != http.StatusOK {
		t.Errorf("re-follow: %d, want 200", rec.Code)
	}
	if rec := do(t, srv, http.MethodPut, "/api/v1/followed-symbols/NOPE"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown: %d, want 404", rec.Code)
	}
	if rec := do(t, srv, http.MethodPut, "/api/v1/followed-symbols/WARR"); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("not computable: %d, want 422", rec.Code)
	}
	rec := do(t, srv, http.MethodGet, "/api/v1/followed-symbols")
	var list followedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Items) != 1 || list.Items[0].Symbol != "SPY" {
		t.Fatalf("list: %v %s", err, rec.Body)
	}
	rec = do(t, srv, http.MethodDelete, "/api/v1/followed-symbols/SPY")
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || rec.Code != http.StatusOK || len(list.Items) != 0 {
		t.Fatalf("unfollow: %d %s", rec.Code, rec.Body)
	}
}

func TestComputeEndpoints(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	since := now.Add(-5 * time.Minute)
	fs := &fakeFollow{
		classes:  map[string]store.SymbolClass{"SPY": {Symbol: "SPY", AssetType: "etf", Listing: "us"}},
		computed: []store.ComputedRow{{Symbol: "SPY", AssetType: "etf", Reasons: []string{"manual"}, ManualSince: &since, QueuedSince: &since}},
	}
	srv := newFollowServer(t, fs, now)

	rec := do(t, srv, http.MethodPut, "/api/v1/computed-symbols/SPY")
	if rec.Code != http.StatusAccepted || len(fs.requests) != 1 {
		t.Fatalf("compute: %d %s", rec.Code, rec.Body)
	}
	var resp computedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.DataTimeoutMinutes != 30 || len(resp.Items) != 1 || resp.Items[0].State != computeWaitingForData ||
		resp.Items[0].ManualRequestedAt == nil {
		t.Errorf("computed response = %+v", resp)
	}
	if rec := do(t, srv, http.MethodDelete, "/api/v1/computed-symbols/SPY"); rec.Code != http.StatusOK || len(fs.stopped) != 1 {
		t.Errorf("stop: %d", rec.Code)
	}
	if rec := do(t, srv, http.MethodGet, "/api/v1/symbols/directory?q=gld"); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"symbol":"GLD"`) {
		t.Errorf("directory: %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, srv, http.MethodGet, "/api/v1/symbols/directory?q="); rec.Code != http.StatusBadRequest {
		t.Errorf("empty query: %d, want 400", rec.Code)
	}
}

func TestFollowEndpointsWithoutStore(t *testing.T) {
	srv := newTestServer(t, &fakeStore{}, time.Now())
	if rec := do(t, srv, http.MethodGet, "/api/v1/followed-symbols"); rec.Code != http.StatusNotImplemented {
		t.Errorf("no store: %d, want 501", rec.Code)
	}
}

func TestBuildCashFlow(t *testing.T) {
	cf := buildCashFlow(store.AnalysisInputs{CashFlow: &store.CashFlowStatement{
		Form: "10-K", EndDate: "2025-12-31", FiledDate: "2026-02-10",
		Lines: map[string]float64{
			"NetCashProvidedByUsedInOperatingActivities": 100,
			"PaymentsToAcquireProductiveAssets":          20,
			"PaymentsForRepurchaseOfCommonStock":         5,
		},
	}})
	if !cf.Available || cf.UnavailableReason != nil || len(cf.Lines) != len(cashFlowLines) {
		t.Fatalf("available statement: %+v", cf)
	}
	vals := map[string]*float64{}
	for _, l := range cf.Lines {
		vals[l.Key] = l.Value
	}
	if *vals["operating"] != 100 || *vals["capex"] != 20 || *vals["buybacks"] != 5 || vals["dividends"] != nil {
		t.Errorf("lines = %+v", cf.Lines)
	}

	s := func(v string) *string { return &v }
	for name, c := range map[string]struct {
		cov  store.StatementCoverage
		want string
	}{
		"crypto":        {store.StatementCoverage{AssetType: s("crypto")}, "crypto"},
		"etf":           {store.StatementCoverage{AssetType: s("etf")}, "funds"},
		"foreign":       {store.StatementCoverage{AssetType: s("equity"), Listing: s("foreign")}, "foreign listing"},
		"none returned": {store.StatementCoverage{AssetType: s("equity"), Status: s("none_returned")}, "20-F"},
		"not fetched":   {store.StatementCoverage{}, "no filings are stored"},
	} {
		out := buildCashFlow(store.AnalysisInputs{Coverage: c.cov})
		if out.Available || out.UnavailableReason == nil || !strings.Contains(*out.UnavailableReason, c.want) {
			t.Errorf("%s: %+v", name, out)
		}
	}
}

// A 20-F filer's statement: reporting currency named, source labelled, a line
// the filer tags with its own concept reads "not reported as a comparable
// line", and a newer 20-F missing from companyfacts is surfaced.
func TestBuildCashFlow_20F(t *testing.T) {
	cf := &store.IFRSCashFlow{
		Form: "20-F", Filed: "2025-04-17", PeriodEnd: "2024-12-31", FiscalYear: 2024, Currency: "TWD",
		Lines: map[string]float64{"operating": 1826177100000, "investing": -864842800000, "financing": -346301000000,
			"capex": 956006500000, "dividends": 363055200000},
	}
	cf.Latest20F = &struct {
		Filed     string `json:"filed"`
		PeriodEnd string `json:"period_end"`
	}{Filed: "2026-04-16", PeriodEnd: "2025-12-31"}
	out := buildCashFlow(store.AnalysisInputs{CashFlow20F: cf})
	if !out.Available || *out.Currency != "TWD" || *out.Source != "20-F via SEC EDGAR" || *out.FiscalYear != 2024 || *out.Form != "20-F" {
		t.Fatalf("20-F card = %+v", out)
	}
	if out.NewerFiling == nil || out.NewerFiling.Filed != "2026-04-16" || out.NewerFiling.PeriodEnd != "2025-12-31" {
		t.Errorf("newer filing = %+v", out.NewerFiling)
	}
	for _, l := range out.Lines {
		switch l.Key {
		case "buybacks":
			if l.Value != nil || l.MissingNote == nil || *l.MissingNote != "not reported as a comparable line" {
				t.Errorf("buybacks = %+v", l)
			}
		case "operating":
			if l.Value == nil || *l.Value != 1826177100000 || l.MissingNote != nil {
				t.Errorf("operating = %+v", l)
			}
		}
	}
	cf.Latest20F.PeriodEnd = "2024-12-31"
	if out := buildCashFlow(store.AnalysisInputs{CashFlow20F: cf}); out.NewerFiling != nil {
		t.Errorf("the shown filing is the newest: no notice, got %+v", out.NewerFiling)
	}
	us := buildCashFlow(store.AnalysisInputs{CashFlow: &store.CashFlowStatement{Form: "10-K", EndDate: "2025-12-31", Lines: map[string]float64{}}})
	if *us.Currency != "USD" || *us.Source != "10-K via Finnhub" || *us.FiscalYear != 2025 || *us.Lines[0].MissingNote != "not in filing" {
		t.Errorf("10-K card = %+v", us)
	}
}

// Package tools registers svc-mcp's read-only MCP tools. Every tool maps to
// one GET on momentum-api; inputs are validated here before any URL is built.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/konsbe/trading-agent/services/svc-mcp/internal/momentum"
)

// MaxResultBytes caps a tool result; larger replies ask Claude to narrow the
// query instead of being cut mid-JSON.
const MaxResultBytes = 100 << 10

const framing = "Read-only view of the trading-agent research platform. It describes stored data; it does not predict prices and is not trading advice."

var (
	alertTypeRe  = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
	handbookIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,79}$`)
	errorCodeRe  = regexp.MustCompile(`^[a-z][a-z0-9_]{0,59}$`)
)

// validSymbol matches momentum-api's own symbol check (bars.go).
func validSymbol(s string) bool {
	if len(s) == 0 || len(s) > 15 {
		return false
	}
	for _, r := range s {
		if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-') {
			return false
		}
	}
	return true
}

type Getter interface {
	Get(ctx context.Context, path string, q url.Values) (momentum.Response, error)
}

type deps struct {
	api     Getter
	caveats Caveats
	sleep   func(context.Context, time.Duration) error
}

// Instructions is sent in the initialize result, ahead of any tool call.
func Instructions(c Caveats) string {
	return framing + "\n\nScreener: " + c.Evidence + "\n\nResearch score: " + c.ResearchScore + "\n\nPattern and action signals: " + c.HeuristicTA
}

func NewServer(api Getter, c Caveats) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "trading-agent", Title: "trading-agent (read-only)", Version: "0.1.0"},
		&mcp.ServerOptions{Instructions: Instructions(c)})
	Register(s, api, c, sleepCtx)
	return s
}

func Register(s *mcp.Server, api Getter, c Caveats, sleep func(context.Context, time.Duration) error) {
	d := &deps{api: api, caveats: c, sleep: sleep}
	ro := &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: boolPtr(false), DestructiveHint: boolPtr(false)}
	add := func(name, title, desc string) *mcp.Tool {
		return &mcp.Tool{Name: name, Title: title, Description: framing + " " + desc, Annotations: ro}
	}

	mcp.AddTool(s, add("get_market_report", "Daily market report",
		"Today's market report: breadth, regime, sector and macro context, each with its descriptive text."), d.marketReport)
	mcp.AddTool(s, add("list_alerts", "Alerts",
		"Alert history from the analyst bot. mode=raw lists individual alerts newest first; mode=grouped returns one row per symbol and alert type with count and first/last fired. "+
			"Filters combine. Page with before (an alert id from the previous page). "+c.HeuristicTA), d.alerts)
	mcp.AddTool(s, add("get_candidates", "Today's candidates",
		"The latest momentum scan: candidates that met the published gates, with gate values and the research score. "+c.Evidence+" "+c.ResearchScore), d.candidates)
	mcp.AddTool(s, add("get_candidate", "One candidate",
		"One symbol's row from the latest momentum scan (404 if it was not a candidate). "+c.Evidence+" "+c.ResearchScore), d.candidate)
	mcp.AddTool(s, add("get_symbol_analysis", "Symbol analysis",
		"Full analysis of one symbol: technicals, fundamentals, balance sheet, correlations, heuristic chart patterns and the BUY/TRIM action signal, and the latest cash-flow statement. "+
			"Computed on demand if stale; may report status computing. "+c.HeuristicTA), d.analysis)
	mcp.AddTool(s, add("get_watchlist", "Watchlist",
		"The user's watchlist with each symbol's latest scan context. Read-only: this tool cannot add or remove symbols."), d.watchlist)
	mcp.AddTool(s, add("get_tracked_positions", "Tracked positions",
		"Positions the momentum tracker follows after a candidate alert, with entry reference, exit reason and its note. status: active (default), closed or all. "+c.Evidence), d.tracked)
	mcp.AddTool(s, add("get_data_source_status", "Data-source status",
		"Health of the data feeds (Tiingo, Finnhub budgets and errors) and of the daily scan chain per session, including whether each session ran clean."), d.dataSources)
	mcp.AddTool(s, add("list_handbook_sections", "Handbook contents",
		"The Education Handbook's sections and entries (ids and titles). Use get_handbook_section with an id to read one."), d.handbookList)
	mcp.AddTool(s, add("get_handbook_section", "Handbook section",
		"One Handbook section, or one entry, by id (from list_handbook_sections). The Handbook explains what each screen shows and how it is computed."), d.handbookSection)
}

func boolPtr(b bool) *bool { return &b }

type noInput struct{}

type symbolInput struct {
	Symbol string `json:"symbol" jsonschema:"ticker, e.g. AAPL (letters, digits, '.' or '-', max 15)"`
}

type alertsInput struct {
	Mode      string   `json:"mode,omitempty" jsonschema:"raw (default) or grouped"`
	Symbol    string   `json:"symbol,omitempty" jsonschema:"only this ticker"`
	AlertType []string `json:"alert_type,omitempty" jsonschema:"only these alert types (e.g. rsi_oversold)"`
	Severity  []string `json:"severity,omitempty" jsonschema:"only these severities: info, notice, warning"`
	Since     string   `json:"since,omitempty" jsonschema:"inclusive start: YYYY-MM-DD (UTC) or RFC3339"`
	Until     string   `json:"until,omitempty" jsonschema:"exclusive end: YYYY-MM-DD (whole day, UTC) or RFC3339"`
	Before    int64    `json:"before,omitempty" jsonschema:"page: only alerts (or groups) older than this alert id"`
	Limit     int      `json:"limit,omitempty" jsonschema:"rows or groups, 1-100 (default 50)"`
}

type trackedInput struct {
	Status string `json:"status,omitempty" jsonschema:"active (default), closed or all"`
}

type handbookInput struct {
	ID string `json:"id" jsonschema:"section or entry id from list_handbook_sections"`
}

func (d *deps) marketReport(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
	return d.fetch(ctx, "/api/v1/market-report/today", nil)
}

func (d *deps) candidates(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
	return d.fetch(ctx, "/api/v1/scanner/today", nil)
}

func (d *deps) candidate(ctx context.Context, _ *mcp.CallToolRequest, in symbolInput) (*mcp.CallToolResult, any, error) {
	sym, err := symbol(in.Symbol)
	if err != nil {
		return nil, nil, err
	}
	return d.fetch(ctx, "/api/v1/scanner/today/"+sym, nil)
}

func (d *deps) watchlist(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
	return d.fetch(ctx, "/api/v1/watchlist", nil)
}

func (d *deps) dataSources(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
	return d.fetch(ctx, "/api/v1/data-sources/status", nil)
}

func (d *deps) tracked(ctx context.Context, _ *mcp.CallToolRequest, in trackedInput) (*mcp.CallToolResult, any, error) {
	st := strings.ToLower(strings.TrimSpace(in.Status))
	switch st {
	case "":
		st = "active"
	case "active", "closed", "all":
	default:
		return nil, nil, errors.New("status must be active, closed or all")
	}
	return d.fetch(ctx, "/api/v1/scanner/tracked", url.Values{"status": {st}})
}

func (d *deps) alerts(ctx context.Context, _ *mcp.CallToolRequest, in alertsInput) (*mcp.CallToolResult, any, error) {
	q := url.Values{}
	switch in.Mode {
	case "", "raw":
		q.Set("mode", "raw")
	case "grouped":
		q.Set("mode", "grouped")
	default:
		return nil, nil, errors.New("mode must be raw or grouped")
	}
	if in.Symbol != "" {
		sym, err := symbol(in.Symbol)
		if err != nil {
			return nil, nil, err
		}
		q.Set("symbol", sym)
	}
	if len(in.AlertType) > 20 || len(in.Severity) > 3 {
		return nil, nil, errors.New("too many filter values")
	}
	for _, t := range in.AlertType {
		if !alertTypeRe.MatchString(t) {
			return nil, nil, errors.New("alert_type values are lower-case identifiers such as rsi_oversold")
		}
		q.Add("alert_type", t)
	}
	for _, s := range in.Severity {
		if s != "info" && s != "notice" && s != "warning" {
			return nil, nil, errors.New("severity values are info, notice or warning")
		}
		q.Add("severity", s)
	}
	for key, v := range map[string]string{"since": in.Since, "until": in.Until} {
		if v == "" {
			continue
		}
		if !isDateOrTime(v) {
			return nil, nil, fmt.Errorf("%s must be YYYY-MM-DD or RFC3339", key)
		}
		q.Set(key, v)
	}
	if in.Before < 0 {
		return nil, nil, errors.New("before must be a positive alert id")
	}
	if in.Before > 0 {
		q.Set("before", strconv.FormatInt(in.Before, 10))
	}
	limit := in.Limit
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return nil, nil, errors.New("limit must be 1-100")
	}
	q.Set("limit", strconv.Itoa(limit))
	return d.fetch(ctx, "/api/v1/alerts", q)
}

// analysisWait bounds how long one tool call waits for an on-demand analysis.
const analysisWait = 20 * time.Second

func (d *deps) analysis(ctx context.Context, _ *mcp.CallToolRequest, in symbolInput) (*mcp.CallToolResult, any, error) {
	sym, err := symbol(in.Symbol)
	if err != nil {
		return nil, nil, err
	}
	path := "/api/v1/scanner/today/" + sym + "/analysis"
	deadline := time.Now().Add(analysisWait)
	for {
		resp, err := d.api.Get(ctx, path, nil)
		if err != nil {
			return nil, nil, upstreamErr(err)
		}
		if resp.Status != 202 {
			return result(resp)
		}
		var pending struct {
			RetryAfterMS int64 `json:"retry_after_ms"`
		}
		_ = json.Unmarshal(resp.Body, &pending)
		wait := min(max(time.Duration(pending.RetryAfterMS)*time.Millisecond, 500*time.Millisecond), 3*time.Second)
		if time.Now().Add(wait).After(deadline) {
			return text(resp.Body)
		}
		if err := d.sleep(ctx, wait); err != nil {
			return nil, nil, err
		}
	}
}

type handbookDoc struct {
	Sections []struct {
		ID      string            `json:"id"`
		Title   string            `json:"title"`
		Entries []json.RawMessage `json:"entries"`
	} `json:"sections"`
}

type entryHead struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func (d *deps) handbook(ctx context.Context) (handbookDoc, []byte, error) {
	resp, err := d.api.Get(ctx, "/api/v1/education/handbook", nil)
	if err != nil {
		return handbookDoc{}, nil, upstreamErr(err)
	}
	if resp.Status != 200 {
		return handbookDoc{}, nil, statusErr(resp)
	}
	var doc handbookDoc
	if err := json.Unmarshal(resp.Body, &doc); err != nil {
		return handbookDoc{}, nil, errors.New("handbook is unreadable")
	}
	return doc, resp.Body, nil
}

func (d *deps) handbookList(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
	doc, _, err := d.handbook(ctx)
	if err != nil {
		return nil, nil, err
	}
	type section struct {
		ID      string      `json:"id"`
		Title   string      `json:"title"`
		Entries []entryHead `json:"entries"`
	}
	out := struct {
		Sections []section `json:"sections"`
	}{Sections: []section{}}
	for _, s := range doc.Sections {
		sec := section{ID: s.ID, Title: s.Title, Entries: []entryHead{}}
		for _, raw := range s.Entries {
			var e entryHead
			if json.Unmarshal(raw, &e) == nil && e.ID != "" {
				sec.Entries = append(sec.Entries, e)
			}
		}
		out.Sections = append(out.Sections, sec)
	}
	return jsonResult(out)
}

func (d *deps) handbookSection(ctx context.Context, _ *mcp.CallToolRequest, in handbookInput) (*mcp.CallToolResult, any, error) {
	id := strings.TrimSpace(in.ID)
	if !handbookIDRe.MatchString(id) {
		return nil, nil, errors.New("id must be a Handbook id from list_handbook_sections")
	}
	_, body, err := d.handbook(ctx)
	if err != nil {
		return nil, nil, err
	}
	var full struct {
		Sections []map[string]json.RawMessage `json:"sections"`
	}
	if err := json.Unmarshal(body, &full); err != nil {
		return nil, nil, errors.New("handbook is unreadable")
	}
	for _, s := range full.Sections {
		var sid, title string
		_ = json.Unmarshal(s["id"], &sid)
		_ = json.Unmarshal(s["title"], &title)
		if sid == id {
			return jsonResult(s)
		}
		var entries []json.RawMessage
		_ = json.Unmarshal(s["entries"], &entries)
		for _, raw := range entries {
			var e entryHead
			if json.Unmarshal(raw, &e) == nil && e.ID == id {
				return jsonResult(map[string]any{"section_id": sid, "section_title": title, "entry": raw})
			}
		}
	}
	return nil, nil, fmt.Errorf("no Handbook section or entry with id %q", id)
}

func (d *deps) fetch(ctx context.Context, path string, q url.Values) (*mcp.CallToolResult, any, error) {
	resp, err := d.api.Get(ctx, path, q)
	if err != nil {
		return nil, nil, upstreamErr(err)
	}
	return result(resp)
}

func result(resp momentum.Response) (*mcp.CallToolResult, any, error) {
	if resp.Status < 200 || resp.Status > 299 {
		return nil, nil, statusErr(resp)
	}
	return text(resp.Body)
}

func text(body []byte) (*mcp.CallToolResult, any, error) {
	if len(body) > MaxResultBytes {
		return nil, nil, fmt.Errorf("result is %d KB, over the %d KB limit; narrow the query (limit, symbol, status)", len(body)>>10, MaxResultBytes>>10)
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(body)}}}, nil, nil
}

func jsonResult(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, nil, err
	}
	clean, err := momentum.Redact(b)
	if err != nil {
		return nil, nil, err
	}
	return text(clean)
}

// statusErr reports momentum-api's error code, never its raw body.
func statusErr(resp momentum.Response) error {
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(resp.Body, &e)
	if errorCodeRe.MatchString(e.Error) {
		return fmt.Errorf("platform returned %d: %s", resp.Status, e.Error)
	}
	return fmt.Errorf("platform returned %d", resp.Status)
}

// upstreamErr hides the internal URL that net/http errors carry.
func upstreamErr(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return errors.New("platform did not answer in time")
	}
	if errors.Is(err, momentum.ErrTooLarge) {
		return errors.New("platform reply too large")
	}
	return errors.New("platform unavailable")
}

func symbol(raw string) (string, error) {
	s := strings.ToUpper(strings.TrimSpace(raw))
	if !validSymbol(s) {
		return "", errors.New("symbol must be a ticker: letters, digits, '.' or '-', max 15")
	}
	return s, nil
}

func isDateOrTime(s string) bool {
	if _, err := time.Parse(time.DateOnly, s); err == nil {
		return true
	}
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

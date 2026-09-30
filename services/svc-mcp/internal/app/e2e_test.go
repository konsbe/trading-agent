package app_test

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/konsbe/trading-agent/services/svc-mcp/internal/app"
	"github.com/konsbe/trading-agent/services/svc-mcp/internal/config"
	"github.com/konsbe/trading-agent/services/svc-mcp/internal/momentum"
	"github.com/konsbe/trading-agent/services/svc-mcp/internal/tools"
)

const (
	ownerID       = int64(424242)
	strangerID    = int64(999)
	anthropicIP   = "160.79.104.10"
	homeIP        = "203.0.113.5"
	outsideIP     = "198.51.100.7"
	clientID      = "claude-test-client"
	clientSecret  = "test-client-secret-0123456789abcdefghij"
	ghSecret      = "gh-test-secret"
	plantedSecret = "PLANTED-SECRET-VALUE"
	claudeRedir   = "https://claude.ai/api/mcp/auth_callback"
)

// fakeGitHub signs in whichever user the test picks, and records every
// authorize request so the test can check the scopes asked for.
type fakeGitHub struct {
	srv       *httptest.Server
	mu        sync.Mutex
	userID    int64
	authorize []url.Values
	revoked   int
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	g := &fakeGitHub{userID: ownerID}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /login/oauth/authorize", func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		g.authorize = append(g.authorize, r.URL.Query())
		g.mu.Unlock()
		back := r.URL.Query().Get("redirect_uri") + "?" + url.Values{"code": {"gh-code"}, "state": {r.URL.Query().Get("state")}}.Encode()
		http.Redirect(w, r, back, http.StatusFound)
	})
	mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("client_secret") != ghSecret || r.PostForm.Get("code") != "gh-code" {
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "bad_verification_code"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "gh-token", "token_type": "bearer", "scope": ""})
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer gh-token" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		g.mu.Lock()
		id := g.userID
		g.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "login": "someone"})
	})
	mux.HandleFunc("DELETE /applications/{cid}/token", func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		g.revoked++
		g.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	g.srv = httptest.NewTLSServer(mux)
	t.Cleanup(g.srv.Close)
	return g
}

// fakeMomentum answers the routes the tools use; data-sources plants secrets
// the redaction must remove.
func fakeMomentum(t *testing.T) (*httptest.Server, *[]string) {
	var mu sync.Mutex
	var methods []string
	js := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		methods = append(methods, r.Method+" "+r.URL.Path)
		mu.Unlock()
		switch p := r.URL.Path; {
		case p == "/api/v1/market-report/today":
			js(w, 200, map[string]any{"as_of": "2026-09-29", "breadth": map[string]any{"pct_above_50dma": 41.2}})
		case p == "/api/v1/alerts":
			js(w, 200, map[string]any{"mode": r.URL.Query().Get("mode"), "limit": r.URL.Query().Get("limit"), "alerts": []any{}})
		case p == "/api/v1/scanner/today":
			js(w, 200, map[string]any{"session": "2026-09-29", "candidates": []any{map[string]any{"symbol": "CAAP", "score": 0.61}}})
		case p == "/api/v1/scanner/today/CAAP":
			js(w, 200, map[string]any{"symbol": "CAAP"})
		case p == "/api/v1/scanner/today/ZZZZ":
			js(w, 404, map[string]any{"error": "not_a_candidate"})
		case p == "/api/v1/scanner/today/CAAP/analysis":
			js(w, 200, map[string]any{"symbol": "CAAP", "heuristic_signals": map[string]any{"action_signal": map[string]any{"action": "TRIM_WATCH"}}, "cash_flow": map[string]any{"available": false}})
		case p == "/api/v1/watchlist":
			js(w, 200, map[string]any{"items": []any{}})
		case p == "/api/v1/scanner/tracked":
			js(w, 200, map[string]any{"status": r.URL.Query().Get("status"), "rows": []any{}})
		case p == "/api/v1/data-sources/status":
			js(w, 200, map[string]any{
				"providers": map[string]any{"tiingo": map[string]any{
					"last_error": "GET https://api.tiingo.com/tiingo/daily/X/prices?token=" + plantedSecret + "&x=1 failed",
					"api_key":    plantedSecret,
				}},
				"note": "Authorization: Bearer " + plantedSecret,
			})
		case p == "/api/v1/education/handbook":
			js(w, 200, map[string]any{"sections": []any{
				map[string]any{"id": "data-source", "title": "Data Source", "intro": "x", "entries": []any{
					map[string]any{"id": "data-source-chain-statuses", "title": "Chain statuses", "blocks": []any{}},
				}},
			}})
		default:
			js(w, 404, map[string]any{"error": "not_found"})
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &methods
}

// sourceIP sets CF-Connecting-IP as Cloudflare would; the test server's peer
// (127.0.0.1) is configured as the trusted proxy.
type sourceIP struct {
	ip   string
	base http.RoundTripper
}

func (s sourceIP) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("CF-Connecting-IP", s.ip)
	return s.base.RoundTrip(r)
}

type bearerRT struct {
	token string
	base  http.RoundTripper
}

func (b bearerRT) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(r)
}

type env struct {
	srv      *httptest.Server
	gh       *fakeGitHub
	logs     *strings.Builder
	methods  *[]string
	claude   *http.Client // server-to-server, from Anthropic's range
	browser  *http.Client // the user's browser, from home; does not follow redirects
	resource string
}

func setup(t *testing.T) *env {
	t.Helper()
	gh := newFakeGitHub(t)
	apiURL := os.Getenv("MCP_E2E_MOMENTUM_API")
	var methods *[]string
	if apiURL == "" {
		var srv *httptest.Server
		srv, methods = fakeMomentum(t)
		apiURL = srv.URL
	}
	caveats, err := tools.LoadCaveats("../../../../shared/content/momentum_caveats.json")
	if err != nil {
		t.Fatal(err)
	}
	logs := &strings.Builder{}
	var logMu sync.Mutex
	log := slog.New(slog.NewJSONHandler(lockedWriter{&logMu, logs}, nil))

	srv := httptest.NewUnstartedServer(nil)
	srv.StartTLS()
	t.Cleanup(srv.Close)
	cfg := config.Config{
		PublicURL: srv.URL, ClientID: clientID, ClientSecret: clientSecret,
		SigningKey: []byte("0123456789abcdef0123456789abcdef"), RedirectURIs: []string{claudeRedir},
		AccessTTL: time.Hour, RefreshTTL: 720 * time.Hour,
		GitHubClientID: "gh-client", GitHubClientSecret: ghSecret, GitHubUserID: ownerID,
		GitHubWebURL: gh.srv.URL, GitHubAPIURL: gh.srv.URL, GitHubHTTP: gh.srv.Client(),
		IPAllowlistEnable: true, IPAllowlist: []netip.Prefix{netip.MustParsePrefix("160.79.104.0/21")},
		TrustedProxies: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")},
		RatePerToken:   1000, RateGlobal: 1000, RateAuthPerIP: 1000,
	}
	srv.Config.Handler = app.Handler(cfg, momentum.New(apiURL), caveats, log, nil)

	tlsRT := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	return &env{
		srv: srv, gh: gh, logs: logs, methods: methods, resource: srv.URL + "/mcp",
		claude: &http.Client{Transport: sourceIP{anthropicIP, tlsRT}},
		browser: &http.Client{Transport: sourceIP{homeIP, tlsRT}, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
	}
}

type lockedWriter struct {
	mu *sync.Mutex
	b  *strings.Builder
}

func (l lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func getJSON(t *testing.T, c *http.Client, u string, out any) {
	t.Helper()
	resp, err := c.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET %s: %d", u, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatal(err)
	}
}

func pkcePair() (verifier, challenge string) {
	verifier = strings.Repeat("v", 20) + "-verifier-for-the-e2e-test-0123456789"
	h := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(h[:])
}

// signIn walks the browser leg: /authorize, GitHub, callback, back to Claude.
// It returns Claude's callback URL.
func (e *env) signIn(t *testing.T, authz string, challenge string) *url.URL {
	t.Helper()
	q := url.Values{
		"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {claudeRedir},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"}, "state": {"claude-state"},
		"scope": {"mcp:read"}, "resource": {e.resource},
	}
	resp, err := e.browser.Get(authz + "?" + q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("authorize: %d", resp.StatusCode)
	}
	cookies := resp.Cookies()
	// GitHub (fake) redirects straight back to the callback.
	resp, err = e.browser.Get(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	cb, _ := http.NewRequest(http.MethodGet, resp.Header.Get("Location"), nil)
	for _, c := range cookies {
		cb.AddCookie(c)
	}
	resp, err = e.browser.Do(cb)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("callback: %d", resp.StatusCode)
	}
	u, _ := url.Parse(resp.Header.Get("Location"))
	return u
}

func (e *env) token(t *testing.T, tokenURL string, form url.Values) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(url.QueryEscape(clientID), url.QueryEscape(clientSecret))
	resp, err := e.claude.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

func TestEndToEnd(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	// 1. Unauthenticated /mcp: 401 pointing at the protected-resource metadata.
	resp, err := e.claude.Post(e.resource, "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	wa := resp.Header.Get("WWW-Authenticate")
	if resp.StatusCode != 401 || !strings.Contains(wa, `resource_metadata="`+e.srv.URL+`/.well-known/oauth-protected-resource/mcp"`) {
		t.Fatalf("want 401 with resource_metadata, got %d %q", resp.StatusCode, wa)
	}

	// 2. Discovery, as Claude does it.
	var prm struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
	}
	getJSON(t, e.claude, e.srv.URL+"/.well-known/oauth-protected-resource/mcp", &prm)
	if prm.Resource != e.resource || len(prm.AuthorizationServers) != 1 {
		t.Fatalf("PRM: %+v", prm)
	}
	var asm struct {
		Issuer        string   `json:"issuer"`
		Authorization string   `json:"authorization_endpoint"`
		Token         string   `json:"token_endpoint"`
		PKCE          []string `json:"code_challenge_methods_supported"`
	}
	getJSON(t, e.claude, prm.AuthorizationServers[0]+"/.well-known/oauth-authorization-server", &asm)
	if asm.Issuer != e.srv.URL || !slices.Equal(asm.PKCE, []string{"S256"}) {
		t.Fatalf("AS metadata: %+v", asm)
	}

	// 3. Browser sign-in; GitHub is asked for no scope.
	verifier, challenge := pkcePair()
	cb := e.signIn(t, asm.Authorization, challenge)
	if cb.Scheme+"://"+cb.Host+cb.Path != claudeRedir || cb.Query().Get("state") != "claude-state" || cb.Query().Get("iss") != e.srv.URL {
		t.Fatalf("callback to Claude: %s", cb.Redacted())
	}
	code := cb.Query().Get("code")
	if code == "" {
		t.Fatalf("no code: %s", cb.Query().Get("error"))
	}
	for _, q := range e.gh.authorize {
		if _, ok := q["scope"]; ok {
			t.Fatalf("GitHub authorize asked for scope %q", q.Get("scope"))
		}
	}
	if e.gh.revoked == 0 {
		t.Fatal("GitHub token was not revoked after the id lookup")
	}

	// 4. Token exchange: wrong verifier fails and burns nothing; right one works once.
	st, _ := e.token(t, asm.Token, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {claudeRedir}, "code_verifier": {strings.Repeat("x", 43)}})
	if st != 400 {
		t.Fatalf("wrong verifier: %d", st)
	}
	// The code is single-use even after a failed attempt: sign in again.
	code = e.signIn(t, asm.Authorization, challenge).Query().Get("code")
	st, tok := e.token(t, asm.Token, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {claudeRedir}, "code_verifier": {verifier}})
	if st != 200 {
		t.Fatalf("token: %d %v", st, tok)
	}
	if st2, _ := e.token(t, asm.Token, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {claudeRedir}, "code_verifier": {verifier}}); st2 != 400 {
		t.Fatalf("code reuse: %d", st2)
	}
	access, _ := tok["access_token"].(string)
	refresh, _ := tok["refresh_token"].(string)

	// 5. Refresh.
	st, tok2 := e.token(t, asm.Token, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}})
	if st != 200 || tok2["access_token"] == "" {
		t.Fatalf("refresh: %d", st)
	}
	if st, _ := e.token(t, asm.Token, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {access}}); st != 400 {
		t.Fatalf("access token accepted as refresh token: %d", st)
	}

	// 6. MCP session with the token: every tool, caveats, redaction.
	cs := e.connect(t, ctx, access)
	defer cs.Close()
	if ins := cs.InitializeResult().Instructions; !strings.Contains(ins, "SCREENER, not a forecast") || !strings.Contains(ins, "not a demonstrated edge") {
		t.Fatalf("instructions miss the caveats: %q", ins)
	}
	lt, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"get_candidate", "get_candidates", "get_data_source_status", "get_handbook_section", "get_market_report",
		"get_symbol_analysis", "get_tracked_positions", "get_watchlist", "list_alerts", "list_handbook_sections"}
	var names []string
	for _, tl := range lt.Tools {
		names = append(names, tl.Name)
		if tl.Annotations == nil || !tl.Annotations.ReadOnlyHint {
			t.Errorf("%s is not marked read-only", tl.Name)
		}
		if !strings.Contains(tl.Description, "does not predict") {
			t.Errorf("%s description lacks the framing", tl.Name)
		}
	}
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Fatalf("tools: %v", names)
	}
	for _, n := range []string{"get_candidates", "get_candidate", "get_tracked_positions"} {
		if d := toolDesc(lt, n); !strings.Contains(d, "SCREENER, not a forecast") {
			t.Errorf("%s lacks the evidence caveat", n)
		}
	}
	for _, n := range []string{"list_alerts", "get_symbol_analysis"} {
		if d := toolDesc(lt, n); !strings.Contains(d, "not a demonstrated edge") {
			t.Errorf("%s lacks the heuristic-signals caveat", n)
		}
	}

	calls := []struct {
		name string
		args map[string]any
	}{
		{"get_market_report", nil},
		{"list_alerts", map[string]any{"mode": "grouped", "limit": 5}},
		{"list_alerts", map[string]any{"severity": []string{"warning"}, "limit": 5}},
		{"get_candidates", nil},
		{"get_candidate", map[string]any{"symbol": "caap"}},
		{"get_symbol_analysis", map[string]any{"symbol": "CAAP"}},
		{"get_watchlist", nil},
		{"get_tracked_positions", map[string]any{"status": "all"}},
		{"get_data_source_status", nil},
		{"list_handbook_sections", nil},
		{"get_handbook_section", map[string]any{"id": "data-source"}},
		{"get_handbook_section", map[string]any{"id": "data-source-chain-statuses"}},
	}
	secretish := regexp.MustCompile(`(?i)(token=[^\[&\s"]|bearer [a-z0-9]{8}|"api_?key"|` + plantedSecret + `)`)
	for _, c := range calls {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: c.name, Arguments: c.args})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		body := resultText(res)
		if res.IsError {
			t.Errorf("%s %v: tool error %s", c.name, c.args, body)
			continue
		}
		if len(body) > tools.MaxResultBytes {
			t.Errorf("%s: %d bytes", c.name, len(body))
		}
		if m := secretish.FindString(body); m != "" {
			t.Errorf("%s: secret-like text in result: %q", c.name, m)
		}
		t.Logf("%-24s ok, %6d bytes", c.name, len(body))
	}

	// Bad input never reaches momentum-api.
	for _, c := range []struct {
		name string
		args map[string]any
	}{
		{"get_candidate", map[string]any{"symbol": "../watchlist"}},
		{"get_candidate", map[string]any{"symbol": "AAPL?x=1"}},
		{"list_alerts", map[string]any{"mode": "delete"}},
		{"list_alerts", map[string]any{"limit": 500}},
		{"get_tracked_positions", map[string]any{"status": "open"}},
		{"get_handbook_section", map[string]any{"id": "../../etc"}},
	} {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: c.name, Arguments: c.args})
		if err != nil || !res.IsError {
			t.Errorf("%s %v: want a tool error", c.name, c.args)
		}
	}
	if e.methods != nil {
		for _, m := range *e.methods {
			if !strings.HasPrefix(m, "GET ") {
				t.Errorf("non-GET reached momentum-api: %s", m)
			}
		}
	}

	// 7. Logs: no tokens, codes, secrets or response bodies.
	logs := e.logs.String()
	for _, s := range []string{access, refresh, code, clientSecret, ghSecret, "gh-token", "gh-code", plantedSecret, "claude-state", "CAAP"} {
		if strings.Contains(logs, s) {
			t.Errorf("log contains %q", s[:min(len(s), 12)])
		}
	}
	if !strings.Contains(logs, `"tool":"get_symbol_analysis"`) {
		t.Error("tool calls are not logged")
	}
}

func (e *env) connect(t *testing.T, ctx context.Context, token string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   e.resource,
		HTTPClient: &http.Client{Transport: bearerRT{token, e.claude.Transport}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return cs
}

func toolDesc(lt *mcp.ListToolsResult, name string) string {
	for _, tl := range lt.Tools {
		if tl.Name == name {
			return tl.Description
		}
	}
	return ""
}

func resultText(r *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range r.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func TestRejections(t *testing.T) {
	e := setup(t)
	var asm struct {
		Authorization string `json:"authorization_endpoint"`
		Token         string `json:"token_endpoint"`
	}
	getJSON(t, e.claude, e.srv.URL+"/.well-known/oauth-authorization-server", &asm)

	t.Run("another GitHub user is refused", func(t *testing.T) {
		e.gh.mu.Lock()
		e.gh.userID = strangerID
		e.gh.mu.Unlock()
		defer func() { e.gh.mu.Lock(); e.gh.userID = ownerID; e.gh.mu.Unlock() }()
		_, challenge := pkcePair()
		cb := e.signIn(t, asm.Authorization, challenge)
		if cb.Query().Get("code") != "" || cb.Query().Get("error") != "access_denied" {
			t.Fatalf("stranger got %s", cb.Query().Encode())
		}
		if !strings.Contains(e.logs.String(), fmt.Sprintf(`"github_user_id":%d`, strangerID)) {
			t.Error("rejected user id not logged")
		}
	})

	t.Run("unknown client or redirect gets no redirect", func(t *testing.T) {
		for _, q := range []url.Values{
			{"client_id": {"other"}, "redirect_uri": {claudeRedir}},
			{"client_id": {clientID}, "redirect_uri": {"https://evil.example/cb"}},
		} {
			resp, err := e.browser.Get(asm.Authorization + "?" + q.Encode())
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != 400 || resp.Header.Get("Location") != "" {
				t.Fatalf("%v: %d %q", q, resp.StatusCode, resp.Header.Get("Location"))
			}
		}
	})

	t.Run("plain PKCE is refused", func(t *testing.T) {
		q := url.Values{"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {claudeRedir},
			"code_challenge": {strings.Repeat("a", 43)}, "code_challenge_method": {"plain"}}
		resp, _ := e.browser.Get(asm.Authorization + "?" + q.Encode())
		resp.Body.Close()
		if loc, _ := url.Parse(resp.Header.Get("Location")); loc == nil || loc.Query().Get("error") != "invalid_request" {
			t.Fatalf("plain PKCE: %d %s", resp.StatusCode, resp.Header.Get("Location"))
		}
	})

	t.Run("wrong client secret", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, asm.Token, strings.NewReader("grant_type=refresh_token&refresh_token=x"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(clientID, "wrong-secret-wrong-secret-wrong-secret")
		resp, err := e.claude.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Fatalf("got %d", resp.StatusCode)
		}
	})

	t.Run("forged and tampered tokens", func(t *testing.T) {
		for _, tok := range []string{"mcp1.e30.AAAA", "garbage", strings.Repeat("a", 5000)} {
			req, _ := http.NewRequest(http.MethodPost, e.resource, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tok)
			resp, err := e.claude.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != 401 {
				t.Fatalf("forged token: %d", resp.StatusCode)
			}
		}
	})

	t.Run("outside Anthropic's range is blocked and logged", func(t *testing.T) {
		out := &http.Client{Transport: sourceIP{outsideIP, e.claude.Transport.(sourceIP).base}}
		for _, p := range []string{"/mcp", "/token"} {
			resp, err := out.Post(e.srv.URL+p, "application/json", strings.NewReader("{}"))
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 403 {
				t.Fatalf("%s from outside: %d %s", p, resp.StatusCode, b)
			}
		}
		if !strings.Contains(e.logs.String(), `"msg":"blocked: outside IP allowlist"`) || !strings.Contains(e.logs.String(), outsideIP) {
			t.Error("blocked request not logged")
		}
		// The browser leg stays reachable from home.
		resp, err := e.browser.Get(e.srv.URL + "/.well-known/oauth-authorization-server")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("metadata from home: %d", resp.StatusCode)
		}
	})
}

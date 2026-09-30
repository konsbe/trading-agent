// Package oauth is svc-mcp's single-client, single-user OAuth 2.1
// authorization server. Claude is the only client (pre-registered ID and
// secret, fixed redirect URIs, PKCE S256). The user proves who they are by
// signing in with GitHub; only one numeric GitHub user id is accepted.
package oauth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/konsbe/trading-agent/services/svc-mcp/internal/guard"
)

const (
	Scope          = "mcp:read"
	pendingTTL     = 10 * time.Minute
	codeTTL        = 60 * time.Second
	bindCookie     = "mcp_auth_bind"
	githubCallback = "/oauth/github/callback"
)

var (
	pkceRe  = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
	stateRe = regexp.MustCompile(`^[\x21-\x7e]{1,512}$`)
)

type Config struct {
	Issuer       string // https origin; also the public base URL
	Resource     string // Issuer + "/mcp"
	ClientID     string
	ClientSecret string
	RedirectURIs []string
	SigningKey   []byte
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
	GitHub       GitHubConfig
	Now          func() time.Time
}

type pending struct {
	clientRedirect string
	clientState    string
	challenge      string
	bindHash       [32]byte
	expires        time.Time
}

type issuedCode struct {
	redirect  string
	challenge string
	sub       int64
	expires   time.Time
}

type Server struct {
	cfg     Config
	sign    signer
	gh      *GitHub
	lockout *guard.Lockout
	log     *slog.Logger

	mu      sync.Mutex
	pending map[string]pending
	codes   map[string]issuedCode
}

func New(cfg Config, lockout *guard.Lockout, log *slog.Logger) *Server {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Server{
		cfg: cfg, sign: signer{key: cfg.SigningKey}, gh: NewGitHub(cfg.GitHub),
		lockout: lockout, log: log,
		pending: map[string]pending{}, codes: map[string]issuedCode{},
	}
}

func (s *Server) ResourceMetadataURL() string {
	return s.cfg.Issuer + "/.well-known/oauth-protected-resource/mcp"
}

// Routes mounts metadata and OAuth endpoints; /mcp is mounted by the caller.
func (s *Server) Routes(mux *http.ServeMux) {
	prm := auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               s.cfg.Resource,
		AuthorizationServers:   []string{s.cfg.Issuer},
		ScopesSupported:        []string{Scope},
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "trading-agent (read-only)",
	})
	mux.Handle("GET /.well-known/oauth-protected-resource", prm)
	mux.Handle("GET /.well-known/oauth-protected-resource/mcp", prm)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", s.handleASMeta)
	mux.HandleFunc("GET /authorize", s.handleAuthorize)
	mux.HandleFunc("GET "+githubCallback, s.handleGitHubCallback)
	mux.HandleFunc("POST /token", s.handleToken)
}

func (s *Server) handleASMeta(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                         s.cfg.Issuer,
		"authorization_endpoint":                         s.cfg.Issuer + "/authorize",
		"token_endpoint":                                 s.cfg.Issuer + "/token",
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          []string{"client_secret_basic", "client_secret_post"},
		"scopes_supported":                               []string{Scope},
		"authorization_response_iss_parameter_supported": true,
	})
}

func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	ip := guard.ClientIP(r.Context())
	q := r.URL.Query()
	redirect := q.Get("redirect_uri")
	// Until client and redirect are known good, errors go to the browser,
	// never to a redirect (RFC 6749 §4.1.2.1).
	if !s.clientIDOK(q.Get("client_id")) || !slices.Contains(s.cfg.RedirectURIs, redirect) {
		s.lockout.Fail(ip)
		s.log.Warn("authorize rejected", "ip", ip, "reason", "unknown client_id or redirect_uri")
		plain(w, http.StatusBadRequest, "Unknown client or redirect URI.")
		return
	}
	state := q.Get("state")
	if state != "" && !stateRe.MatchString(state) {
		plain(w, http.StatusBadRequest, "Invalid state.")
		return
	}
	fail := func(code, desc string) {
		s.log.Warn("authorize rejected", "ip", ip, "reason", code)
		s.redirectClient(w, r, redirect, url.Values{"error": {code}, "error_description": {desc}}, state)
	}
	if q.Get("response_type") != "code" {
		fail("unsupported_response_type", "response_type must be code")
		return
	}
	if q.Get("code_challenge_method") != "S256" || !pkceRe.MatchString(q.Get("code_challenge")) {
		fail("invalid_request", "PKCE with code_challenge_method=S256 is required")
		return
	}
	if res := q.Get("resource"); res != "" && res != s.cfg.Resource {
		fail("invalid_target", "unknown resource")
		return
	}

	ghState, bind := randomToken(32), randomToken(32)
	s.mu.Lock()
	s.sweepLocked()
	s.pending[ghState] = pending{
		clientRedirect: redirect, clientState: state, challenge: q.Get("code_challenge"),
		bindHash: sha256.Sum256([]byte(bind)), expires: s.cfg.Now().Add(pendingTTL),
	}
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name: bindCookie, Value: bind, Path: githubCallback, MaxAge: int(pendingTTL.Seconds()),
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	s.log.Info("authorize: sent to GitHub", "ip", ip)
	http.Redirect(w, r, s.gh.AuthorizeURL(ghState, s.cfg.Issuer+githubCallback), http.StatusFound)
}

func (s *Server) handleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	ip := guard.ClientIP(r.Context())
	q := r.URL.Query()
	ghState := q.Get("state")
	s.mu.Lock()
	p, ok := s.pending[ghState]
	delete(s.pending, ghState)
	s.mu.Unlock()
	if !ok || s.cfg.Now().After(p.expires) {
		s.lockout.Fail(ip)
		plain(w, http.StatusBadRequest, "This sign-in link has expired. Start again from Claude.")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: bindCookie, Path: githubCallback, MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	c, err := r.Cookie(bindCookie)
	if err != nil {
		s.lockout.Fail(ip)
		plain(w, http.StatusBadRequest, "Sign-in must finish in the browser that started it.")
		return
	}
	if h := sha256.Sum256([]byte(c.Value)); subtle.ConstantTimeCompare(h[:], p.bindHash[:]) != 1 {
		s.lockout.Fail(ip)
		plain(w, http.StatusBadRequest, "Sign-in must finish in the browser that started it.")
		return
	}
	if e := q.Get("error"); e != "" {
		s.log.Warn("github sign-in declined", "ip", ip)
		s.redirectClient(w, r, p.clientRedirect, url.Values{"error": {"access_denied"}}, p.clientState)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	userID, err := s.gh.UserID(ctx, q.Get("code"), s.cfg.Issuer+githubCallback)
	if err != nil {
		s.lockout.Fail(ip)
		s.log.Warn("github sign-in failed", "ip", ip, "err", err.Error())
		plain(w, http.StatusBadGateway, "GitHub sign-in failed. Try again from Claude.")
		return
	}
	if userID != s.cfg.GitHub.AllowedUserID {
		s.lockout.Fail(ip)
		s.log.Warn("github user rejected", "ip", ip, "github_user_id", userID)
		s.redirectClient(w, r, p.clientRedirect, url.Values{"error": {"access_denied"}, "error_description": {"this account is not allowed"}}, p.clientState)
		return
	}
	code := randomToken(32)
	s.mu.Lock()
	s.codes[code] = issuedCode{redirect: p.clientRedirect, challenge: p.challenge, sub: userID, expires: s.cfg.Now().Add(codeTTL)}
	s.mu.Unlock()
	s.log.Info("github user accepted; code issued", "ip", ip)
	s.redirectClient(w, r, p.clientRedirect, url.Values{"code": {code}}, p.clientState)
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	ip := guard.ClientIP(r.Context())
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		tokenErr(w, http.StatusBadRequest, "invalid_request", "form body required")
		return
	}
	if !s.clientAuthOK(r) {
		s.lockout.Fail(ip)
		s.log.Warn("token rejected", "ip", ip, "reason", "invalid_client")
		w.Header().Set("WWW-Authenticate", `Basic realm="token"`)
		tokenErr(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
		return
	}
	if res := r.PostForm.Get("resource"); res != "" && res != s.cfg.Resource {
		tokenErr(w, http.StatusBadRequest, "invalid_target", "unknown resource")
		return
	}
	now := s.cfg.Now()
	var sub int64
	switch gt := r.PostForm.Get("grant_type"); gt {
	case "authorization_code":
		code := r.PostForm.Get("code")
		s.mu.Lock()
		ic, ok := s.codes[code]
		delete(s.codes, code)
		s.mu.Unlock()
		verifier := r.PostForm.Get("code_verifier")
		switch {
		case !ok || now.After(ic.expires):
			s.grantFail(w, ip, "unknown or expired code")
			return
		case r.PostForm.Get("redirect_uri") != ic.redirect:
			s.grantFail(w, ip, "redirect_uri mismatch")
			return
		case !pkceRe.MatchString(verifier) || !pkceOK(verifier, ic.challenge):
			s.grantFail(w, ip, "PKCE verification failed")
			return
		}
		sub = ic.sub
	case "refresh_token":
		c, err := s.sign.verify(r.PostForm.Get("refresh_token"), typRefresh, now)
		if err != nil || !s.claimsOK(c) {
			s.grantFail(w, ip, "invalid refresh token")
			return
		}
		sub = c.Sub
	default:
		tokenErr(w, http.StatusBadRequest, "unsupported_grant_type", "authorization_code or refresh_token")
		return
	}
	access, err1 := s.issue(typAccess, sub, now, s.cfg.AccessTTL)
	refresh, err2 := s.issue(typRefresh, sub, now, s.cfg.RefreshTTL)
	if err := errors.Join(err1, err2); err != nil {
		tokenErr(w, http.StatusInternalServerError, "server_error", "")
		return
	}
	s.log.Info("tokens issued", "ip", ip, "grant", r.PostForm.Get("grant_type"), "token_id", TokenID(access))
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": access, "token_type": "Bearer", "expires_in": int(s.cfg.AccessTTL.Seconds()),
		"refresh_token": refresh, "scope": Scope,
	})
}

// Verify is the bearer-token check for /mcp.
func (s *Server) Verify(_ context.Context, tok string, _ *http.Request) (*auth.TokenInfo, error) {
	c, err := s.sign.verify(tok, typAccess, s.cfg.Now())
	if err != nil || !s.claimsOK(c) {
		return nil, auth.ErrInvalidToken
	}
	return &auth.TokenInfo{Scopes: []string{c.Scope}, Expiration: time.Unix(c.Exp, 0), UserID: strconv.FormatInt(c.Sub, 10)}, nil
}

// claimsOK ties a token to this resource, client and the allowed user, so
// changing GITHUB_ALLOWED_USER_ID also revokes the old user's tokens.
func (s *Server) claimsOK(c claims) bool {
	return c.Aud == s.cfg.Resource && c.ClientID == s.cfg.ClientID && c.Sub == s.cfg.GitHub.AllowedUserID
}

func (s *Server) issue(typ string, sub int64, now time.Time, ttl time.Duration) (string, error) {
	return s.sign.sign(claims{
		Typ: typ, Sub: sub, Aud: s.cfg.Resource, ClientID: s.cfg.ClientID, Scope: Scope,
		Iat: now.Unix(), Exp: now.Add(ttl).Unix(), JTI: randomToken(12),
	})
}

func (s *Server) grantFail(w http.ResponseWriter, ip, reason string) {
	s.lockout.Fail(ip)
	s.log.Warn("token rejected", "ip", ip, "reason", reason)
	tokenErr(w, http.StatusBadRequest, "invalid_grant", "")
}

func (s *Server) clientIDOK(id string) bool {
	return subtle.ConstantTimeCompare([]byte(id), []byte(s.cfg.ClientID)) == 1
}

func (s *Server) clientAuthOK(r *http.Request) bool {
	id, secret, ok := r.BasicAuth()
	if ok {
		// client_secret_basic form-encodes both parts (RFC 6749 §2.3.1).
		var err1, err2 error
		id, err1 = url.QueryUnescape(id)
		secret, err2 = url.QueryUnescape(secret)
		if err1 != nil || err2 != nil {
			return false
		}
	} else {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	secretOK := subtle.ConstantTimeCompare([]byte(secret), []byte(s.cfg.ClientSecret)) == 1
	return s.clientIDOK(id) && secretOK
}

func (s *Server) redirectClient(w http.ResponseWriter, r *http.Request, redirect string, v url.Values, state string) {
	u, err := url.Parse(redirect)
	if err != nil {
		plain(w, http.StatusBadRequest, "Invalid redirect URI.")
		return
	}
	q := u.Query()
	for k, vs := range v {
		q[k] = vs
	}
	if state != "" {
		q.Set("state", state)
	}
	q.Set("iss", s.cfg.Issuer)
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (s *Server) sweepLocked() {
	now := s.cfg.Now()
	for k, p := range s.pending {
		if now.After(p.expires) {
			delete(s.pending, k)
		}
	}
	for k, c := range s.codes {
		if now.After(c.expires) {
			delete(s.codes, k)
		}
	}
}

func pkceOK(verifier, challenge string) bool {
	h := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(h[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func tokenErr(w http.ResponseWriter, status int, code, desc string) {
	body := map[string]string{"error": code}
	if desc != "" {
		body["error_description"] = desc
	}
	writeJSON(w, status, body)
}

func plain(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(strings.TrimSpace(msg) + "\n"))
}

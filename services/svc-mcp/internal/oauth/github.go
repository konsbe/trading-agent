package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type GitHubConfig struct {
	ClientID      string
	ClientSecret  string
	AllowedUserID int64
	WebURL        string // https://github.com
	APIURL        string // https://api.github.com
	HTTP          *http.Client
}

// GitHub is used only to learn who signed in. No scope is requested, so the
// token can read public profile data and nothing else; it is revoked as soon
// as the numeric id is known and never stored.
type GitHub struct {
	cfg  GitHubConfig
	http *http.Client
}

func NewGitHub(cfg GitHubConfig) *GitHub {
	c := cfg.HTTP
	if c == nil {
		c = &http.Client{Timeout: 10 * time.Second}
	}
	return &GitHub{cfg: cfg, http: c}
}

func (g *GitHub) AuthorizeURL(state, redirect string) string {
	q := url.Values{"client_id": {g.cfg.ClientID}, "redirect_uri": {redirect}, "state": {state}, "allow_signup": {"false"}}
	return g.cfg.WebURL + "/login/oauth/authorize?" + q.Encode()
}

// UserID exchanges the callback code and returns the numeric GitHub user id.
// Errors never include the code, the token or the client secret.
func (g *GitHub) UserID(ctx context.Context, code, redirect string) (int64, error) {
	if code == "" || len(code) > 512 {
		return 0, errors.New("missing code")
	}
	form := url.Values{"client_id": {g.cfg.ClientID}, "client_secret": {g.cfg.ClientSecret}, "code": {code}, "redirect_uri": {redirect}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.cfg.WebURL+"/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return 0, errors.New("building token request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	var tok struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := g.do(req, &tok); err != nil {
		return 0, fmt.Errorf("token exchange: %w", err)
	}
	if tok.AccessToken == "" {
		return 0, fmt.Errorf("token exchange refused (%s)", safeCode(tok.Error))
	}
	defer g.revoke(tok.AccessToken)

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, g.cfg.APIURL+"/user", nil)
	if err != nil {
		return 0, errors.New("building user request")
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	var user struct {
		ID int64 `json:"id"`
	}
	if err := g.do(req, &user); err != nil {
		return 0, fmt.Errorf("user lookup: %w", err)
	}
	if user.ID < 1 {
		return 0, errors.New("user lookup: no id")
	}
	return user.ID, nil
}

func (g *GitHub) revoke(token string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	body, _ := json.Marshal(map[string]string{"access_token": token})
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, g.cfg.APIURL+"/applications/"+url.PathEscape(g.cfg.ClientID)+"/token", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.SetBasicAuth(g.cfg.ClientID, g.cfg.ClientSecret)
	req.Header.Set("Accept", "application/vnd.github+json")
	if resp, err := g.http.Do(req); err == nil {
		resp.Body.Close()
	}
}

func (g *GitHub) do(req *http.Request, out any) error {
	req.Header.Set("User-Agent", "trading-agent-svc-mcp")
	resp, err := g.http.Do(req)
	if err != nil {
		return errors.New("request failed")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return errors.New("reading reply")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return errors.New("reply is not JSON")
	}
	return nil
}

func safeCode(s string) string {
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r == '_') {
			return "unknown"
		}
	}
	if s == "" || len(s) > 40 {
		return "unknown"
	}
	return s
}

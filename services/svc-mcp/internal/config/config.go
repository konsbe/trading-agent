// Package config reads svc-mcp's settings from the environment. Secrets have
// no defaults: a missing one fails startup rather than running open.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr string
	// PublicURL is the HTTPS origin Claude reaches (https://mcp.example.com).
	// It is the OAuth issuer; the protected resource is PublicURL + "/mcp".
	PublicURL      string
	MomentumAPIURL string
	CaveatsPath    string

	ClientID     string
	ClientSecret string
	SigningKey   []byte
	RedirectURIs []string
	AccessTTL    time.Duration
	RefreshTTL   time.Duration

	GitHubClientID     string
	GitHubClientSecret string
	GitHubUserID       int64
	GitHubWebURL       string
	GitHubAPIURL       string
	// GitHubHTTP is set only by tests (a fake GitHub with its own TLS cert).
	GitHubHTTP *http.Client

	IPAllowlistEnable bool
	IPAllowlist       []netip.Prefix
	// TrustedProxies are the peers whose CF-Connecting-IP header is believed
	// (cloudflared on the Compose network). From any other peer the header is
	// ignored and the TCP address is the client.
	TrustedProxies []netip.Prefix

	RatePerToken  int
	RateGlobal    int
	RateAuthPerIP int
}

func (c Config) ResourceURL() string { return c.PublicURL + "/mcp" }

func Load() (Config, error) {
	var errs []error
	req := func(key string) string {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is required", key))
		}
		return v
	}
	c := Config{
		Addr:               env("MCP_ADDR", "0.0.0.0:8095"),
		PublicURL:          strings.TrimRight(req("MCP_PUBLIC_URL"), "/"),
		MomentumAPIURL:     strings.TrimRight(env("MCP_MOMENTUM_API_URL", "http://momentum-api:8090"), "/"),
		CaveatsPath:        env("MCP_CAVEATS_PATH", "/shared/content/momentum_caveats.json"),
		ClientID:           req("MCP_OAUTH_CLIENT_ID"),
		ClientSecret:       req("MCP_OAUTH_CLIENT_SECRET"),
		RedirectURIs:       list(env("MCP_OAUTH_REDIRECT_URIS", "https://claude.ai/api/mcp/auth_callback,https://claude.com/api/mcp/auth_callback")),
		GitHubClientID:     req("GITHUB_OAUTH_CLIENT_ID"),
		GitHubClientSecret: req("GITHUB_OAUTH_CLIENT_SECRET"),
		GitHubWebURL:       strings.TrimRight(env("GITHUB_WEB_URL", "https://github.com"), "/"),
		GitHubAPIURL:       strings.TrimRight(env("GITHUB_API_URL", "https://api.github.com"), "/"),
	}
	if u, err := url.Parse(c.PublicURL); c.PublicURL != "" && (err != nil || u.Scheme != "https" || u.Host == "" || u.Path != "") {
		errs = append(errs, errors.New("MCP_PUBLIC_URL must be an https origin with no path"))
	}
	if len(c.ClientSecret) > 0 && len(c.ClientSecret) < 32 {
		errs = append(errs, errors.New("MCP_OAUTH_CLIENT_SECRET must be at least 32 characters"))
	}
	if raw := req("MCP_TOKEN_SIGNING_KEY"); raw != "" {
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil || len(key) < 32 {
			errs = append(errs, errors.New("MCP_TOKEN_SIGNING_KEY must be base64 of at least 32 bytes"))
		}
		c.SigningKey = key
	}
	if raw := req("GITHUB_ALLOWED_USER_ID"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 1 {
			errs = append(errs, errors.New("GITHUB_ALLOWED_USER_ID must be the numeric GitHub user id"))
		}
		c.GitHubUserID = id
	}
	var err error
	if c.AccessTTL, err = time.ParseDuration(env("MCP_ACCESS_TOKEN_TTL", "1h")); err != nil {
		errs = append(errs, fmt.Errorf("MCP_ACCESS_TOKEN_TTL: %w", err))
	}
	if c.RefreshTTL, err = time.ParseDuration(env("MCP_REFRESH_TOKEN_TTL", "720h")); err != nil {
		errs = append(errs, fmt.Errorf("MCP_REFRESH_TOKEN_TTL: %w", err))
	}
	c.IPAllowlistEnable = env("MCP_IP_ALLOWLIST_ENABLE", "true") != "false"
	if c.IPAllowlist, err = prefixes(env("MCP_IP_ALLOWLIST", "160.79.104.0/21")); err != nil {
		errs = append(errs, fmt.Errorf("MCP_IP_ALLOWLIST: %w", err))
	}
	if c.TrustedProxies, err = prefixes(env("MCP_TRUSTED_PROXIES", "172.16.0.0/12,10.0.0.0/8,192.168.0.0/16")); err != nil {
		errs = append(errs, fmt.Errorf("MCP_TRUSTED_PROXIES: %w", err))
	}
	c.RatePerToken = intEnv("MCP_RATE_PER_TOKEN_PER_MIN", 60, &errs)
	c.RateGlobal = intEnv("MCP_RATE_GLOBAL_PER_MIN", 300, &errs)
	c.RateAuthPerIP = intEnv("MCP_RATE_AUTH_PER_IP_PER_MIN", 10, &errs)
	for _, u := range c.RedirectURIs {
		if p, err := url.Parse(u); err != nil || p.Scheme != "https" {
			errs = append(errs, fmt.Errorf("MCP_OAUTH_REDIRECT_URIS: %q is not an https URL", u))
		}
	}
	return c, errors.Join(errs...)
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func list(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func prefixes(s string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, p := range list(s) {
		pfx, err := netip.ParsePrefix(p)
		if err != nil {
			return nil, err
		}
		out = append(out, pfx.Masked())
	}
	return out, nil
}

func intEnv(key string, def int, errs *[]error) int {
	raw := env(key, "")
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		*errs = append(*errs, fmt.Errorf("%s must be a positive integer", key))
		return def
	}
	return n
}

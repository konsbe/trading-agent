// Package app assembles svc-mcp's HTTP handler; main and the end-to-end
// test build the same one.
package app

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/konsbe/trading-agent/services/svc-mcp/internal/config"
	"github.com/konsbe/trading-agent/services/svc-mcp/internal/guard"
	"github.com/konsbe/trading-agent/services/svc-mcp/internal/oauth"
	"github.com/konsbe/trading-agent/services/svc-mcp/internal/tools"
)

// maxMCPBody is far above any tools/call this server accepts.
const maxMCPBody = 64 << 10

func Handler(cfg config.Config, api tools.Getter, caveats tools.Caveats, log *slog.Logger, now func() time.Time) http.Handler {
	if now == nil {
		now = time.Now
	}
	lockout := guard.NewLockout(5, 15*time.Minute, now)
	as := oauth.New(oauth.Config{
		Issuer: cfg.PublicURL, Resource: cfg.ResourceURL(),
		ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, RedirectURIs: cfg.RedirectURIs,
		SigningKey: cfg.SigningKey, AccessTTL: cfg.AccessTTL, RefreshTTL: cfg.RefreshTTL,
		GitHub: oauth.GitHubConfig{
			ClientID: cfg.GitHubClientID, ClientSecret: cfg.GitHubClientSecret, AllowedUserID: cfg.GitHubUserID,
			WebURL: cfg.GitHubWebURL, APIURL: cfg.GitHubAPIURL, HTTP: cfg.GitHubHTTP,
		},
		Now: now,
	}, lockout, log)

	server := tools.NewServer(api, caveats)
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{
		Stateless: true, JSONResponse: true, MaxRequestBodyBytes: maxMCPBody,
	})
	protected := auth.RequireBearerToken(as.Verify, &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: as.ResourceMetadataURL(),
		Scopes:              []string{oauth.Scope},
	})(mcpHandler)

	mux := http.NewServeMux()
	as.Routes(mux)
	mux.Handle("/mcp", protected)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) })

	return guard.Wrap(mux, guard.Options{
		Log: log, TrustedProxies: cfg.TrustedProxies,
		IPAllowlistEnable: cfg.IPAllowlistEnable, IPAllowlist: cfg.IPAllowlist,
		Lockout:   lockout,
		PerToken:  guard.NewLimiter(cfg.RatePerToken, now),
		Global:    guard.NewLimiter(cfg.RateGlobal, now),
		AuthPerIP: guard.NewLimiter(cfg.RateAuthPerIP, now),
	})
}

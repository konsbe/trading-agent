// svc-mcp is the read-only MCP server behind the claude.ai custom connector.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/konsbe/trading-agent/services/svc-mcp/internal/app"
	"github.com/konsbe/trading-agent/services/svc-mcp/internal/config"
	"github.com/konsbe/trading-agent/services/svc-mcp/internal/momentum"
	"github.com/konsbe/trading-agent/services/svc-mcp/internal/tools"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		// Load's errors name variables, never their values.
		log.Error("config", "err", err.Error())
		os.Exit(1)
	}
	caveats, err := tools.LoadCaveats(cfg.CaveatsPath)
	if err != nil {
		log.Error("caveats", "err", err.Error())
		os.Exit(1)
	}
	if !cfg.IPAllowlistEnable {
		log.Warn("IP allowlist disabled (MCP_IP_ALLOWLIST_ENABLE=false): /mcp and /token accept any source")
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           app.Handler(cfg, momentum.New(cfg.MomentumAPIURL), caveats, log, nil),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shut, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shut)
	}()
	log.Info("svc-mcp listening", "addr", cfg.Addr, "resource", cfg.ResourceURL(), "momentum_api", cfg.MomentumAPIURL,
		"ip_allowlist", cfg.IPAllowlistEnable)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("server", "err", err.Error())
		os.Exit(1)
	}
}

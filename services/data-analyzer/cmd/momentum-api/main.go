// Command momentum-api serves the momentum scanner's persisted output to the
// web app: docs/MOMENTUM_SCANNER_API.md.
//
// Read-only. It computes nothing — no gates, no scoring, no features — and
// never writes. Everything served is what momentum-scanner already stored.
//
// Unlike the other cmd/ binaries this is a long-running server, not a one-shot
// job.
//
// NO AUTHENTICATION. There is no identity provider to validate tokens against
// yet, so the server binds to localhost by default and must only be reachable
// on a trusted network. Real auth is a hard blocker before exposing it any
// further — see the README next to this file.
//
//	DATABASE_URL=... go run ./cmd/momentum-api
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // the session calendar needs America/New_York in a distroless image

	"github.com/joho/godotenv"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/db"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/logx"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentumapi"
)

// maxCacheTTL is the ceiling from the spec: longer would let a stale cache hide
// a corrected re-run.
const maxCacheTTL = 5 * time.Minute

func main() {
	// Service-local .env first, then the repo root's (the documented home of
	// the shared config when run from services/data-analyzer). Load never
	// overrides variables already set, so the environment still wins.
	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")
	log := logx.New(env("LOG_LEVEL", "info"))

	databaseURL := env("DATABASE_URL", "")
	if databaseURL == "" {
		log.Error("momentum-api: DATABASE_URL is required")
		os.Exit(1)
	}
	addr := env("MOMENTUM_API_ADDR", "127.0.0.1:8090")
	caveatsPath := env("MOMENTUM_CAVEATS_PATH", "../../shared/content/momentum_caveats.json")
	scanGrace := duration(log, "MOMENTUM_API_SCAN_GRACE", 6*time.Hour)
	cacheTTL := duration(log, "MOMENTUM_API_CACHE_TTL", maxCacheTTL)
	if cacheTTL > maxCacheTTL {
		log.Warn("momentum-api: cache TTL capped", "requested", cacheTTL, "cap", maxCacheTTL)
		cacheTTL = maxCacheTTL
	}
	origins := csv(env("MOMENTUM_API_CORS_ORIGINS", "http://localhost:3000"))

	caveats, err := momentumapi.LoadCaveats(caveatsPath)
	if err != nil {
		log.Error("momentum-api: caveats", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		log.Error("momentum-api: database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if _, err := momentumapi.IsTradingDay(time.Now().AddDate(0, 0, 60)); err != nil {
		log.Warn("momentum-api: session calendar runs out within 60 days", "err", err)
	}

	srv := momentumapi.NewServer(momentumapi.Config{
		Store:             momentumapi.DBStore{Q: pool, PingFn: pool.Ping},
		Caveats:           caveats,
		Log:               log,
		SessionReadyAfter: scanGrace,
		CacheTTL:          cacheTTL,
		CORSOrigins:       origins,
	})
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Info("momentum-api: listening", "addr", addr, "cors_origins", origins, "cache_ttl", cacheTTL.String(),
		"scan_grace", scanGrace.String(), "auth", "none — trusted network only")
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("momentum-api: serve", "err", err)
		os.Exit(1)
	}
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func duration(log *slog.Logger, key string, def time.Duration) time.Duration {
	raw := env(key, "")
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		log.Warn("momentum-api: invalid duration, using default", "key", key, "value", raw, "default", def)
		return def
	}
	return d
}

func csv(raw string) []string {
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

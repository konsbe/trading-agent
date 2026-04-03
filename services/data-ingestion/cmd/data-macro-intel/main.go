// data-macro-intel ingests macro calendars, geopolitical proxies (GPR, GDELT), RSS macro headlines,
// and Finnhub general market news into TimescaleDB. No LLM — raw + light aggregates only.
//
// Tables: economic_calendar_events, earnings_calendar_events, geopolitical_risk_monthly,
//         gdelt_macro_daily, news_headlines (RSS + Finnhub general).
//
// TODO [PAID]: TradingEconomics / Investing.com calendars if Finnhub tier blocks economic API.
// TODO [LLM]: narrative_scores filled by analyst-bot (FOMC hawkish/dovish scoring).
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/config"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/db"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/finnhub"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/gdelt"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/gpr"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/rss"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/logx"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.LoadMacroIntel()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	log := logx.New(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	fh := finnhub.New(cfg.FinnhubKey)
	gdc := gdelt.New()

	run := func() {
		runAll(ctx, log, pool, cfg, fh, gdc)
	}
	run()

	t := time.NewTicker(cfg.PollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown")
			return
		case <-t.C:
			run()
		}
	}
}

func runAll(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, cfg config.MacroIntel, fh *finnhub.Client, gdc *gdelt.Client) {
	if cfg.EnableEconomicCalendar && fh.HasToken() {
		runEconomicCalendar(ctx, log, pool, fh)
	}
	if cfg.EnableEarningsCalendar && fh.HasToken() {
		runEarningsCalendar(ctx, log, pool, fh, cfg.EarningsSymbols)
	}
	if cfg.EnableFinnhubGeneralNews && fh.HasToken() {
		runFinnhubGeneral(ctx, log, pool, fh)
	}
	for _, u := range cfg.RSSFeedURLs {
		runRSS(ctx, log, pool, u, cfg.RSSMaxItems)
	}
	if cfg.GPRCSVURL != "" {
		runGPR(ctx, log, pool, cfg.GPRCSVURL)
	}
	if cfg.GDELTEnabled {
		runGDELT(ctx, log, pool, gdc, cfg.GDELTQuery, cfg.GDELTMaxRec, cfg.GDELTLookback)
	}
}

func runEconomicCalendar(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, fh *finnhub.Client) {
	from := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	to := time.Now().UTC().AddDate(0, 0, 14).Format("2006-01-02")
	items, err := fh.CalendarEconomic(ctx, from, to)
	if err != nil {
		log.Warn("economic calendar", "err", err)
		return
	}
	for _, m := range items {
		eventName, _ := m["event"].(string)
		if eventName == "" {
			continue
		}
		country, _ := m["country"].(string)
		impact, _ := m["impact"].(string)
		unit, _ := m["unit"].(string)
		eventTS := parseFinnhubTime(m)
		act := finnhubFloat(m["actual"])
		est := finnhubFloat(m["estimate"])
		prev := finnhubFloat(m["prev"])
		if prev == nil {
			prev = finnhubFloat(m["previous"])
		}
		extID := externalID("fh_eco", country, eventName, eventTS)
		if err := store.UpsertEconomicCalendarEvent(ctx, pool, eventTS, country, eventName, impact, act, est, prev, unit, "finnhub", extID, m); err != nil {
			log.Error("upsert economic", "err", err)
		}
	}
	log.Info("economic calendar ingested", "n", len(items))
}

func runEarningsCalendar(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, fh *finnhub.Client, symbols []string) {
	from := time.Now().UTC().AddDate(0, 0, -2).Format("2006-01-02")
	to := time.Now().UTC().AddDate(0, 0, 21).Format("2006-01-02")
	if len(symbols) == 0 {
		items, err := fh.CalendarEarnings(ctx, from, to, "")
		if err != nil {
			log.Warn("earnings calendar", "err", err)
			return
		}
		ingestEarnings(ctx, log, pool, items)
		return
	}
	for _, sym := range symbols {
		items, err := fh.CalendarEarnings(ctx, from, to, sym)
		if err != nil {
			log.Warn("earnings calendar", "symbol", sym, "err", err)
			continue
		}
		ingestEarnings(ctx, log, pool, items)
	}
}

func ingestEarnings(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, items []map[string]any) {
	for _, m := range items {
		sym, _ := m["symbol"].(string)
		if sym == "" {
			continue
		}
		ds, _ := m["date"].(string)
		if ds == "" {
			continue
		}
		d, err := time.Parse("2006-01-02", ds)
		if err != nil {
			continue
		}
		hour, _ := m["hour"].(string)
		yr := finnhubInt(m["year"])
		quarter := finnhubString(m["quarter"])
		epsE := finnhubFloat(m["epsEstimate"])
		epsA := finnhubFloat(m["epsActual"])
		revE := finnhubFloat(m["revenueEstimate"])
		revA := finnhubFloat(m["revenueActual"])
		extID := fmt.Sprintf("fh_earn_%s_%s_%s", sym, ds, quarter)
		if err := store.UpsertEarningsCalendarEvent(ctx, pool, d, sym, hour, yr, quarter, epsE, epsA, revE, revA, "finnhub", extID, m); err != nil {
			log.Error("upsert earnings", "err", err)
		}
	}
	log.Info("earnings rows processed", "batch", len(items))
}

func runFinnhubGeneral(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, fh *finnhub.Client) {
	items, err := fh.MarketNews(ctx, "general")
	if err != nil {
		log.Warn("finnhub general news", "err", err)
		return
	}
	for _, item := range items {
		headline, _ := item["headline"].(string)
		if headline == "" {
			continue
		}
		urlStr, _ := item["url"].(string)
		ts := time.Now().UTC()
		if ds, ok := item["datetime"].(float64); ok {
			sec := int64(ds)
			if sec > 1e12 {
				sec /= 1000
			}
			ts = time.Unix(sec, 0).UTC()
		}
		if err := store.InsertNews(ctx, pool, ts, "finnhub_macro_general", "", headline, urlStr, nil, item); err != nil {
			log.Error("insert macro general news", "err", err)
		}
	}
	log.Info("finnhub general macro news", "n", len(items))
}

func runRSS(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, feedURL string, max int) {
	items, err := rss.FetchItems(ctx, feedURL, max)
	if err != nil {
		log.Warn("rss fetch", "url", feedURL, "err", err)
		return
	}
	src := "rss_macro_" + shortHash(feedURL)
	for _, it := range items {
		if err := store.InsertNews(ctx, pool, it.PubDate, src, "", it.Title, it.Link, nil, map[string]any{"feed": feedURL}); err != nil {
			log.Error("insert rss", "err", err)
		}
	}
	log.Info("rss macro headlines", "feed", feedURL, "n", len(items))
}

func runGPR(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, csvURL string) {
	row, err := gpr.FetchLatestMonthly(ctx, csvURL)
	if err != nil {
		log.Warn("gpr fetch", "err", err)
		return
	}
	if err := store.UpsertGPRMonthly(ctx, pool, row.MonthTS, row.GPRTotal, row.GPRAct, row.GPRThreat, "gpr_csv", map[string]any{"raw_tail": row.Raw}); err != nil {
		log.Error("upsert gpr", "err", err)
		return
	}
	log.Info("gpr monthly stored", "month", row.MonthTS.Format("2006-01"))
}

func runGDELT(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, c *gdelt.Client, query string, maxRec int, lookback time.Duration) {
	res, err := c.FetchArtList(ctx, query, maxRec, lookback)
	if err != nil {
		log.Warn("gdelt", "err", err)
		return
	}
	n, avgT, avgG := gdelt.AggregateTone(res)
	dayTS := time.Now().UTC().Truncate(24 * time.Hour)
	label := "macro_query_default"
	if err := store.UpsertGDELTDaily(ctx, pool, dayTS, label, n, avgT, avgG, map[string]any{"query": query}); err != nil {
		log.Error("upsert gdelt", "err", err)
		return
	}
	log.Info("gdelt daily aggregate", "articles", n, "avg_tone", avgT)
}

func parseFinnhubTime(m map[string]any) time.Time {
	if t, ok := m["time"].(float64); ok {
		sec := int64(t)
		if sec > 1e12 {
			sec /= 1000
		}
		return time.Unix(sec, 0).UTC()
	}
	if s, ok := m["time"].(string); ok {
		layouts := []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", time.RFC3339, "2006-01-02"}
		for _, ly := range layouts {
			if tt, err := time.Parse(ly, s); err == nil {
				return tt.UTC()
			}
		}
	}
	return time.Now().UTC()
}

func finnhubFloat(v any) *float64 {
	switch x := v.(type) {
	case float64:
		return &x
	case string:
		x = strings.TrimSpace(x)
		if x == "" {
			return nil
		}
		f, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return nil
		}
		return &f
	default:
		return nil
	}
}

func finnhubString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		if x == float64(int(x)) {
			return strconv.Itoa(int(x))
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return ""
	}
}

func finnhubInt(v any) *int {
	switch x := v.(type) {
	case float64:
		i := int(x)
		return &i
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return nil
		}
		return &i
	default:
		return nil
	}
}

func externalID(prefix, a, b string, ts time.Time) string {
	raw := fmt.Sprintf("%s|%s|%s|%d", prefix, a, b, ts.Unix())
	h := sha256.Sum256([]byte(raw))
	return prefix + "_" + hex.EncodeToString(h[:12])
}

func shortHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:6])
}

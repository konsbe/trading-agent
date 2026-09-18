// Command catalyst-backfill is Step 8: it populates catalyst_events for a set of
// historical candidates so §3.11's catalyst tier can be EVALUATED rather than
// assumed.
//
// Catalyst is the one scoring component that has never been tested. It was null
// throughout every Step 7 run, is worth up to 15 of the 90 allocated points, and
// is a different kind of signal — news-driven — from the four price/volume
// components that were evaluated. Getting real evidence on it is cheap: a
// keyword classifier over news that Finnhub already serves, no new data source
// and no wider backfill.
//
// Input is the CSV emitted by `momentum-backtest -dump-candidates`, so the
// evaluation runs against exactly the rows the base rate was computed on rather
// than a re-derived approximation.
//
//	DATABASE_URL=... FINNHUB_API_KEY=... \
//	  go run ./cmd/catalyst-backfill -candidates /tmp/candidates.csv
//
// # The window Finnhub can actually serve
//
// Free-tier company news reaches back roughly 12 months (measured: a 2026-01
// window returns 243 AAPL articles, 2025-09 returns 0). Evaluable candidates are
// all at least 120 sessions old by construction, so only the more recent ones
// fall inside both constraints. This command reports that overlap explicitly
// instead of letting an out-of-range window look like a symbol with no news.
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/catalyst"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/db"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/finnhub"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/logx"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/ratelimit"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

type candidate struct {
	symbol string
	date   time.Time
	score  int
	bucket string
	gain   float64
	hit    bool
}

func main() {
	csvPath := flag.String("candidates", "", "CSV from momentum-backtest -dump-candidates (required)")
	keywords := flag.String("keywords", "", "JSON keyword vocabulary; empty uses §3.11's built-in table")
	windowHours := flag.Int("window-hours", 48, "§3.11 news window before the candidate date")
	newsMonths := flag.Int("news-months", 12, "how far back the provider serves news; candidates older than this are skipped")
	dryRun := flag.Bool("dry-run", false, "classify without writing catalyst_events")
	// §3.11 wants the vocabulary tunable without a rebuild. Tuning it is useless
	// if every iteration costs another ~80 provider requests, so the raw
	// headlines are cached and re-classification runs entirely offline.
	cachePath := flag.String("cache", "", "write fetched headlines here for offline re-classification")
	fromCache := flag.String("from-cache", "", "classify from a cache file instead of fetching")
	flag.Parse()

	if *csvPath == "" {
		fmt.Fprintln(os.Stderr, "-candidates is required")
		os.Exit(2)
	}

	log := logx.New(os.Getenv("LOG_LEVEL"))
	ctx := context.Background()

	cfg, err := catalyst.LoadConfig(*keywords)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cands, err := readCandidates(*csvPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read candidates:", err)
		os.Exit(1)
	}

	cutoff := time.Now().UTC().AddDate(0, -*newsMonths, 0)
	var inWindow, tooOld []candidate
	for _, c := range cands {
		if c.date.Before(cutoff) {
			tooOld = append(tooOld, c)
		} else {
			inWindow = append(inWindow, c)
		}
	}

	fmt.Printf("candidates: %d total, %d inside the ~%dm news window, %d too old to fetch\n",
		len(cands), len(inWindow), *newsMonths, len(tooOld))
	if len(inWindow) == 0 {
		fmt.Println("nothing to fetch — every candidate predates the provider's news history")
		return
	}

	// Offline path: re-classify a cached fetch. No provider calls, no database.
	if *fromCache != "" {
		cache, err := loadCache(*fromCache)
		if err != nil {
			fmt.Fprintln(os.Stderr, "load cache:", err)
			os.Exit(1)
		}
		byTier := map[catalyst.Tier][]candidate{}
		var withNews, kwMatched int
		for _, c := range inWindow {
			hs := cache[cacheKey(c)]
			if len(hs) > 0 {
				withNews++
			}
			res := catalyst.ClassifyWindow(hs, cfg)
			if res.KeywordMatchedHeadlines > 0 {
				kwMatched++
			}
			byTier[res.Tier] = append(byTier[res.Tier], c)
		}
		fmt.Printf("classified %d candidates from cache (%d with coverage, %d with a real keyword hit)\n",
			len(inWindow), withNews, kwMatched)
		reportCatalyst(inWindow, byTier, len(inWindow), withNews, kwMatched, true)
		return
	}

	pool, err := db.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "db connect:", err)
		os.Exit(1)
	}
	defer pool.Close()

	fh := finnhub.NewWithLimiter(os.Getenv("FINNHUB_API_KEY"), ratelimit.SharedFinnhub(ctx, pool, log))
	if !fh.HasToken() {
		fmt.Fprintln(os.Stderr, "FINNHUB_API_KEY not set")
		os.Exit(1)
	}

	var fetched, withNews, stored, kwMatched int
	tierCount := map[catalyst.Tier]int{}
	byTier := map[catalyst.Tier][]candidate{}
	cache := map[string][]string{}

	for i, c := range inWindow {
		from := c.date.Add(-time.Duration(*windowHours) * time.Hour)
		items, err := fh.CompanyNewsRange(ctx, c.symbol, from, c.date)
		if err != nil {
			log.Warn("company news fetch failed", "symbol", c.symbol, "date", c.date.Format(time.DateOnly), "err", err)
			continue
		}
		fetched++

		headlines := make([]string, 0, len(items))
		type article struct {
			headline, url, source string
		}
		arts := make([]article, 0, len(items))
		for _, it := range items {
			h := strAt(it, "headline")
			if strings.TrimSpace(h) == "" {
				continue
			}
			headlines = append(headlines, h)
			arts = append(arts, article{h, strAt(it, "url"), strAt(it, "source")})
		}
		if len(headlines) > 0 {
			withNews++
		}

		cache[cacheKey(c)] = headlines

		res := catalyst.ClassifyWindow(headlines, cfg)
		tierCount[res.Tier]++
		byTier[res.Tier] = append(byTier[res.Tier], c)
		if res.KeywordMatchedHeadlines > 0 {
			kwMatched++
		}

		if !*dryRun {
			// §3.11: store EVERY matched event, not just the winning tier —
			// Phase 2's most valuable analysis is which specific keywords preceded
			// real runners, and that needs the raw matches.
			for _, a := range arts {
				cl := catalyst.ClassifyHeadline(a.headline, cfg)
				for _, m := range cl.Matches {
					ev := store.CatalystEvent{
						TS:       c.date,
						Symbol:   c.symbol,
						Source:   a.source,
						Headline: a.headline,
						URL:      a.url,
						Keyword:  m.Keyword,
						Tier:     string(m.Tier),
						ScanTS:   c.date,
					}
					if err := store.UpsertCatalystEvent(ctx, pool, ev); err != nil {
						log.Error("upsert catalyst event", "symbol", c.symbol, "err", err)
						continue
					}
					stored++
				}
			}
		}

		if (i+1)%20 == 0 {
			fmt.Printf("  %d/%d fetched (tiers so far: A=%d B=%d none=%d)\n",
				i+1, len(inWindow), tierCount[catalyst.TierA], tierCount[catalyst.TierB], tierCount[catalyst.TierNone])
		}
	}

	if *cachePath != "" {
		if err := saveCache(*cachePath, cache); err != nil {
			log.Warn("cache write failed", "err", err)
		} else {
			fmt.Printf("cached %d candidate windows to %s\n", len(cache), *cachePath)
		}
	}

	fmt.Printf("\n  candidates with a real keyword hit: %d of %d fetched\n", kwMatched, fetched)
	reportCatalyst(inWindow, byTier, fetched, withNews, stored, *dryRun)
}

// cacheKey identifies one candidate's news window.
func cacheKey(c candidate) string {
	return c.symbol + "@" + c.date.Format(time.DateOnly)
}

func saveCache(path string, cache map[string][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(cache)
}

func loadCache(path string) (map[string][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out map[string][]string
	if err := json.NewDecoder(f).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func reportCatalyst(inWindow []candidate, byTier map[catalyst.Tier][]candidate, fetched, withNews, stored int, dry bool) {
	fmt.Println("\n═══ Step 8: catalyst tier evaluation ═══")
	fmt.Printf("  candidates fetched:   %d\n", fetched)
	fmt.Printf("  with any coverage:    %d\n", withNews)
	if dry {
		fmt.Println("  catalyst_events:      DRY RUN, nothing written")
	} else {
		fmt.Printf("  catalyst_events rows: %d\n", stored)
	}

	base := hitRate(inWindow)
	fmt.Printf("\n  base rate on this subset: %d/%d = %.2f%%\n", countHits(inWindow), len(inWindow), base)

	fmt.Printf("\n  %-6s %-6s %-9s %-8s %s\n", "tier", "n", "hit%", "lift", "reading")
	for _, t := range []catalyst.Tier{catalyst.TierA, catalyst.TierB, catalyst.TierNone} {
		g := byTier[t]
		if len(g) == 0 {
			fmt.Printf("  %-6s %-6d %-9s %-8s no candidates in this tier\n", t, 0, "—", "—")
			continue
		}
		hr := hitRate(g)
		lift := "—"
		if base > 0 {
			lift = fmt.Sprintf("%.2fx", hr/base)
		}
		fmt.Printf("  %-6s %-6d %-9.2f %-8s %s\n", t, len(g), hr, lift, powerNote(len(g), countHits(g)))
	}

	// §4.2 awards A=15 and B=8. Whether that ordering is right is exactly what
	// this measures, so it is stated as a comparison rather than assumed.
	a, b, none := byTier[catalyst.TierA], byTier[catalyst.TierB], byTier[catalyst.TierNone]
	fmt.Println("\n  ordering check — §4.2 assumes A > B > none:")
	if len(a) > 0 && len(b) > 0 {
		fmt.Printf("    A (%.2f%%) vs B (%.2f%%): %s\n", hitRate(a), hitRate(b), cmp(hitRate(a), hitRate(b)))
	}
	if len(b) > 0 && len(none) > 0 {
		fmt.Printf("    B (%.2f%%) vs none (%.2f%%): %s\n", hitRate(b), hitRate(none), cmp(hitRate(b), hitRate(none)))
	}

	fmt.Println("\n  ⚠ IN-SAMPLE and UNDERPOWERED. This subset is what remains after intersecting")
	fmt.Println("    §6's complete-label requirement with the provider's ~12-month news history,")
	fmt.Println("    so it is a fraction of the 218 and carries proportionally fewer hits. Treat")
	fmt.Println("    a tier ordering that matches §4.2 as consistent, not confirmed — and note")
	fmt.Println("    that the keyword vocabulary itself is untested, which is precisely why every")
	fmt.Println("    individual match is stored rather than only the tier.")
}

func powerNote(n, hits int) string {
	if n < 10 || hits < 3 {
		return fmt.Sprintf("underpowered (%d hits)", hits)
	}
	return fmt.Sprintf("%d hits", hits)
}

func cmp(x, y float64) string {
	switch {
	case x > y:
		return "consistent with the assumed ordering"
	case x < y:
		return "CONTRADICTS the assumed ordering"
	default:
		return "indistinguishable"
	}
}

func countHits(cs []candidate) int {
	n := 0
	for _, c := range cs {
		if c.hit {
			n++
		}
	}
	return n
}

func hitRate(cs []candidate) float64 {
	if len(cs) == 0 {
		return 0
	}
	return 100 * float64(countHits(cs)) / float64(len(cs))
}

func strAt(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func readCandidates(path string) ([]candidate, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	recs, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(recs) < 2 {
		return nil, fmt.Errorf("no rows in %s", path)
	}
	var out []candidate
	for _, rec := range recs[1:] {
		if len(rec) < 6 {
			continue
		}
		d, err := time.Parse(time.DateOnly, rec[1])
		if err != nil {
			continue
		}
		score, _ := strconv.Atoi(rec[2])
		gain, _ := strconv.ParseFloat(rec[4], 64)
		out = append(out, candidate{
			symbol: rec[0], date: d.UTC(), score: score, bucket: rec[3],
			gain: gain, hit: rec[5] == "true",
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].date.Before(out[j].date) })
	return out, nil
}

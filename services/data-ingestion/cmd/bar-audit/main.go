// Command bar-audit checks stored equity bars for corporate-action adjustment
// defects and reports them per symbol.
//
// It exists because a provider defect got all the way into the database once
// already: Twelve Data's adjust=all returned series that alternated bar by bar
// between adjusted and unadjusted values, fabricating one-day moves of up to
// +1382% across 17% of the pilot universe and 41% of its penny-price bucket.
// That was only found by comparing against a second provider. This tool applies
// internal/barquality so the same class of defect is detectable with one
// provider, before the scanner reads the rows.
//
// Run it after any backfill, and before trusting a new provider:
//
//	DATABASE_URL=... go run ./cmd/bar-audit -source twelve_data
//	DATABASE_URL=... go run ./cmd/bar-audit -source tiingo -all
//
// A non-zero exit means suspected defects were found, so it can gate a pipeline.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/barquality"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

func main() {
	source := flag.String("source", "", "equity_ohlcv.source to audit (required)")
	interval := flag.String("interval", "1Day", "equity_ohlcv.interval to audit")
	all := flag.Bool("all", false, "audit the whole universe instead of only the pilot subset")
	verbose := flag.Bool("v", false, "print every suspected break, not just a per-symbol summary")
	limit := flag.Int("examples", 15, "how many flagged symbols to list")
	flag.Parse()

	if *source == "" {
		fmt.Fprintln(os.Stderr, "-source is required (e.g. tiingo, twelve_data, yahoo_finance)")
		os.Exit(2)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "db connect:", err)
		os.Exit(2)
	}
	defer pool.Close()

	bySymbol, err := store.LoadBarsBySource(ctx, pool, *interval, *source, !*all)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load bars:", err)
		os.Exit(2)
	}
	if len(bySymbol) == 0 {
		fmt.Printf("no bars stored for source=%s interval=%s\n", *source, *interval)
		return
	}

	cfg := barquality.DefaultConfig()
	type finding struct {
		symbol string
		breaks []barquality.Break
	}
	var findings []finding
	totalBars, totalBreaks := 0, 0

	for sym, rows := range bySymbol {
		totalBars += len(rows)
		bars := make([]barquality.Bar, len(rows))
		for i, r := range rows {
			bars[i] = barquality.Bar{TS: r.TS, Open: r.Open, High: r.High, Low: r.Low, Close: r.Close, Volume: r.Volume}
		}
		if b := barquality.DetectAdjustmentBreaks(bars, cfg); len(b) > 0 {
			findings = append(findings, finding{sym, b})
			totalBreaks += len(b)
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if len(findings[i].breaks) != len(findings[j].breaks) {
			return len(findings[i].breaks) > len(findings[j].breaks)
		}
		return findings[i].symbol < findings[j].symbol
	})

	scope := "pilot subset"
	if *all {
		scope = "whole universe"
	}
	fmt.Printf("bar-audit: source=%s interval=%s scope=%s\n", *source, *interval, scope)
	fmt.Printf("  symbols audited:      %d (%d bars)\n", len(bySymbol), totalBars)
	fmt.Printf("  symbols with defects: %d (%.1f%%)\n",
		len(findings), 100*float64(len(findings))/float64(len(bySymbol)))
	fmt.Printf("  suspected breaks:     %d\n", totalBreaks)

	shown := *limit
	if shown > len(findings) {
		shown = len(findings)
	}
	for _, f := range findings[:shown] {
		fmt.Printf("  %-8s %d break(s)\n", f.symbol, len(f.breaks))
		if *verbose {
			for _, b := range f.breaks {
				fmt.Printf("      %s\n", b.Reason())
			}
		} else if len(f.breaks) > 0 {
			fmt.Printf("      %s\n", f.breaks[0].Reason())
		}
	}
	if len(findings) > shown {
		fmt.Printf("  ... and %d more\n", len(findings)-shown)
	}

	if len(findings) > 0 {
		// Non-zero so this can gate a pipeline rather than only inform a human.
		os.Exit(1)
	}
}

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/edgar"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

// source20F / metric20F: the IFRS cash-flow statement of a 20-F filer, read
// from SEC EDGAR companyfacts (ifrs-full), one row per fiscal year. Values are
// in the filing's reporting currency, never converted.
const (
	source20F = "sec_edgar_20f"
	metric20F = "cash_flow_statement"
)

// payload20F is the stored row: the statement plus the newest 20-F on file,
// so the reader can say when a newer 20-F is not in companyfacts yet (TSM's
// FY2025 20-F, filed 2026-04-16, was missing from it on 2026-09-27).
type payload20F struct {
	edgar.CashFlow20F
	CIK          string        `json:"cik"`
	Latest20F    *edgar.Filing `json:"latest_20f"`
	ConceptsNote string        `json:"concepts_note"`
}

// store20FCashFlow runs for a symbol Finnhub returned no 10-K/10-Q for. A
// symbol SEC does not list, or with no ifrs-full cash-flow facts (a fund, a
// US-GAAP 10-K filer), stores nothing.
func (w *worker) store20FCashFlow(ctx context.Context, sym string) {
	if w.ed == nil {
		return
	}
	if err := w.fetch20FCashFlow(ctx, sym); err != nil {
		w.log.Warn("sec edgar 20-F cash flow", "symbol", sym, "err", err)
	}
}

func (w *worker) fetch20FCashFlow(ctx context.Context, sym string) error {
	cik, err := w.ed.CIK(ctx, sym)
	if err != nil {
		return fmt.Errorf("cik: %w", err)
	}
	if cik == "" {
		w.log.Debug("sec edgar: no CIK", "symbol", sym)
		return nil
	}
	body, err := w.ed.CompanyFacts(ctx, cik)
	if err != nil {
		return fmt.Errorf("companyfacts: %w", err)
	}
	cf, err := edgar.ParseCashFlow20F(body)
	if err != nil {
		return err
	}
	if cf == nil {
		w.log.Debug("sec edgar: no ifrs-full cash-flow facts", "symbol", sym, "cik", cik)
		return nil
	}
	latest, err := w.ed.Latest20F(ctx, cik)
	if err != nil {
		w.log.Warn("sec edgar submissions", "symbol", sym, "err", err) // the statement is still stored
	}
	p := payload20F{
		CashFlow20F: *cf, CIK: cik, Latest20F: latest,
		ConceptsNote: "standard ifrs-full concepts only; a line the filer reports under its own extension or folds into a broader line is absent",
	}
	if err := store.UpsertFundamental(ctx, w.pool, time.Now().UTC(), sym,
		fmt.Sprintf("annual_%d", cf.FiscalYear), metric20F, nil, p, source20F); err != nil {
		return fmt.Errorf("store: %w", err)
	}
	w.log.Info("sec edgar 20-F cash flow stored", "symbol", sym, "fiscal_year", cf.FiscalYear,
		"currency", cf.Currency, "lines", len(cf.Lines), "filed", cf.Filed)
	return nil
}

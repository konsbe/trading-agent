package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/config"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/alphavantage"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/finnhub"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.LoadFundamental()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	lvl := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		lvl = slog.LevelDebug
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))

	if !cfg.EnableMetrics && !cfg.EnableFinancials && !cfg.EnableEarnings {
		log.Info("all fundamental fetchers disabled; exiting")
		return
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	fh := finnhub.New(cfg.FinnhubKey)
	if !fh.HasToken() {
		log.Warn("FINNHUB_API_KEY not set; fundamental fetching disabled")
		return
	}

	av := alphavantage.New(cfg.AlphaVantageKey)
	if !av.HasKey() {
		log.Warn("ALPHA_VANTAGE_API_KEY not set; forward P/E and sector data will be missing")
	}

	w := &worker{cfg: cfg, pool: pool, fh: fh, av: av, log: log}

	// Run immediately on startup, then on ticker cadence.
	if cfg.EnableMetrics {
		w.runMetrics(context.Background())
	}
	if cfg.EnableFinancials {
		w.runFinancials(context.Background())
	}
	if cfg.EnableEarnings {
		w.runEarnings(context.Background())
	}
	if cfg.EnableOverview && av.HasKey() {
		w.runOverview(context.Background())
	}

	tMetrics := time.NewTicker(cfg.PollMetrics)
	tFinancials := time.NewTicker(cfg.PollFinancials)
	tEarnings := time.NewTicker(cfg.PollEarnings)
	tOverview := time.NewTicker(cfg.PollOverview)
	defer tMetrics.Stop()
	defer tFinancials.Stop()
	defer tEarnings.Stop()
	defer tOverview.Stop()

	for {
		select {
		case <-tMetrics.C:
			if cfg.EnableMetrics {
				w.runMetrics(context.Background())
			}
		case <-tFinancials.C:
			if cfg.EnableFinancials {
				w.runFinancials(context.Background())
			}
		case <-tEarnings.C:
			if cfg.EnableEarnings {
				w.runEarnings(context.Background())
			}
		case <-tOverview.C:
			if cfg.EnableOverview && av.HasKey() {
				w.runOverview(context.Background())
			}
		}
	}
}

type worker struct {
	cfg  config.Fundamental
	pool *pgxpool.Pool
	fh   *finnhub.Client
	av   *alphavantage.Client
	log  *slog.Logger
}

// ─── Tier 1: Key metrics (TTM ratios) ─────────────────────────────────────────
//
// Finnhub /stock/metric?metric=all returns a large flat map under the "metric" key.
// We extract the Tier 1 FA scalars and store each as an individual row so the
// signal layer can query by metric name just like technical_indicators.

func (w *worker) runMetrics(ctx context.Context) {
	ts := time.Now().UTC()
	for _, sym := range w.cfg.Symbols {
		raw, err := w.fh.Metrics(ctx, sym)
		if err != nil {
			w.log.Error("finnhub metrics", "symbol", sym, "err", err)
			continue
		}
		metricMap, ok := raw["metric"].(map[string]any)
		if !ok {
			w.log.Warn("finnhub metrics: unexpected response shape", "symbol", sym)
			continue
		}

		upsert := func(metric string, value *float64, payload any) {
			if err := store.UpsertFundamental(ctx, w.pool, ts, sym, "ttm", metric, value, payload, "finnhub_metric"); err != nil {
				w.log.Error("upsert fundamental", "metric", metric, "symbol", sym, "err", err)
			}
		}

		// ── EPS & Earnings Growth ─────────────────────────────────────────────
		upsert("eps_ttm", floatPtr(metricMap, "epsBasicExclExtraItemsTTM"), nil)
		upsert("eps_annual", floatPtr(metricMap, "epsBasicExclExtraItemsAnnual"), nil)
		upsert("eps_growth_3y", floatPtr(metricMap, "epsGrowth3Y"), nil)
		upsert("eps_growth_5y", floatPtr(metricMap, "epsGrowth5Y"), nil)
		upsert("eps_growth_ttm_yoy", floatPtr(metricMap, "epsGrowthTTMYoy"), nil)
		upsert("eps_growth_quarterly_yoy", floatPtr(metricMap, "epsGrowthQuarterlyYoy"), nil)

		// ── Revenue & Revenue Growth ──────────────────────────────────────────
		upsert("revenue_ttm", floatPtr(metricMap, "revenueTTM"), nil)
		upsert("revenue_per_share_ttm", floatPtr(metricMap, "revenuePerShareTTM"), nil)
		upsert("revenue_growth_3y", floatPtr(metricMap, "revenueGrowth3Y"), nil)
		upsert("revenue_growth_5y", floatPtr(metricMap, "revenueGrowth5Y"), nil)
		upsert("revenue_growth_ttm_yoy", floatPtr(metricMap, "revenueGrowthTTMYoy"), nil)
		upsert("revenue_growth_quarterly_yoy", floatPtr(metricMap, "revenueGrowthQuarterlyYoy"), nil)

		// ── P/E Ratio ─────────────────────────────────────────────────────────
		pe := floatPtr(metricMap, "peBasicExclExtraTTM")
		if pe == nil {
			pe = floatPtr(metricMap, "peTTM")
		}
		upsert("pe_ratio_ttm", pe, nil)
		upsert("pe_ratio_annual", floatPtr(metricMap, "peExclExtraAnnual"), nil)
		upsert("pe_ratio_5y_avg", floatPtr(metricMap, "peExclExtraNormalizedAnnual"), nil)
		// Forward P/E: Finnhub free tier does not expose forward estimates in /metric;
		// stored as nil so the row exists and downstream knows it was queried.
		upsert("pe_ratio_forward", nil, map[string]any{
			"note": "Forward P/E not available on Finnhub free tier; requires /stock/eps-estimates (paid).",
		})

		// ── Free Cash Flow & FCF Yield ────────────────────────────────────────
		upsert("fcf_ttm", floatPtr(metricMap, "freeCashFlowTTM"), nil)
		upsert("fcf_per_share_ttm", floatPtr(metricMap, "freeCashFlowPerShareTTM"), nil)
		upsert("fcf_yield_1y", floatPtr(metricMap, "freeCashFlowYield1Y"), nil)
		upsert("fcf_yield_5y", floatPtr(metricMap, "fcfMargin5Y"), nil)

		// ── Profit Margins ────────────────────────────────────────────────────
		upsert("gross_margin_ttm", floatPtr(metricMap, "grossMarginTTM"), nil)
		upsert("gross_margin_annual", floatPtr(metricMap, "grossMarginAnnual"), nil)
		upsert("gross_margin_5y", floatPtr(metricMap, "grossMargin5Y"), nil)
		upsert("operating_margin_ttm", floatPtr(metricMap, "operatingMarginTTM"), nil)
		upsert("operating_margin_annual", floatPtr(metricMap, "operatingMarginAnnual"), nil)
		upsert("net_margin_ttm", floatPtr(metricMap, "netProfitMarginTTM"), nil)
		upsert("net_margin_annual", floatPtr(metricMap, "netProfitMarginAnnual"), nil)
		upsert("net_margin_5y", floatPtr(metricMap, "netProfitMargin5Y"), nil)

		// ── Market cap & shares (needed to derive FCF yield locally) ─────────
		upsert("market_cap", floatPtr(metricMap, "marketCapitalization"), nil)
		upsert("shares_outstanding", floatPtr(metricMap, "shareOutstanding"), nil)

		// ── Tier 2: Return on capital & profitability efficiency ───────────────
		// ROIC and ROE are the master profitability metrics (Tier 2, rank 06).
		// ROE above 15% sustained = Buffett-style quality / moat signal.
		// TODO: Python — compute ROIC = NOPAT ÷ Invested Capital from XBRL series.
		upsert("roe_ttm", floatPtr(metricMap, "roeTTM"), nil)
		upsert("roa_ttm", floatPtr(metricMap, "roaTTM"), nil)
		upsert("roic_5y", floatPtr(metricMap, "roic5Y"), nil)
		upsert("ebitda_per_share_ttm", floatPtr(metricMap, "ebitdaPerShareTTM"), nil)

		// ── Tier 2: Leverage — Debt/Equity (rank 07) ───────────────────────────
		// D/E above 2× demands scrutiny; industry context essential.
		// netDebtAnnual from Finnhub = Total Debt – Cash (used for Net Debt/EBITDA).
		upsert("debt_to_equity_quarterly", floatPtr(metricMap, "totalDebtToEquityQuarterly"), nil)
		upsert("debt_to_equity_annual", floatPtr(metricMap, "totalDebtToEquityAnnual"), nil)
		upsert("net_debt_annual", floatPtr(metricMap, "netDebtAnnual"), nil)

		// ── Tier 2: Liquidity — Current & Quick ratio (rank 10) ───────────────
		// Current <1.0 means short-term liabilities exceed liquid assets.
		upsert("current_ratio_quarterly", floatPtr(metricMap, "currentRatioQuarterly"), nil)
		upsert("current_ratio_annual", floatPtr(metricMap, "currentRatioAnnual"), nil)
		upsert("quick_ratio_quarterly", floatPtr(metricMap, "quickRatioQuarterly"), nil)

		// ── Tier 2: Per-share book value (used with market_cap for local P/B) ─
		upsert("book_value_per_share_quarterly", floatPtr(metricMap, "bookValuePerShareQuarterly"), nil)
		upsert("book_value_per_share_annual", floatPtr(metricMap, "bookValuePerShareAnnual"), nil)

		// ── Also store the full metric payload for forward-compat access ──────
		if err := store.UpsertFundamental(ctx, w.pool, ts, sym, "ttm", "metrics_raw", nil, metricMap, "finnhub_metric"); err != nil {
			w.log.Error("upsert metrics_raw", "symbol", sym, "err", err)
		}

		w.log.Info("fundamentals metrics stored", "symbol", sym)
	}
}

// ─── Detailed financials (income statement + cash flow) ────────────────────────
//
// Finnhub /stock/financials-reported returns the most recent N filed reports.
// We store quarterly (10-Q) and optionally annual (10-K) reports separately so
// the analyzer can compute 8-quarter margin trends and year-over-year comparisons.
//
// XBRL concept names differ between 10-Q and 10-K filings. Each conceptVal call
// lists the most common alternatives in priority order to handle company-specific
// tags (e.g. Apple uses RevenueFromContractWithCustomerExcludingAssessedTax).

func (w *worker) runFinancials(ctx context.Context) {
	w.storeFinancials(context.Background(), w.cfg.FinancialsFreq, w.cfg.FinancialsLimit)
	if w.cfg.EnableAnnualFinancials && w.cfg.FinancialsFreq != "annual" {
		w.storeFinancials(context.Background(), "annual", w.cfg.AnnualFinancialsLimit)
	}
}

func (w *worker) storeFinancials(ctx context.Context, freq string, limit int) {
	ts := time.Now().UTC()
	for _, sym := range w.cfg.Symbols {
		raw, err := w.fh.FinancialsReported(ctx, sym, freq)
		if err != nil {
			w.log.Error("finnhub financials-reported", "symbol", sym, "freq", freq, "err", err)
			continue
		}

		reports, _ := raw["data"].([]any)
		if len(reports) == 0 {
			w.log.Warn("finnhub financials-reported: no reports", "symbol", sym, "freq", freq)
			continue
		}

		if limit <= 0 || limit > len(reports) {
			limit = len(reports)
		}

		source := "finnhub_financials_reported"
		stored := 0

		for i := 0; i < limit; i++ {
			report, ok := reports[i].(map[string]any)
			if !ok {
				continue
			}

			period := financialPeriodLabel(report)

			upsert := func(metric string, value *float64, payload any) {
				if err := store.UpsertFundamental(ctx, w.pool, ts, sym, period, metric, value, payload, source); err != nil {
					w.log.Error("upsert fundamental", "metric", metric, "symbol", sym, "period", period, "err", err)
				}
			}

			reportBody, _ := report["report"].(map[string]any)
			ic := statementMap(reportBody, "ic") // income statement
			cf := statementMap(reportBody, "cf") // cash flow statement
			bs := statementMap(reportBody, "bs") // balance sheet

			// Income statement — concept names vary between 10-Q and 10-K filings.
			// Apple 10-K uses RevenueFromContractWithCustomerExcludingAssessedTax.
			upsert("revenue_reported", conceptVal(ic,
				"RevenueFromContractWithCustomerExcludingAssessedTax",
				"RevenueFromContractWithCustomerIncludingAssessedTax",
				"Revenues", "Revenue", "SalesRevenueNet",
				"SalesRevenueGoodsNet", "TotalRevenues",
			), nil)
			upsert("gross_profit_reported", conceptVal(ic,
				"GrossProfit",
			), nil)
			upsert("operating_income_reported", conceptVal(ic,
				"OperatingIncomeLoss", "OperatingIncome",
				"IncomeLossFromContinuingOperationsBeforeIncomeTaxesExtraordinaryItemsNoncontrollingInterest",
			), nil)
			upsert("net_income_reported", conceptVal(ic,
				"NetIncomeLoss", "ProfitLoss", "NetIncome",
				"NetIncomeLossAvailableToCommonStockholdersBasic",
			), nil)
			upsert("eps_diluted_reported", conceptVal(ic,
				"EarningsPerShareDiluted", "EarningsPerShareBasic",
				"IncomeLossFromContinuingOperationsPerDilutedShare",
			), nil)
			upsert("eps_basic_reported", conceptVal(ic,
				"EarningsPerShareBasic",
				"IncomeLossFromContinuingOperationsPerBasicShare",
			), nil)

			// Cash flow.
			upsert("operating_cf_reported", conceptVal(cf,
				"NetCashProvidedByUsedInOperatingActivities",
				"NetCashProvidedByOperatingActivities",
			), nil)
			upsert("capex_reported", conceptVal(cf,
				"PaymentsToAcquirePropertyPlantAndEquipment",
				"CapitalExpenditures",
				"AcquisitionsOfPropertyPlantAndEquipment",
				"PurchaseOfPropertyPlantAndEquipment",
			), nil)
			opCF := conceptVal(cf,
				"NetCashProvidedByUsedInOperatingActivities",
				"NetCashProvidedByOperatingActivities",
			)
			capex := conceptVal(cf,
				"PaymentsToAcquirePropertyPlantAndEquipment",
				"CapitalExpenditures",
				"AcquisitionsOfPropertyPlantAndEquipment",
				"PurchaseOfPropertyPlantAndEquipment",
			)
			if opCF != nil && capex != nil {
				fcf := *opCF - abs(*capex)
				upsert("fcf_reported", &fcf, map[string]any{
					"operating_cf": *opCF,
					"capex":        *capex,
					"note":         "fcf = operating_cf - abs(capex)",
				})
			}

			// Balance sheet.
			upsert("total_assets_reported", conceptVal(bs,
				"Assets",
			), nil)
			upsert("total_liabilities_reported", conceptVal(bs,
				"Liabilities", "LiabilitiesAndStockholdersEquity",
			), nil)
			upsert("total_equity_reported", conceptVal(bs,
				"StockholdersEquity", "Equity",
				"StockholdersEquityIncludingPortionAttributableToNoncontrollingInterest",
			), nil)
			upsert("total_debt_reported", conceptVal(bs,
				"LongTermDebt", "LongTermDebtNoncurrent",
				"LongTermDebtAndCapitalLeaseObligations",
				"DebtAndCapitalLeaseObligations",
			), nil)
			upsert("cash_reported", conceptVal(bs,
				"CashAndCashEquivalentsAtCarryingValue",
				"CashAndCashEquivalents", "Cash",
				"CashCashEquivalentsAndShortTermInvestments",
			), nil)

			// ── Tier 3: Goodwill & Intangible Assets (rank 18) ───────────────
			// Goodwill arises from acquisitions paying above book value. Heavy
			// goodwill (>40% of total assets) carries impairment risk.
			// XBRL concept names vary by filer; list most common in priority order.
			upsert("goodwill_reported", conceptVal(bs,
				"Goodwill", "GoodwillNet",
				"BusinessAcquisitionCostOfAcquiredEntityPurchasePrice",
			), nil)
			upsert("intangible_assets_reported", conceptVal(bs,
				"IntangibleAssetsNetExcludingGoodwill",
				"FiniteLivedIntangibleAssetsNet",
				"IntangibleAssetsNet",
				"OtherIntangibleAssetsNet",
			), nil)

			// ── Tier 3: Inventory (rank 16 — inventory turnover) ─────────────
			// Slowing inventory turnover signals weakening demand before revenue drops.
			upsert("inventory_reported", conceptVal(bs,
				"InventoryNet", "Inventories",
				"FIFOInventoryAmount", "InventoryFinishedGoods",
				"InventoryRawMaterialsAndSupplies",
			), nil)

			// ── Tier 3: Interest Expense (rank 15 — interest coverage) ───────
			// Interest coverage = EBIT / Interest Expense.
			// Interest expense is typically negative in XBRL; we take abs() in the analyzer.
			upsert("interest_expense_reported", conceptVal(ic,
				"InterestExpense",
				"InterestAndDebtExpense",
				"InterestExpenseDebt",
				"FinanceLeaseInterestExpense",
			), nil)

			// Raw report payload for forward-compat access.
			upsert("report_raw", nil, report)
			stored++
		}

		w.log.Info("fundamentals financials stored", "symbol", sym, "freq", freq, "count", stored)
	}
}

// ─── Earnings history (EPS actuals vs estimates) ──────────────────────────────

func (w *worker) runEarnings(ctx context.Context) {
	ts := time.Now().UTC()
	for _, sym := range w.cfg.Symbols {
		items, err := w.fh.Earnings(ctx, sym)
		if err != nil {
			w.log.Error("finnhub earnings", "symbol", sym, "err", err)
			continue
		}
		if len(items) == 0 {
			w.log.Warn("finnhub earnings: no data", "symbol", sym)
			continue
		}

		source := "finnhub_earnings"

		// Store the most recent quarter prominently, then archive all quarters.
		for i, item := range items {
			period := earningsPeriodLabel(item)
			if period == "" {
				period = fmt.Sprintf("earnings_idx_%d", i)
			}

			actual := floatPtr(item, "actual")
			estimate := floatPtr(item, "estimate")

			var surprise *float64
			if actual != nil && estimate != nil && *estimate != 0 {
				s := (*actual - *estimate) / abs(*estimate) * 100
				surprise = &s
			}

			if err := store.UpsertFundamental(ctx, w.pool, ts, sym, period, "eps_actual", actual, nil, source); err != nil {
				w.log.Error("upsert eps_actual", "symbol", sym, "err", err)
			}
			if err := store.UpsertFundamental(ctx, w.pool, ts, sym, period, "eps_estimate", estimate, nil, source); err != nil {
				w.log.Error("upsert eps_estimate", "symbol", sym, "err", err)
			}
			if surprise != nil {
				if err := store.UpsertFundamental(ctx, w.pool, ts, sym, period, "eps_surprise_pct", surprise, map[string]any{
					"actual":   actual,
					"estimate": estimate,
				}, source); err != nil {
					w.log.Error("upsert eps_surprise", "symbol", sym, "err", err)
				}
			}

			// Full quarter payload.
			if err := store.UpsertFundamental(ctx, w.pool, ts, sym, period, "earnings_raw", actual, item, source); err != nil {
				w.log.Error("upsert earnings_raw", "symbol", sym, "err", err)
			}
		}

		w.log.Info("fundamentals earnings stored", "symbol", sym, "quarters", len(items))
	}
}

// ─── Alpha Vantage: forward estimates, sector, beta ───────────────────────────
//
// Alpha Vantage COMPANY_OVERVIEW returns data that Finnhub's free tier omits:
//   - ForwardPE / forward EPS estimates
//   - Beta (market sensitivity)
//   - Sector & Industry (for relative P/E context in analyst-bot)
//   - Analyst target price (consensus upside/downside)
//   - PEG ratio (P/E ÷ growth rate — <1 is undervalued relative to growth)
//
// Free tier: 25 requests/day. For ≤5 symbols with a 7-day poll interval
// (default) this uses at most 5 calls/week, well within limits.

func (w *worker) runOverview(ctx context.Context) {
	ts := time.Now().UTC()
	source := "alphavantage_overview"

	for _, sym := range w.cfg.Symbols {
		data, err := w.av.Overview(ctx, sym)
		if err != nil {
			w.log.Warn("alpha vantage overview", "symbol", sym, "err", err)
			continue
		}

		upsert := func(metric string, value *float64, payload any) {
			if err := store.UpsertFundamental(ctx, w.pool, ts, sym, "ttm", metric, value, payload, source); err != nil {
				w.log.Error("upsert fundamental", "metric", metric, "symbol", sym, "err", err)
			}
		}

		// ── Forward valuation (the key Finnhub free-tier gap) ────────────────
		upsert("forward_pe", alphavantage.FloatField(data, "ForwardPE"), map[string]any{
			"source_field": "ForwardPE",
			"note":         "Price ÷ next-12M EPS estimate (analyst consensus)",
		})
		upsert("peg_ratio", alphavantage.FloatField(data, "PEGRatio"), map[string]any{
			"note": "P/E ÷ EPS growth rate. <1 = undervalued relative to growth.",
		})
		upsert("analyst_target_price", alphavantage.FloatField(data, "AnalystTargetPrice"), nil)

		// ── Market / risk metrics ─────────────────────────────────────────────
		upsert("beta", alphavantage.FloatField(data, "Beta"), map[string]any{
			"note": ">1 = more volatile than market; <1 = defensive",
		})
		upsert("price_to_book", alphavantage.FloatField(data, "PriceToBookRatio"), nil)
		upsert("ev_to_ebitda", alphavantage.FloatField(data, "EVToEBITDA"), nil)

		// ── Dividend context ──────────────────────────────────────────────────
		upsert("dividend_yield", alphavantage.FloatField(data, "DividendYield"), nil)
		upsert("payout_ratio", alphavantage.FloatField(data, "PayoutRatio"), nil)

		// ── Quarterly growth (cross-validates Finnhub) ────────────────────────
		upsert("quarterly_earnings_growth_yoy_av", alphavantage.FloatField(data, "QuarterlyEarningsGrowthYOY"), map[string]any{
			"note": "Alpha Vantage quarterly EPS YoY — cross-check vs Finnhub eps_growth_quarterly_yoy",
		})
		upsert("quarterly_revenue_growth_yoy_av", alphavantage.FloatField(data, "QuarterlyRevenueGrowthYOY"), nil)

		// ── 52-week range context for P/E normalisation ───────────────────────
		upsert("week52_high", alphavantage.FloatField(data, "52WeekHigh"), nil)
		upsert("week52_low", alphavantage.FloatField(data, "52WeekLow"), nil)
		upsert("ma_50d", alphavantage.FloatField(data, "50DayMovingAverage"), nil)
		upsert("ma_200d", alphavantage.FloatField(data, "200DayMovingAverage"), nil)

		// ── Sector/industry stored as payload (string, not float) ─────────────
		sector := alphavantage.StringField(data, "Sector")
		industry := alphavantage.StringField(data, "Industry")
		if sector != "" {
			if err := store.UpsertFundamental(ctx, w.pool, ts, sym, "ttm", "sector_profile", nil,
				map[string]any{
					"sector":       sector,
					"industry":     industry,
					"asset_type":   alphavantage.StringField(data, "AssetType"),
					"fiscal_year":  alphavantage.StringField(data, "FiscalYearEnd"),
					"latest_qtr":   alphavantage.StringField(data, "LatestQuarter"),
				}, source); err != nil {
				w.log.Error("upsert sector_profile", "symbol", sym, "err", err)
			}
		}

		w.log.Info("alpha vantage overview stored",
			"symbol", sym,
			"sector", sector,
			"forward_pe", alphavantage.FloatField(data, "ForwardPE"),
		)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// floatPtr extracts a float64 pointer from a map[string]any by key.
// Returns nil if the key is absent or the value is not a number.
func floatPtr(m map[string]any, key string) *float64 {
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch x := v.(type) {
	case float64:
		return &x
	case int:
		f := float64(x)
		return &f
	}
	return nil
}

// conceptVal searches a list of XBRL concept names in the flat IC/CF/BS map
// and returns the first non-nil match. Finnhub normalises many concept names.
func conceptVal(m map[string]any, concepts ...string) *float64 {
	for _, c := range concepts {
		if v := floatPtr(m, c); v != nil {
			return v
		}
	}
	return nil
}

// statementMap returns the inner map for a statement type ("ic", "cf", "bs")
// from the report body. Finnhub may return either an object or an array of
// concept rows; we flatten arrays into a name→value map.
func statementMap(reportBody map[string]any, key string) map[string]any {
	if reportBody == nil {
		return map[string]any{}
	}
	raw, ok := reportBody[key]
	if !ok {
		return map[string]any{}
	}
	// Already a flat map.
	if m, ok := raw.(map[string]any); ok {
		return m
	}
	// Array of {concept, label, unit, value} objects.
	items, ok := raw.([]any)
	if !ok {
		return map[string]any{}
	}
	out := make(map[string]any, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		concept, _ := row["concept"].(string)
		if concept == "" {
			concept, _ = row["label"].(string)
		}
		if concept != "" {
			out[concept] = row["value"]
		}
	}
	return out
}

// financialPeriodLabel builds a period string like "q_2024Q3" or "annual_2024".
func financialPeriodLabel(report map[string]any) string {
	freq, _ := report["freq"].(string)
	year, _ := report["year"].(float64)
	quarter, _ := report["quarter"].(float64)
	if freq == "quarterly" && quarter > 0 {
		return fmt.Sprintf("q_%dQ%d", int(year), int(quarter))
	}
	if year > 0 {
		return fmt.Sprintf("annual_%d", int(year))
	}
	return "unknown"
}

// earningsPeriodLabel builds a period string from an earnings item, e.g. "q_2024Q3".
func earningsPeriodLabel(item map[string]any) string {
	p, _ := item["period"].(string) // Finnhub returns "2024-09-30" style
	if p != "" {
		return "q_" + p
	}
	return ""
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

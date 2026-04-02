package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
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
	if cfg.EnableRecommendation {
		w.runRecommendations(context.Background())
	}
	if cfg.EnableInsiderTransactions {
		w.runInsiderTransactions(context.Background())
	}
	if cfg.EnableNewsSentiment && av.HasKey() {
		w.runNewsSentiment(context.Background())
	}
	if cfg.EnableInstitutionalOwnership {
		w.runInstitutionalOwnership(context.Background())
	}

	tMetrics := time.NewTicker(cfg.PollMetrics)
	tFinancials := time.NewTicker(cfg.PollFinancials)
	tEarnings := time.NewTicker(cfg.PollEarnings)
	tOverview := time.NewTicker(cfg.PollOverview)
	tRecommendation := time.NewTicker(cfg.PollRecommendation)
	tInsider := time.NewTicker(cfg.PollInsiderTransactions)
	tNewsSentiment := time.NewTicker(cfg.PollNewsSentiment)
	tInstitutional := time.NewTicker(cfg.PollInstitutionalOwnership)
	defer tMetrics.Stop()
	defer tFinancials.Stop()
	defer tEarnings.Stop()
	defer tOverview.Stop()
	defer tRecommendation.Stop()
	defer tInsider.Stop()
	defer tNewsSentiment.Stop()
	defer tInstitutional.Stop()

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
		case <-tRecommendation.C:
			if cfg.EnableRecommendation {
				w.runRecommendations(context.Background())
			}
		case <-tInsider.C:
			if cfg.EnableInsiderTransactions {
				w.runInsiderTransactions(context.Background())
			}
		case <-tNewsSentiment.C:
			if cfg.EnableNewsSentiment && av.HasKey() {
				w.runNewsSentiment(context.Background())
			}
		case <-tInstitutional.C:
			if cfg.EnableInstitutionalOwnership {
				w.runInstitutionalOwnership(context.Background())
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
		// All dollar amounts are divided by 1e6 (divM) to match Finnhub metric API scale (millions).
		upsert("revenue_reported", divM(conceptVal(ic,
			"RevenueFromContractWithCustomerExcludingAssessedTax",
			"RevenueFromContractWithCustomerIncludingAssessedTax",
			"Revenues", "Revenue", "SalesRevenueNet",
			"SalesRevenueGoodsNet", "TotalRevenues",
		)), nil)
		upsert("gross_profit_reported", divM(conceptVal(ic,
			"GrossProfit",
		)), nil)
		upsert("operating_income_reported", divM(conceptVal(ic,
			"OperatingIncomeLoss", "OperatingIncome",
			"IncomeLossFromContinuingOperationsBeforeIncomeTaxesExtraordinaryItemsNoncontrollingInterest",
		)), nil)
		upsert("net_income_reported", divM(conceptVal(ic,
			"NetIncomeLoss", "ProfitLoss", "NetIncome",
			"NetIncomeLossAvailableToCommonStockholdersBasic",
		)), nil)
		// EPS values are per-share — no unit conversion needed.
		upsert("eps_diluted_reported", conceptVal(ic,
			"EarningsPerShareDiluted", "EarningsPerShareBasic",
			"IncomeLossFromContinuingOperationsPerDilutedShare",
		), nil)
		upsert("eps_basic_reported", conceptVal(ic,
			"EarningsPerShareBasic",
			"IncomeLossFromContinuingOperationsPerBasicShare",
		), nil)
		// Weighted-average diluted shares — divide by 1e6 to store in millions.
		upsert("shares_wa_reported", divM(conceptVal(ic,
			"WeightedAverageNumberOfDilutedSharesOutstanding",
			"WeightedAverageNumberOfSharesOutstandingBasic",
		)), nil)

		// Cash flow — all in raw dollars; convert to millions.
		upsert("operating_cf_reported", divM(conceptVal(cf,
			"NetCashProvidedByUsedInOperatingActivities",
			"NetCashProvidedByOperatingActivities",
		)), nil)
		upsert("capex_reported", divM(conceptVal(cf,
			"PaymentsToAcquirePropertyPlantAndEquipment",
			"CapitalExpenditures",
			"AcquisitionsOfPropertyPlantAndEquipment",
			"PurchaseOfPropertyPlantAndEquipment",
		)), nil)
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
			// Convert raw dollars to millions before storing.
			fcfRaw := *opCF - abs(*capex)
			fcfMVal := fcfRaw / 1e6
			upsert("fcf_reported", &fcfMVal, map[string]any{
				"operating_cf_millions": *opCF / 1e6,
				"capex_millions":        *capex / 1e6,
				"note":                  "fcf = operating_cf - abs(capex), in millions",
			})
		}

		// Balance sheet — all dollar amounts converted to millions.
		upsert("total_assets_reported", divM(conceptVal(bs,
			"Assets",
		)), nil)
		upsert("total_liabilities_reported", divM(conceptVal(bs,
			"Liabilities", "LiabilitiesAndStockholdersEquity",
		)), nil)
		upsert("total_equity_reported", divM(conceptVal(bs,
			"StockholdersEquity", "Equity",
			"StockholdersEquityIncludingPortionAttributableToNoncontrollingInterest",
		)), nil)
		// Long-term debt only; does not include commercial paper or short-term notes.
		upsert("total_debt_reported", divM(conceptVal(bs,
			"LongTermDebt", "LongTermDebtNoncurrent",
			"LongTermDebtAndCapitalLeaseObligations",
			"DebtAndCapitalLeaseObligations",
		)), nil)
		upsert("cash_reported", divM(conceptVal(bs,
			"CashAndCashEquivalentsAtCarryingValue",
			"CashAndCashEquivalents", "Cash",
			"CashCashEquivalentsAndShortTermInvestments",
		)), nil)
		// Shares outstanding — divide by 1e6 to store in millions (matches Finnhub metric scale).
		upsert("shares_outstanding_reported", divM(conceptVal(bs,
			"CommonStockSharesOutstanding",
			"CommonStockSharesIssued",
		)), nil)

		// ── Tier 3: Goodwill & Intangible Assets (rank 18) ───────────────
		// Goodwill arises from acquisitions paying above book value. Heavy
		// goodwill (>40% of total assets) carries impairment risk.
		// XBRL concept names vary by filer; list most common in priority order.
		upsert("goodwill_reported", divM(conceptVal(bs,
			"Goodwill", "GoodwillNet",
			"BusinessAcquisitionCostOfAcquiredEntityPurchasePrice",
		)), nil)
		upsert("intangible_assets_reported", divM(conceptVal(bs,
			"IntangibleAssetsNetExcludingGoodwill",
			"FiniteLivedIntangibleAssetsNet",
			"IntangibleAssetsNet",
			"OtherIntangibleAssetsNet",
		)), nil)

		// ── Tier 3: Inventory (rank 16 — inventory turnover) ─────────────
		// Slowing inventory turnover signals weakening demand before revenue drops.
		upsert("inventory_reported", divM(conceptVal(bs,
			"InventoryNet", "Inventories",
			"FIFOInventoryAmount", "InventoryFinishedGoods",
			"InventoryRawMaterialsAndSupplies",
		)), nil)

		// ── Tier 3: Interest Expense (rank 15 — interest coverage) ───────
		// Interest coverage = EBIT / Interest Expense.
		// Interest expense is typically negative in XBRL; we take abs() in the analyzer.
		upsert("interest_expense_reported", divM(conceptVal(ic,
			"InterestExpense",
			"InterestAndDebtExpense",
			"InterestExpenseDebt",
			"FinanceLeaseInterestExpense",
		)), nil)

		// ── ROIC inputs: pre-tax income, tax expense, current liabilities ─
		// Used by the analyzer to compute NOPAT / Invested Capital.
		// NOPAT = OperatingIncome × (1 − effective_tax_rate)
		// Invested Capital = Total Assets − Current Liabilities
		upsert("pretax_income_reported", divM(conceptVal(ic,
			"IncomeLossFromContinuingOperationsBeforeIncomeTaxesExtraordinaryItemsNoncontrollingInterest",
			"IncomeLossFromContinuingOperationsBeforeIncomeTaxes",
			"IncomeLossBeforeIncomeTaxes",
		)), nil)
		upsert("tax_expense_reported", divM(conceptVal(ic,
			"IncomeTaxExpenseBenefit",
			"IncomeTaxExpense",
			"CurrentIncomeTaxExpenseBenefit",
		)), nil)
		upsert("current_liabilities_reported", divM(conceptVal(bs,
			"LiabilitiesCurrent",
			"CurrentLiabilities",
		)), nil)

		// ── R&D Expense ─────────────────────────────────────────────────────
		// Enables qual_rd_intensity: R&D% of revenue signals innovation investment.
		// Stored in millions, consistent with all other XBRL dollar amounts.
		upsert("rd_expense_reported", divM(conceptVal(ic,
			"ResearchAndDevelopmentExpense",
			"ResearchAndDevelopmentExpenseExcludingAcquiredInProcessCost",
		)), nil)

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

// ─── Analyst Recommendation Trend ────────────────────────────────────────────
//
// Finnhub /stock/recommendation returns a monthly series of analyst consensus
// counts: strongBuy, buy, hold, sell, strongSell.
//
// We compute:
//   net_score_current  = (strongBuy + buy) − (strongSell + sell)  for latest month
//   analyst_rec_trend  = net_score_current − net_score_prior  (month-over-month Δ)
//
// Positive trend = analysts are upgrading (net buys increasing).
// Negative trend = analysts are downgrading (net buys decreasing).
// Both values are stored in equity_fundamentals so the scoring worker can consume them.

func (w *worker) runRecommendations(ctx context.Context) {
	ts := time.Now().UTC()
	for _, sym := range w.cfg.Symbols {
		items, err := w.fh.Recommendation(ctx, sym)
		if err != nil {
			w.log.Warn("finnhub recommendation", "symbol", sym, "err", err)
			continue
		}
		if len(items) < 2 {
			w.log.Debug("recommendation: not enough data", "symbol", sym, "months", len(items))
			continue
		}

		netScore := func(m map[string]any) float64 {
			get := func(key string) float64 {
				if v, ok := m[key].(float64); ok {
					return v
				}
				return 0
			}
			return (get("strongBuy") + get("buy")) - (get("strongSell") + get("sell"))
		}

		current := netScore(items[0])
		prior := netScore(items[1])
		delta := current - prior

		upsert := func(metric string, value *float64, payload any) {
			if err := store.UpsertFundamental(ctx, w.pool, ts, sym, "ttm", metric, value, payload, "finnhub_recommendation"); err != nil {
				w.log.Error("upsert fundamental", "metric", metric, "symbol", sym, "err", err)
			}
		}

		// Current net buy score (absolute level).
		cur := current
		upsert("analyst_rec_net_score", &cur, map[string]any{
			"period":      items[0]["period"],
			"strong_buy":  items[0]["strongBuy"],
			"buy":         items[0]["buy"],
			"hold":        items[0]["hold"],
			"sell":        items[0]["sell"],
			"strong_sell": items[0]["strongSell"],
			"note":        "net_score = (strongBuy+buy) − (strongSell+sell) for latest analyst poll month",
		})

		// Month-over-month change in net buy score (the revision trend signal).
		upsert("analyst_rec_trend", &delta, map[string]any{
			"current_period":      items[0]["period"],
			"prior_period":        items[1]["period"],
			"net_score_current":   current,
			"net_score_prior":     prior,
			"delta":               delta,
			"note":                "positive = analysts upgrading consensus; negative = downgrading",
		})

		w.log.Info("analyst recommendation trend stored",
			"symbol", sym,
			"current_net_score", current,
			"trend_delta", delta,
		)
	}
}

// ─── Qualitative data: insider transactions ───────────────────────────────────
//
// Fetches SEC Form 4 insider buy/sell filings from Finnhub.
// Cluster buying (3+ distinct insiders in 90 days) is a high-conviction signal.
//
// Transaction codes:
//
//	P = open-market purchase  (most bullish)
//	S = sale                  (informational — insiders sell for many reasons)
//	A = award/grant           (compensation, not informational)
//	F = tax withholding       (automatic, ignore)
//	M = option exercise       (often followed by S, not informational on its own)

func (w *worker) runInsiderTransactions(ctx context.Context) {
	for _, sym := range w.cfg.Symbols {
		items, err := w.fh.InsiderTransactions(ctx, sym)
		if err != nil {
			w.log.Warn("finnhub insider-transactions", "symbol", sym, "err", err)
			continue
		}

		stored := 0
		for _, item := range items {
			// transactionDate: "2024-01-15" → parse to time.Time.
			dateStr, _ := item["transactionDate"].(string)
			if dateStr == "" {
				continue
			}
			ts, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				continue
			}

			name, _ := item["name"].(string)
			code, _ := item["transactionCode"].(string)
			if code == "" {
				code, _ = item["acquistionOrDisposition"].(string)
			}

			var shares *float64
			if v, ok := item["change"].(float64); ok {
				abs := v
				if abs < 0 {
					abs = -abs
				}
				shares = &abs
			}

			var price *float64
			if v, ok := item["transactionPrice"].(float64); ok && v > 0 {
				price = &v
			}

			var filingDate *time.Time
			if fd, _ := item["filingDate"].(string); fd != "" {
				if ft, err2 := time.Parse("2006-01-02", fd); err2 == nil {
					filingDate = &ft
				}
			}

			if err := store.UpsertInsiderTransaction(ctx, w.pool, ts, sym, name, code, shares, price, filingDate); err != nil {
				w.log.Error("upsert insider_transaction", "symbol", sym, "err", err)
				continue
			}
			stored++
		}

		w.log.Info("insider transactions stored", "symbol", sym, "count", stored)
	}
}

// ─── Qualitative data: news sentiment ────────────────────────────────────────
//
// Fetches up to 50 recent articles with per-ticker sentiment scores from
// Alpha Vantage NEWS_SENTIMENT. Each article is inserted into news_headlines
// with the numeric sentiment score populated.
//
// Sentiment scale: > 0.35 Bullish, 0.15–0.35 Somewhat-Bullish,
// -0.15–0.15 Neutral, -0.35–-0.15 Somewhat-Bearish, < -0.35 Bearish.

func (w *worker) runNewsSentiment(ctx context.Context) {
	if !w.av.HasKey() {
		w.log.Debug("Alpha Vantage key not set; skipping news sentiment")
		return
	}
	ts := time.Now().UTC()
	_ = ts
	for _, sym := range w.cfg.Symbols {
		articles, err := w.av.NewsSentiment(ctx, sym)
		if err != nil {
			w.log.Warn("alpha vantage news_sentiment", "symbol", sym, "err", err)
			continue
		}

		stored := 0
		for _, art := range articles {
			score := art.OverallSentimentScore
			if art.HasTickerSentiment {
				score = art.TickerSentimentScore
			}
			sentScore := score

			if err := store.InsertNews(ctx, w.pool, art.TimePublished, "alphavantage_sentiment", sym,
				art.Title, art.URL, &sentScore, nil,
			); err != nil {
				w.log.Debug("insert news sentiment", "symbol", sym, "err", err)
				continue
			}
			stored++
		}

		w.log.Info("news sentiment stored", "symbol", sym, "articles", stored)
	}
}

// ─── Qualitative data: institutional ownership ────────────────────────────────
//
// Fetches the top institutional holders from Finnhub and stores their
// aggregate position change as an equity_fundamentals derived metric.
// Positive total change = institutions net accumulating; negative = distributing.

func (w *worker) runInstitutionalOwnership(ctx context.Context) {
	ts := time.Now().UTC()
	for _, sym := range w.cfg.Symbols {
		holders, err := w.fh.InvestorOwnership(ctx, sym, w.cfg.InstitutionalOwnershipLimit)
		if err != nil {
			w.log.Warn("finnhub investor-ownership", "symbol", sym, "err", err)
			continue
		}
		if len(holders) == 0 {
			continue
		}

		var totalShares, totalChange float64
		for _, h := range holders {
			if v, ok := h["share"].(float64); ok {
				totalShares += v
			}
			if v, ok := h["change"].(float64); ok {
				totalChange += v
			}
		}

		holderCount := float64(len(holders))
		upsert := func(metric string, value *float64, payload any) {
			if err := store.UpsertFundamental(ctx, w.pool, ts, sym, "ttm", metric, value, payload, "finnhub_ownership"); err != nil {
				w.log.Error("upsert fundamental", "metric", metric, "symbol", sym, "err", err)
			}
		}

		upsert("institutional_holder_count", &holderCount, map[string]any{
			"top_n":        w.cfg.InstitutionalOwnershipLimit,
			"total_shares": totalShares,
			"total_change": totalChange,
			"note":         "count of top institutional holders in Finnhub investor-ownership response",
		})
		upsert("institutional_net_change", &totalChange, map[string]any{
			"positive": "institutions net buying (accumulating)",
			"negative": "institutions net selling (distributing)",
		})

		w.log.Info("institutional ownership stored", "symbol", sym, "holders", len(holders), "net_change", totalChange)
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

// divM converts a raw XBRL dollar value to millions so it is consistent with
// the Finnhub /stock/metric API, which already returns values in millions.
// Per-share values (EPS, book value per share) should NOT be passed through divM.
func divM(v *float64) *float64 {
	if v == nil {
		return nil
	}
	m := *v / 1e6
	return &m
}

// statementMap returns the inner map for a statement type ("ic", "cf", "bs")
// from the report body. Finnhub may return either an object or an array of
// concept rows; we flatten arrays into a name→value map.
//
// Namespace normalisation: Finnhub returns concept names with an XBRL namespace
// prefix separated by an underscore (e.g. "us-gaap_Assets"). We strip the prefix
// so callers can look up plain concept names like "Assets".
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
	// Array of {concept, label, unit, value} objects — flatten and normalise keys.
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
		// Strip XBRL namespace prefix: "us-gaap_Assets" → "Assets".
		// Finnhub uses underscores; some legacy data may use colons.
		if i := strings.IndexAny(concept, "_:"); i >= 0 {
			concept = concept[i+1:]
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

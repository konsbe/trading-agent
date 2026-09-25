# Momentum Scanner — API Service Spec, Addendum: Full Stock Analysis + Alerts

**Extends:** `docs/MOMENTUM_SCANNER_API.md` / `services/data-analyzer/cmd/momentum-api`.
**Status:** Part A is built (2026-09-25); what shipped, and where it departs
from §2.1–2.3, is in §2.5. Part B: see §3.1.
**Serves:** `mfe-scanner`'s Stock Detail page (extended), and a new
`mfe-alarm-history`.

---

## 0. Two different things, deliberately kept apart

**Part A (§1–2): technical/fundamental/balance-sheet/correlations/sentiment
data.** This reshapes data that's almost certainly already computed and
stored by `technical-analysis`, `fundamental-analysis`, and the
qualitative/correlation workers — the Discord bot already renders it, so
it exists somewhere queryable. **Verify the actual tables in step 1**;
don't assume names from the lexicon doc.

**Part B (§3–4): alerts.** This is not a reshape. **No table persists
fired alerts today** — they exist only as Discord messages, generated
and discarded by the 5-minute scan job. Everything about alerts —
serving them on Stock Detail, badging them on the candidates list,
building Alarm History — depends on a new table that doesn't exist yet.
This is a real prerequisite, not paperwork; treat it with the same
weight the dead macro pipeline got before the Daily Market Report could
be built.

**A third, cross-cutting rule governs both:** the classical TA patterns
(head & shoulders, liquidity sweeps, order blocks, BUY_WATCH/TRIM_WATCH
labels) are **untested against this repo's own data** — not "tested and
found null" like the momentum score, but never evaluated at all. Keep
that distinction in the caveat wording (§2.4); don't reuse
`RESEARCH_SCORE_CAVEAT`'s language verbatim, since it claims a specific
measured result (MH OR 0.991) that doesn't apply here.

---

## 1. Verification — do this before writing any handler

| Question | Why it matters |
|---|---|
| What table(s) hold `technical_indicators` (RSI, MACD, ADX, trend, ATR, BB squeeze, VIX regime, pivots, SMC counts, patterns)? | Confirms the read path; column names must match the lexicon's fields exactly, not be guessed |
| What table(s) hold fundamentals Tier 1/2/3 (EPS, revenue, P/E, FCF, margins, ROE, D/E, EV/EBITDA, current ratio, DCF, etc.)? | Same |
| What table holds qualitative signals (moat proxy, insider activity, news sentiment, R&D intensity)? | Same |
| What table holds correlation clusters and master divergence signals? | Same |
| Does `technical-analysis` write `head_shoulders`/`inv_head_shoulders`/`bb_squeeze`/liquidity-sweep/order-block data anywhere queryable, or only render it inline when generating a Discord embed? | If only rendered inline, this is new persistence work, same class of gap as §3's alerts table — don't assume it's a simple reshape until confirmed |
| Does the `BUY_WATCH`/`TRIM_WATCH` confluence-score logic live in a reusable function, or only inside the Discord command handler? | Determines whether this endpoint can call existing logic or needs it extracted first |

Report back with real answers before proceeding — same discipline as
every prior addendum's step 1.

### 1.1 Verification results (2026-09-25) — the live state wins

**The data was not "already stored".** On verification `technical_indicators`
had 0 rows and `equity_fundamentals` had 0 derived rows: the
`technical-analysis` and `fundamental-analysis` workers had been stopped
since 2026-04-16. Both were rebuilt and restarted on 2026-09-25 and verified
by real rows (50 indicators for all 34 configured symbols; 562 derived
fundamentals rows across 24 symbols with values and tiers).

**Coverage is the watchlist, not the scanner universe.** Both workers run on
`TECHNICAL_EQUITY_SYMBOLS` / `FUNDAMENTAL_SYMBOLS` (26 symbols). A scanner
candidate outside that list has no stored analysis, so Part A would return
nulls for most candidates until coverage is widened.

**Real names** (the lexicon-style names above are not the stored ones):

| Area | Table | Stored as |
|---|---|---|
| Technical | `technical_indicators` (`indicator`, `value`, `payload`) | `rsi_14`, `macd_12_26_9`, `adx_14`, `atr_14`, `trend` (payload `direction`), `ma_ribbon` (`golden_cross`/`death_cross`), `bb_squeeze`, `vix_regime` (payload `regime`), `pivots_prior_bar`/`pivots_weekly`/`pivots_monthly`, `fvg_min0.1_lb50`, `order_blocks_sw3_imp1.5`, `liquidity_sweep_sw3`, `hs_pattern_sw5` (not `head_shoulders`), `flag_pole5_len10` |
| Fundamentals tiers | `equity_fundamentals`, `period='derived'`, `source='fundamental_analysis'` | `composite_score`, `eps_strength`, `revenue_strength`, `pe_vs_5y_mean`, `fcf_yield`/`fcf_yield_tier`, `gross_margin_tier`, `net_margin_tier`, `t2_*` (balance-sheet composite `t2_health_score`, `t2_roe`, `t2_current_ratio`, `t2_leverage`, `t2_ev_ebitda`, …), `t3_*` (`t3_dcf`, …). No `fa_` prefix. |
| Qualitative | same table | `qual_moat_proxy`, `qual_insider_signal`, `qual_news_sentiment_7d`/`_30d`, `qual_rd_intensity` |
| Correlations | same table | `corr_earnings_quality`, `corr_valuation_quality`, `corr_leverage_liquidity`, `corr_operational`, `corr_master_signals`, `corr_summary`. There is no `aligned_signals`; the nearest stored text is each cluster's `positives` list. |

**Bands:** trend direction, MA crosses, BB squeeze, VIX regime, pattern
booleans and every fundamentals tier are stored. RSI and ADX bands are not —
they exist only in the bot, so §2.2's "port the classifier first" applies to
them.

**Pattern and confluence logic:** detection (H&S, sweeps, order blocks,
flags) is pure Go in `internal/compute`, persisted by the worker.
`BUY_WATCH`/`TRIM_WATCH` and confluence are pure Python functions in
`services/analyst-bot/actions/rules/` (not inline in a command handler), so
the Go API cannot call them directly. H&S and flags feed no action rule.

**Alerts (Part B):** no table persisted fired alerts; the only state was a
Redis cooldown flag, set at detection time before the post was attempted.
`action_signal` and `atr_pct_elevated` are not alert kinds the scan emits;
the real kinds are `rsi_oversold`, `rsi_overbought`, `bb_squeeze`,
`vix_elevated`, `fa_tier_flip`, `liquidity_sweep`. Resolved in §3.1.

---

## 2. Part A — `GET /api/v1/scanner/today/{symbol}/analysis`

Extends the existing detail endpoint's data, not a new symbol lookup.
Same 404 behavior as `/today/{symbol}` for an unknown/no-data symbol.
*(Superseded 2026-09-26: any symbol with daily bars is served — §2.5.)*

### 2.1 Response shape (high level)

```json
{
  "symbol": "TSM",
  "as_of": "2026-04-08",
  "technical": {
    "rsi_14": { "value": 50.1, "band": "normal" },
    "macd": { "hist": 0.588, "cross": null },
    "adx_14": 25.7,
    "trend": { "direction": "sideways", "slope_pct": 0.04 },
    "ma_cross": null,
    "atr_14": 12.735,
    "bb_squeeze": { "active": true },
    "vix_regime": { "value": 24.2, "band": "elevated" },
    "pivots": { "pp": 335.97, "r1": 345.14, "s1": 329.87 },
    "smc": { "fvgs_active": 2, "obs_active": 7, "liq_sweeps": 4 }
  },
  "fundamentals": {
    "composite": { "score": 0.70, "tier": "strong" },
    "eps_strength": "strong",
    "revenue": "strong",
    "pe_vs_5y": { "band": "expensive", "value": null },
    "fcf_yield": null,
    "gross_margin": { "value": 59.89, "trend": null },
    "net_margin": { "value": 45.10, "trend": null },
    "ttm_pe": 27.3,
    "market_cap": 46940000000000
  },
  "balance_sheet": {
    "composite": { "score": 1.00, "tier": "healthy" },
    "roe": { "value": 35.12, "band": "excellent" },
    "roa": 23.35,
    "current_ratio": { "value": 2.62, "band": "safe" },
    "quick_ratio": 2.42
  },
  "correlations": {
    "composite": { "score": 0.25, "tier": "neutral" },
    "clusters": [
      { "name": "earnings_quality", "score": 0.6, "tier": "healthy" }
    ],
    "aligned_signals": ["Revenue and EPS growing together — genuine organic quality growth"]
  },
  "sentiment": {
    "headlines": [
      { "title": "...", "url": "...", "source": "...", "published_at": "..." }
    ]
  },
  "context_vs_benchmark": {
    "benchmark_symbol": "SPY",
    "market_cycle_composite": "pullback_healthy",
    "market_cycle_tone": "yellow",
    "price_phase": "pullback",
    "drawdown_from_peak_pct": -5.58,
    "correlation_regime": "stagflation_risk",
    "correlation_regime_tone": "red",
    "relative_strength_20d_pp": 2.85
  },
  "heuristic_signals": {
    "caveat": "<HEURISTIC_TA_CAVEAT, from shared/content/momentum_caveats.json>",
    "chart_patterns": [
      { "pattern": "bear_flag", "confirmed": true, "severity": "notice" }
    ],
    "action_signal": {
      "alert_type": "liquidity_sweep",
      "action": "BUY_WATCH",
      "confluence": { "score": 4, "max": 4 },
      "severity": "notice",
      "reasoning": [
        "Low sweep (6 recent): stop-hunt below swing low detected",
        "Closed back above swept level — institutional accumulation pattern",
        "Bullish order block nearby — strong support confluence",
        "Uptrend intact — sweep aligns with trend continuation"
      ]
    }
  }
}
```

### 2.2 Field notes

- **Every band/tier field (`rsi_14.band`, `fundamentals.eps_strength`,
  `balance_sheet.roe.band`, etc.) is read from stored classification,
  never re-derived client-side** — same rule as `macrotone`. If a
  classifier only exists as inline Discord-formatter logic today, port
  it into a shared Go function first (same pattern as `internal/macrotone`
  and `compute.ClassifyVIX`), so web and Discord read one source.
- **`heuristic_signals` is a clearly separate top-level key**, not
  interleaved with `technical`/`fundamentals`. This is deliberate — see
  §2.4 for why the UI must never blend it into the neutral sections.
- Every numeric field with no value is `null`, rendered as `—` per this
  project's convention throughout — no exceptions for this addendum.

### 2.3 Severity, defined once, not per-field guessed

A `severity` field appears on chart patterns, the action signal, and any
individual technical reading worth flagging (RSI overbought/oversold,
VIX elevated, BB squeeze active). Three levels, applied consistently:

| Severity | Meaning | Examples |
|---|---|---|
| `info` | Worth knowing, not urgent | BB squeeze active, neutral correlation cluster |
| `notice` | Worth a second look | RSI overbought/oversold, confirmed chart pattern, liquidity sweep, moderate confluence |
| `warning` | Actively flagged as elevated risk/urgency | VIX elevated/extreme, `atr_pct_elevated`, `fa_tier_flip` to weak, high confluence TRIM_WATCH |

Source the mapping from a table in code (mirroring `macrotone`'s
mapped-or-test-fails pattern), not ad hoc per call site — a test should
fail if a new alert type or pattern is added with no severity assigned,
same discipline as the tone-mapping tests elsewhere in this project.

### 2.4 The heuristic-signals caveat — new, not reused

Add a new key to `shared/content/momentum_caveats.json`:
`HEURISTIC_TA_CAVEAT`. Suggested text, to be finalized against the
actual repo state confirmed in step 1:

> "Classical pattern signals (head & shoulders, liquidity sweeps, order
> blocks, BUY/TRIM labels) are computed from standard technical-analysis
> heuristics. Unlike the momentum score above, these have not been
> tested against this project's own historical data — they are neither
> validated nor refuted. Treat them as descriptive pattern-matching, not
> a tested strategy."

**Superseded 2026-09-25:** the heuristic signals were since tested
(`docs/HEURISTIC_SIGNALS_PREREGISTRATION.md` §7, no hypothesis confirmed), and
the caveat now cites that result. The live text is `heuristic_ta_caveat` in
`shared/content/momentum_caveats.json`; the suggestion above is historical.

This is deliberately worded differently from `RESEARCH_SCORE_CAVEAT`,
which cites a specific measured null result (MH OR 0.991). Don't merge
the two — they're making different claims and conflating them would
misrepresent both.

**UI requirement, not optional:** this caveat renders directly above the
`heuristic_signals` section, every time, at the same visual prominence
as `RESEARCH_SCORE_CAVEAT` above the score breakdown — not a tooltip,
not collapsed by default.


### 2.5 As built (2026-09-25) — the live state wins

Implemented in `services/data-analyzer` (`internal/momentumapi/analysis.go`,
`analysis_jobs.go`, `internal/store/analysis.go`). Where this section and
§2.1–2.3 disagree, this section describes what runs.

#### On-demand computation, one code path

The workers only cover their configured symbols (§1.1), so the endpoint
computes a missing or out-of-date analysis itself, with the workers' own
per-symbol code — never a second implementation:

- technical: `internal/technical/runner.ComputeAndStore` (the
  `technical-analysis` worker loops it; the pure part is
  `internal/technical.Emitter`, pinned by
  `cmd/technical-analysis/testdata/indicators_golden.json`);
- fundamentals: `internal/fundamental.AnalyzeSymbol` (the
  `fundamental-analysis` worker loops it; pinned by
  `cmd/fundamental-analysis/testdata/derived_golden.json`, an integration test
  written before the code was moved).

Both write the same rows the worker would (`technical_indicators`;
`equity_fundamentals` `period='derived'`, `source='fundamental_analysis'`),
with the worker's config read from the same `.env`. The stored rows are the
cache. This makes momentum-api's second write path (after the watchlist). No
Finnhub or other external fetch is made: outside the watchlist there are no
`finnhub_financials_reported` / `finnhub_earnings` rows, so the fields derived
from them stay null.

#### Freshness rule

- **Technical** is current when the newest `technical_indicators.ts` for
  (symbol, `equity`, `1Day`) equals the symbol's latest bar as the worker's
  own loader sees it (`QueryEquityBars`, one bar per session) — the ts a run
  now would write under.
- **Fundamentals** are current when the newest derived row is no older than
  the newest raw fundamental row for the symbol (any other source) **and** at
  most `MOMENTUM_API_FUNDAMENTALS_MAX_AGE` old (default 26h: the worker's 24h
  cadence plus slack; the insider and news-sentiment passes read
  `NOW()`-relative windows, so derived rows age without new raw rows).
- Only the stale part is recomputed. A computation that finishes without
  making a part current (fewer than 2 bars; fewer than
  `FUNDAMENTAL_ANALYSIS_MIN_METRICS` raw metrics) is a success: the response is
  `ready` with that section `no_data` and nulls, and it is not retried until
  the latest bar or the newest raw row changes (or 26h pass).

#### Statuses

| Condition | Response |
|---|---|
| Both parts current, or a finished computation vouches for these inputs | `200`, `"status": "ready"`, full body below |
| Missing/stale and no computation running | `202`, starts one; body below, `Retry-After` header (seconds) |
| A computation for the symbol is running | `202`, same body; concurrent requests share the one computation |
| The last computation for these inputs failed (error or timeout), within `MOMENTUM_API_ANALYSIS_FAILED_RETRY_AFTER` (1m) | `500`, `{"symbol", "status": "failed", "error": "analysis_failed", "message", "retry_after_ms"}` + `Retry-After`; the next request after that starts a new attempt |
| No scan ever stored / DB down | `503 no_scan_available` / `503 database_unavailable` — same as `/today/{symbol}` |
| The symbol has no daily bars (`equity_ohlcv`, interval `1Day`) | `404 no_data_for_symbol` |

```json
{"symbol": "WRBY", "status": "computing",
 "message": "Computing analysis for this symbol -- first view only",
 "retry_after_ms": 3000, "scanner_data": true}
```

#### Which symbols are served (widened 2026-09-26)

**Any symbol with daily bars**, scanned or not — the analysis is computed from
bars and fundamentals, not from the scanner's row. So TSM, SHEL, SPY, QQQ and
other watchlist ETFs / foreign listings the scanner never covers go
`computing` → `ready` like any candidate. Only a symbol with no `1Day` bars in
`equity_ohlcv` is `404`. `GET /today/{symbol}` is unchanged: it still `404`s
without a `momentum_features` row.

Every analysis body (`200` ready, `202` computing, `500` failed) carries
`"scanner_data": true | false` — whether momentum-scanner has **ever** written
a `momentum_features` row for the symbol (any date, gate-passed or not). The UI
reads it to show "No scanner data for this symbol" (and to skip the
`/today/{symbol}` call) instead of inferring it from a 404.

Limits: `MOMENTUM_API_ANALYSIS_CONCURRENCY` (2) computations at once, each
bounded by `MOMENTUM_API_ANALYSIS_TIMEOUT` (2m, queueing included), single
flight per symbol. `MOMENTUM_API_ANALYSIS_RETRY_AFTER` (3s) is the suggested
poll. Measured on the live DB: a scanner candidate outside the watchlist
(WRBY) computed both parts in 2.9s. Only a fully current `ready` is cached (the
5-minute response cache).

#### Ready body (additions to §2.1)

`status`, `scanner_data` (above), `as_of` (session of the technical rows), `fundamentals_computed_at`,
`sections: {technical, fundamentals}` each `ready` / `stale` / `no_data`, and a
`qualitative` section. Every band/tier-bearing reading is an object
(`{value, band}` or `{score, tier}`), including `adx_14`, `fcf_yield`, `roa` and
`quick_ratio`, which §2.1 showed as bare numbers. Arrays are `[]`, never null.

#### Field mapping (doc field → stored source)

| Field | Source |
|---|---|
| `technical.rsi_14` | `rsi_14` value; `band` = `compute.ClassifyRSI` (bot: `config.py` `bot_rsi_oversold/overbought` 30/70, strict `<`/`>`); `severity` from §2.3 when oversold/overbought |
| `technical.macd` | `macd_12_26_9` value (histogram); `cross` = `bullish`/`bearish` from `bullish_cross_line_signal` / `bearish_cross_line_signal`, else null |
| `technical.adx_14` | `adx_14` value; `band` = `compute.ClassifyADX`: `strong_trend` > 25 else `not_strong_trend` (the bot's only ADX threshold, `actions/rules/bb_squeeze.py`) |
| `technical.trend` | `trend` payload `direction`, `slope_pct` |
| `technical.ma_cross` | `ma_ribbon` payload `golden_cross` / `death_cross` → `"golden_cross"` / `"death_cross"` / null |
| `technical.atr_14` | `atr_14` value |
| `technical.bb_squeeze` | `bb_squeeze` payload `squeeze`; `severity` `info` when active |
| `technical.vix_regime` | `vix_regime` value and stored payload `regime`; `severity` `warning` when VIX > 25 (the bot's `vix_elevated` alert, `bot_vix_alert_threshold`) |
| `technical.pivots` | `pivots_prior_bar` payload `classic.PP/R1/S1` |
| `technical.smc` | `fvg_min0.1_lb50`, `order_blocks_sw3_imp1.5` `active_count`; `liquidity_sweep_sw3` `total_sweeps` |
| `fundamentals.composite` | `composite_score` value, payload `tier` |
| `fundamentals.eps_strength` / `revenue` | `eps_strength` / `revenue_strength` payload `tier` |
| `fundamentals.pe_vs_5y` | `pe_vs_5y_mean` value (% vs own 5y mean; null when no 5y mean), payload `tier` |
| `fundamentals.fcf_yield` | `fcf_yield` value; `tier` from `fcf_yield_tier` |
| `fundamentals.gross_margin` / `net_margin` | `gross_margin_tier` / `net_margin_tier` payload `*_pct`, `tier`; `trend` = `*_trend_8q` payload `direction` |
| `fundamentals.ttm_pe` | `pe_vs_5y_mean` payload `pe_ratio_ttm` (the P/E the worker scored) |
| `fundamentals.market_cap` | newest raw `market_cap` (finnhub_metric, USD) |
| `balance_sheet.composite` | `t2_health_score` |
| `balance_sheet.roe` / `roa` / `current_ratio` / `quick_ratio` | `t2_roe` / `t2_roa` / `t2_current_ratio` / `t2_quick_ratio` value + `tier` |
| `balance_sheet.debt_to_equity` / `net_debt_ebitda` / `roic` (added) | `t2_leverage` / `t2_net_debt_ebitda` / `t2_roic` |
| `correlations.composite` | `corr_summary` value, `tier` |
| `correlations.clusters[]` | `corr_earnings_quality`, `corr_valuation_quality`, `corr_leverage_liquidity`, `corr_operational`: value, `tier`, `positives`, `warnings` |
| `correlations.aligned_signals` | **not stored** (§1.1): served as every cluster's stored `positives`, in cluster order |
| `correlations.master_signals` (added) | `corr_master_signals` `net_signal`, and the names whose `fired` is true |
| `qualitative.*` (added) | `qual_moat_proxy`, `qual_insider_signal`, `qual_news_sentiment_7d/_30d`, `qual_rd_intensity`: value + `tier` |
| `sentiment.headlines` | `news_headlines` for the symbol, newest 10: `headline`, `url`, `source`, `ts`, `sentiment` |
| `context_vs_benchmark.benchmark_symbol`, `market_cycle_composite`, `market_cycle_tone` | `mc_market_cycle` payload `symbol`, `composite_phase`, stored `tone` |
| `context_vs_benchmark.price_phase`, `drawdown_from_peak_pct` | `mc_price_phase:<symbol>` `price_phase`, `drawdown_pct` — only exists for the market report's instruments, else null |
| `context_vs_benchmark.correlation_regime`, `_tone` | `mc_macro_correlation` payload `regime`, `tone` |
| `context_vs_benchmark.relative_strength_20d_pp` | **always null**: not stored (`rs_vs_spy` is disabled, `TECHNICAL_ENABLE_RS_BENCHMARK=false`, and is a 1-bar ratio anyway) |
| `heuristic_signals.caveat` | `heuristic_ta_caveat` in `shared/content/momentum_caveats.json` |
| `heuristic_signals.chart_patterns[]` | `hs_pattern_sw5` (`head_shoulders` / `inv_head_shoulders`, confirmed = neckline break), `flag_pole5_len10` (`bull_flag` / `bear_flag`, detection is the signal, confirmed), `chart_pattern_hints` (`double_top` / `double_bottom`, candidates, never confirmed), `triangle_sw3` (`ascending/descending/symmetrical_triangle`, confirmed = breakout up/down). Only patterns found are listed |
| `heuristic_signals.action_signal` | `heuristics.EvaluateSweepRule` (the parity-tested Go port of `liquidity_sweep.py`) on the stored `liquidity_sweep_sw3`, `order_blocks_sw3_imp1.5`, `trend` rows and the bot's own VIX regime (newest `VIXCLS`, `>35/>20/<12`); present exactly when the bot's scan would raise `liquidity_sweep` (stored count > 0), else null. `reasoning` = `heuristics.SweepRuleReasons`, the Python reason lines verbatim (emoji included), parity-tested. `vix_regime` (added) is the regime the rule read |

Always null outside the watchlist (no reported financials / earnings rows):
the margin trends, FCF yield when no FCF metric, ROIC, net debt/EBITDA,
D/E, moat proxy, R&D intensity, most correlation
checks (clusters are then `mixed_positive` with empty lists).

#### Severity (§2.3) as built

One table, `internal/severity`: `rsi_*` notice, `bb_squeeze` info,
`vix_elevated` warning, `liquidity_sweep` notice, `fa_tier_flip` weak →
warning / otherwise notice — test-checked against
`services/analyst-bot/reports/alert_severity.py` and against the kinds
`reports/builder.py` emits. Chart patterns: confirmed → notice, unconfirmed →
info. Action signal: the `liquidity_sweep` notice, raised to warning for
`TRIM_WATCH` at 4/4 confluence ("high confluence TRIM_WATCH"). A test fails if
any alert kind or emittable pattern has no severity.

#### Conflicts found (flagged, live state used)

- **§2.1 `vix_regime.band` vs §2.3 "VIX elevated → warning"**: the stored
  band is `elevated` above 20 (`TECHNICAL_VIX_ELEVATED_THRESHOLD`), but the
  bot's `vix_elevated` alert fires above 25. `band` is the stored regime;
  `severity` follows the alert, so VIX 20–25 reads `elevated` with no warning.
- **The action rule's VIX regime is not the stored `vix_regime` row**: the
  bot's engine classifies the newest `VIXCLS` with hardcoded 35/20/12, whatever
  `TECHNICAL_VIX_*` say. `action_signal.vix_regime` shows what it read.
- **TSM, the §2.1 example, got 404** (it has no scanner row, and the endpoint
  kept `/today/{symbol}`'s 404; so did SHEL, QQQ, SPY). **Resolved
  2026-09-26:** the endpoint now serves any symbol with daily bars and flags
  `scanner_data: false` (see "Which symbols are served").
- **§2.1 tone words** (`yellow`, `red`) are not stored; the stored macrotone
  words (`constructive` / `neutral` / `stressed`) are served.
- **Stored fundamentals with implausible units** (seen live, not changed here:
  worker outputs are frozen): e.g. MSFT `t3_dcf` / `t3_ps_ratio` use
  `market_cap_millions` = USD market cap, and `fcf_yield` ≈ 1.5e-6 %.
- `momentum_caveats.json` keys are snake case: the constant is
  `HEURISTIC_TA_CAVEAT`, the key `heuristic_ta_caveat`, like `evidence_caveat`.

---

## 3. Part B — persisting alerts (prerequisite for §4)

### 3.1 New table: `fired_alerts`

```sql
CREATE TABLE fired_alerts (
  id BIGSERIAL PRIMARY KEY,
  symbol TEXT NOT NULL,
  exchange_type TEXT NOT NULL,        -- 'equity' | 'crypto', per the lexicon's Exchange field
  alert_type TEXT NOT NULL,           -- rsi_overbought, bb_squeeze, liquidity_sweep, vix_elevated, fa_tier_flip, action_signal
  interval TEXT NOT NULL,
  value NUMERIC,
  severity TEXT NOT NULL,             -- info | notice | warning, per §2.3's mapping
  message TEXT NOT NULL,              -- the human-readable line, e.g. "RSI 75.1 -- overbought (>70.0)"
  fired_at TIMESTAMPTZ NOT NULL,
  UNIQUE (symbol, alert_type, interval, fired_at)
);
```

- **Written by the bot's existing 5-minute alert scan job**, alongside
  (not instead of) posting to Discord. This is the smallest possible
  change — the classification logic already runs and already produces
  this data; it's currently just thrown away after rendering.
- **Verify the cooldown interaction first.** The bot already suppresses
  repeat alerts for 4 hours (`BOT_ALERT_COOLDOWN_SECS`). Decide whether
  `fired_alerts` records every evaluation or only the ones that actually
  posted (post-cooldown) — recording only what posted is almost
  certainly correct, since a suppressed alert wasn't really "fired" from
  a user's point of view, but confirm this explicitly rather than assume.
- **Resolved (2026-09-25, migration 026):** only confirmed posts are
  recorded. `send_alert` now returns whether the platform confirmed the
  post; the cooldown flag and the `fired_alerts` row are both written at
  that point and nowhere else. A failed send leaves no cooldown and no row,
  so the alert is detected again next scan (previously the cooldown was set
  at detection and a failed send burned 4 hours silently). The table adds a
  `CHECK` on `exchange_type` and `severity` and two read indexes.

### 3.2 `GET /api/v1/alerts`

```
GET /api/v1/alerts?symbol=TSM         -- one symbol's recent alerts
GET /api/v1/alerts?since=2026-04-08   -- feed view for Alarm History
```

Response: array of `fired_alerts` rows, each carrying `severity` (§2.3)
and the plain message text. No recomputation — this endpoint only reads
what the scan job already wrote.

---

## 4. Candidates-page alert badge

Add to the candidates list response (`GET /api/v1/scanner/today`, per
the original addendum): an optional `recent_alert` object per candidate,
populated via a `LEFT JOIN` against `fired_alerts` for that symbol
within a short window (e.g. last 24h) — **`LEFT JOIN`, same reasoning as
every other addendum**: a candidate with no recent alert must not
disappear or error, it just has `recent_alert: null`.

```json
"recent_alert": {
  "alert_type": "liquidity_sweep",
  "severity": "notice",
  "message": "Liquidity sweep detected (4 sweeps)",
  "fired_at": "2026-04-08T18:08:00Z"
}
```

Show only the **most recent** alert per candidate on the list view — the
badge is a "something's worth a look" flag, not a full log. Clicking it
(or the row generally) goes to Stock Detail, where the full
`heuristic_signals`/alert history is visible.

---

## 5. Testing

- Every `technical`/`fundamentals`/`balance_sheet` band/tier field
  matches its source classification exactly — no client-side threshold
  re-derivation, mirroring the `macrotone` test discipline.
- A test fails if any alert type or chart pattern has no `severity`
  mapped (§2.3).
- `fired_alerts` write path: confirm it fires only on posted (not
  cooldown-suppressed) alerts, once §3.1's question is resolved.
- Candidates-list `recent_alert` `LEFT JOIN` never drops a candidate row
  with no matching alert.
- `HEURISTIC_TA_CAVEAT` byte-exact test against the shared file, same
  pattern as `EVIDENCE_CAVEAT`/`RESEARCH_SCORE_CAVEAT`.

---

## 6. Build order

| Step | Deliverable |
|---|---|
| 1 | Verification per §1 — real table names, whether TA-pattern/confluence logic is already a reusable function or only inline in the Discord handler. Report back. |
| 2 | `GET /api/v1/scanner/today/{symbol}/analysis` (Part A) — depends only on step 1's confirmed tables |
| 3 | `fired_alerts` table + bot write-path change (Part B, §3) — separate, larger scope; confirm cooldown behavior first |
| 4 | `GET /api/v1/alerts` |
| 5 | Candidates-list `recent_alert` join (§4) |
| 6 | Tests per §5 |
| 7 | Point `mfe-scanner`'s Stock Detail (extended) and new `mfe-alarm-history` at these endpoints |

Parts A and B can proceed in parallel once step 1 reports back — they
touch different tables and different services (A: mostly read from
existing worker output; B: a bot write-path change). Don't block A on B
finishing.
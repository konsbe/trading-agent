# Momentum Scanner — API Service Spec, Addendum: Full Stock Analysis + Alerts

**Extends:** `docs/MOMENTUM_SCANNER_API.md` / `services/data-analyzer/cmd/momentum-api`.
**Status:** specification. Nothing described here is built yet.
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
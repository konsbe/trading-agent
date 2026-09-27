# Education Section — Content Architecture

**Target repo:** `trading-agent`
**Status:** content plan. Nothing described here is written or built yet.
**New nav section:** `EDUCATION`, with three routes: **Handbook**,
**MasterClass**, **Glossary** (a third route, suggested — see §0).

---

## 0. Why three routes, not two

Handbook and MasterClass answer two genuinely different questions, and
mixing them would blur a distinction this whole project has been careful
about everywhere else:

- **Handbook: "What does THIS APP mean when it shows me this?"** Platform-
  specific. Every term is tied to a real, built field — `rvol_20`,
  `breakout_state`, `score_status`, an exit reason, a macro tone. Every
  entry that touches an unvalidated result (the momentum score, the
  heuristic TA signals) **must reuse the exact shared caveat text**
  (`EVIDENCE_CAVEAT`, `RESEARCH_SCORE_CAVEAT`, `HEURISTIC_TA_CAVEAT`) —
  never a fresh paraphrase, for the same reason those caveats are stored
  once and read everywhere else.
- **MasterClass: "What does the stock market itself mean by this?"**
  General, standards-based, true regardless of which app someone uses.
  RSI, P/E, market cap, order types — these existed before this project
  and are not this project's claims. **MasterClass must never mention
  `momentum_score_100`, `BUY_WATCH`/`TRIM_WATCH`, or any of this app's
  own unvalidated outputs.** Someone reading MasterClass should come away
  understanding RSI in general; someone reading the Handbook's RSI entry
  should come away understanding what *this app* does with it. Keep the
  boundary hard — a MasterClass page that starts explaining this app's
  own confluence rule has quietly become a Handbook page.
- **Glossary: "I just need this one term, right now."** Both of the above
  are narrative, read start to end. A person three sections deep into
  Stock Detail who hits "ADX 14" doesn't want to leave and read a
  chapter — they want a fast answer with a link to go deeper if they
  want to. This is the same "plain value vs. full breakdown" split
  already used throughout the app (a candidate row's plain RVOL number
  vs. Stock Detail's full technical section).

---

## 1. Handbook — outline, section by section, matching the app's own order

Mirror the sidebar's actual order (Market Reports → Momentum Scanner →
Research → Tracking → Admin), since that's the order a user actually
encounters these concepts in. Each Handbook section should open with one
short paragraph on **"what this part of the app is for"** before
listing its terms — the orientation matters as much as the definitions.

### 1.1 Daily Market Report
- VIX, 10Y/5Y/2Y yield, EUR/USD — what each number is, in plain terms
- The five stance cards (Monetary Policy, Growth Cycle, Inflation,
  Global/Geopolitical Stress, Macro Correlations Regime) — what each
  measures, why it's colored the way it is (reuse `macrotone`'s actual
  mapping, don't re-invent the color meaning here)
- Market Cycle composite vs. the SPY Index sub-section — the two
  different things bundled in that card, explained separately (this
  project's own confusion about this, resolved a few sessions ago, is
  worth writing up plainly: "cycle" is a blended regime score, "index"
  is one instrument's own price facts)
- Tracked instruments (fixed seven + watchlist), BTC's different windows
- Seasonality/presidential cycle — explicitly labeled "reference tilt,
  not predictive" in the Handbook too, matching the app's own framing
- Automation status list — what "not_automated"/"needs_data"/"partial"
  actually mean, so it doesn't read as an error

### 1.2 Today's Candidates (Momentum Scanner)
- What a "gate" is and why failing one excludes a symbol entirely
  (not a low score — an exclusion)
- Each of the six §3.2 gates, in plain language: price band, history
  minimum, day-change band, RVOL minimum, dollar-volume minimum, market
  cap band
- RVOL, dollar volume, RSI(14), breakout state, 52-week-high % — what
  each column literally measures
- The score: `momentum_score_100`, why it's shown de-emphasized with
  "unvalidated," what `score_attainable` means (the "/75" ceiling)
- **This section's caveat block must quote `EVIDENCE_CAVEAT` verbatim**

### 1.3 Stock Detail — full analysis
- Technical analysis grid: RSI, MACD, ADX, Trend, MA Cross, ATR, BB
  Squeeze, VIX Regime, Pivots, SMC counts — what each one is
- Fundamentals grid: EPS Strength, Revenue, P/E vs 5Y, FCF Yield,
  margins, PEG, TTM P/E, Market Cap
- Balance Sheet: ROE, ROA, Current/Quick Ratio, D/E, Net Debt/EBITDA
- Correlations: cluster health, master signals, what "aligned" vs.
  "divergent" signals mean
- Qualitative Signals: moat proxy, insider activity, news sentiment,
  R&D intensity — and the honest "insufficient data" cases
- Context vs Benchmark: what comparing to SPY tells you
- **Classical Technical Signals section**: chart patterns, liquidity
  sweeps, order blocks, the BUY_WATCH/TRIM_WATCH confluence signal.
  **This section's Handbook entry must state the Thread 2 validation
  result plainly**: which signal (RSI-oversold) held in-sample but
  failed the lockbox, and that the rest showed no measurable effect.
  Quote `HEURISTIC_TA_CAVEAT` verbatim. This is the single most
  important entry in the whole Handbook to get exactly right — it's
  where a user decides how much weight to put on what they're looking
  at.
- Severity badges (info/notice/warning) — what triggers each

### 1.4 Backtest Lab
- Why this page shows a dated, closed report rather than "live" numbers
- What "the rule was committed before the result" means and why it
  matters (this is worth its own short, standalone explanation — it's
  the single idea that makes the whole report trustworthy)
- Reading an odds ratio and a confidence interval in plain terms (this
  bridges into MasterClass §2.6 — link there for the general statistics
  explanation, keep this entry short and specific to what the report
  shows)

### 1.5 Watchlist
- What adding a symbol does (and doesn't do — no alerts unless it's
  also a candidate or has a fired alert)
- Why watchlist prices can show "as of [older date]" — staleness
  explained in the same terms as the candidates page

### 1.6 Tracked Positions
- **This is research instrumentation, not a portfolio** — say this as
  plainly as the app itself does. No P&L language.
- What "active"/"closed" mean, what triggers each of the five exit
  reasons, and — critically — the honest finding that `breakout_failed`
  fires almost immediately and isn't a real signal of failure. Reuse
  each `exit_reason_note` verbatim.
- The freshness banner: difference between "no fresh scan" and "scan
  fresh, tracker behind"

### 1.7 Data Source
- Why this is the one page in the app allowed to use live/status
  language and color (explain the "operational health, not a trading
  signal" distinction directly — it's a fair question for a user to ask
  after seeing every other page avoid this)
- Provider budgets, the daily chain states (clean/paused/failed/not_run)

### 1.8 Alarm History (once built)
- What triggers each alert type, severities, how the candidates-page
  badge relates to this page

---

## 2. MasterClass — curriculum outline

**Format rule, stated once here, applied to every entry:** a **3-line
maximum "as simple as it gets" summary** at the top of every entry,
followed by a **full detailed explanation below it** (collapsible or on
scroll — the summary must never require opening anything to read).
Worked example:

> **RSI (Relative Strength Index)**
> *Three-line version:* RSI measures whether a stock has been bought or
> sold too fast recently. Above 70 usually means "bought a lot,
> lately." Below 30 usually means "sold a lot, lately."
>
> *Full explanation:* RSI is calculated from the size of a stock's
> recent up-moves versus its recent down-moves over a chosen period
> (commonly 14 days)... [continues: the actual formula, why 70/30 are
> conventional not universal, what "overbought" does and doesn't
> predict, how professionals actually use it alongside other tools,
> common misreadings]

**No platform-specific content anywhere in this route.** No mention of
`momentum_score_100`, gates, BUY_WATCH, or this app's own confluence
rule. If an entry starts explaining what *this app* does, it belongs in
the Handbook, not here.

### Module 1 — What a stock market actually is
- What a share represents (ownership, not a lottery ticket)
- Exchanges, tickers, order books — how a trade actually happens
- Market cap, float, shares outstanding — what they mean and how they
  differ
- Market hours, pre-market/after-hours, why liquidity differs by time

### Module 2 — Reading a price chart
- Candlesticks: open/high/low/close, what a green vs. red candle means
- Volume: what it is, why volume "confirms" a price move
- Support and resistance, trendlines
- Timeframes: why a daily chart and a 5-minute chart tell different
  stories about the same stock

### Module 3 — Technical indicators, standards-based
- Moving averages (SMA/EMA), golden cross/death cross
- RSI, MACD, ADX — what each measures and its actual limitations
  (RSI staying "overbought" for weeks in a strong trend is a common
  real-world misread, worth naming explicitly)
- Bollinger Bands and volatility squeezes
- Support/resistance and classic chart patterns (head & shoulders,
  flags, triangles) — described as *pattern recognition heuristics with
  a long history in the field*, not as proven predictors. This is the
  right place for the standard, industry-wide caveat that technical
  patterns are debated in academic finance — general, not tied to this
  app's own null result (that's the Handbook's job).

### Module 4 — Fundamental analysis
- Revenue, earnings, EPS — what a company "making money" actually means
  on a financial statement
- P/E ratio, PEG, P/S, P/B — what each valuation multiple is comparing
- Margins (gross/net) — what they say about a business's economics
- Balance sheet basics: assets, liabilities, equity, debt ratios,
  liquidity ratios
- Cash flow vs. earnings — why they can differ and why that matters
  (this is a good, general on-ramp to what the app's own FCF-conversion
  field is pointing at, without naming the app)

### Module 5 — Market structure and mechanics
- Order types: market, limit, stop, stop-limit
- Bid/ask spread, liquidity, slippage
- Short selling, margin — plainly and neutrally
- Market makers, why prices move on low-volume names more easily

### Module 6 — Risk and position sizing
- Diversification, correlation between holdings
- Volatility (ATR, standard deviation) as a measure of risk, not
  opportunity
- Stop-losses: what they are and the tradeoffs of using one
- Position sizing basics — general risk management principles, not
  specific to any strategy

### Module 7 — Reading market-wide conditions
- Interest rates and equities — why rate moves affect stock valuations
  in general (this is the natural on-ramp into the Handbook's Daily
  Market Report entries, without repeating this app's specific stance
  labels)
- Inflation, GDP, employment data — what they are and why markets react
- VIX — what it measures in general terms (distinct from the Handbook's
  entry on how *this app* bands VIX into "normal/elevated/etc.")
- Market cycles / bull and bear markets — general definitions

### Module 8 — How professionals actually evaluate a stock
- Combining technical + fundamental view, why neither alone is
  considered sufficient by serious practitioners
- What "confluence" means in general trading practice (several
  independent signals agreeing) — a good, neutral place to explain the
  *concept*, distinct from the Handbook's entry on this app's own,
  specifically-tested confluence rule and its result
- Why past performance and backtests have real, well-known limitations
  (survivorship bias, overfitting, regime change) — described as
  general, standard cautions every serious analyst is taught, which
  happens to be exactly why this project built its own protocol the way
  it did (a nice, honest bridge to the Handbook/Backtest Lab entry,
  without this MasterClass page needing to reference this app at all)

---

## 3. Glossary — mechanics

A flat, alphabetized, searchable table. Each entry: term, one-line
definition, and a link to its fuller Handbook or MasterClass entry
(never both duplicated — Glossary points, it doesn't re-explain).

Populate it by pulling every bolded/defined term out of both Handbook
and MasterClass once they're written — don't hand-author it separately,
or the two will drift (same "one source of truth" discipline as every
caveat in this project).

Search should match on the term and on common synonyms a user might
actually type ("PE ratio" as well as "P/E", "moving average" as well as
"MA").

---

## 4. How this fits the existing architecture

**Content, not computation — same pattern as Backtest Lab.** Handbook
and MasterClass are static, authored content (like
`backtest_lab_report.json`), not derived from live data. A new,
analogous content file makes sense: e.g.
`shared/content/handbook.json` / `shared/content/masterclass.json`,
served by a small read-only endpoint on `momentum-api`, cached long
(24h+), same reasoning as Backtest Lab's caching decision.

**Cross-reference discipline:** any Handbook entry describing an
unvalidated result must pull its caveat text from
`shared/content/momentum_caveats.json` at render time (or reference it
by key in the content file), never re-type it. This is the same
byte-exact enforcement already used for `EVIDENCE_CAVEAT` /
`RESEARCH_SCORE_CAVEAT` / `HEURISTIC_TA_CAVEAT` — a Handbook page is
exactly the kind of place a caveat could quietly drift if it's
hand-copied instead of referenced.

**Nav placement:** new `EDUCATION` group in the sidebar, below
`TRACKING` and above `ADMIN` (education is for the end user, admin is
operational — keep them visually separated, matching the existing
grouping logic).

---

## 5. Build order

| Step | Deliverable |
|---|---|
| 1 | Confirm content-serving pattern (new JSON files + endpoint vs. reusing an existing one) and nav placement |
| 2 | Author Handbook content, section by section per §1 — this is real writing work, budget for it as such |
| 3 | Author MasterClass content, module by module per §2 |
| 4 | Build Glossary by extraction from both, once both exist |
| 5 | UI: three new routes, nav group, Handbook/MasterClass as narrative scrollable pages with a section jump-nav, Glossary as a search-first flat list |
| 6 | Tests: caveat cross-reference byte-exact checks (§4); a test asserting MasterClass content contains none of this app's own field names or output labels (`momentum_score_100`, `BUY_WATCH`, `TRIM_WATCH`, `breakout_from_consolidation`, etc.) — enforces the hard boundary in §0 the same way other boundaries in this project are enforced by test, not by convention alone |

Step 6's MasterClass-purity test is worth building even though it sounds
unusual — it's the same instinct as the guard test that fails if
Backtest Lab's response ever contains a live score or symbol-specific
field. A boundary this project cares about should be enforced somewhere
a test can catch a violation, not left to whoever writes content later
remembering the rule.


#   Handbook — Full Term Inventory

**Status:** coverage audit. Only §1.3's "Classical Technical Signals" entry
exists today. Everything else below is outlined in
`EDUCATION_SECTION_CONTENT_SPEC.md` §1 but **not yet written**. This is the
complete checklist — every field, header, and label currently rendered on
the three surfaces named, pulled from the actual built API responses and
UI briefs, not reconstructed from memory.

---

## A. Daily Market Report

### A1. Top strip
- VIX (raw value + regime band: normal/elevated/extreme_fear/complacency)
- 10Y Yield
- EUR/USD

### A2. Five stance cards (each: composite score, tone/label, expandable signals)
**Monetary Policy** — tier 1: Policy Rate, Yield Curve (2s10s / 3m10y),
Real Rate (TIPS 10Y), Balance Sheet (Fed, QE/QT), Credit Spreads (HY/IG OAS).
tier 2: Breakeven Inflation, Treasury Yields (2Y/10Y/30Y), M2 Money Supply.

**Growth Cycle** — tier 1: ISM PMI, LEI, Initial Claims, Housing
Starts/Permits. tier 2: Real GDP, Payrolls + Sahm Rule, Real Retail Sales.
tier 3: Michigan Sentiment, Core Capex.

**Inflation & Prices** — tier 1: Core PCE, CPI + Core CPI, Shelter CPI.
tier 2: PPI Final Demand + PPI-CPI spread, WTI/Brent Oil. tier 3: Wages
(AHE/ECI), Copper.

**Global/Geopolitical Stress** — tier 1: Broad USD (DTWEXBGS — note: not
ICE DXY), USD/JPY (carry unwind risk). tier 2: China GDP YoY, US fiscal
deficit % of GDP.

**Macro Correlations Regime** — the regime label itself (e.g.
`stagflation_risk`, `global_liquidity_stress`), its score, and its flags
list.

### A3. Market Cycle (market-wide) — its own card
- Composite phase + score (the 12-phase enum, tone-mapped)
- Blended inputs (Growth/Policy/Inflation/Global, each with its own tone)
- Index sub-section: Symbol (benchmark), Price Phase, Drawdown from Peak,
  vs 200DMA, SMA200, Close, Crash Velocity Flag
- **Needs its own explanation of why this is two different things bundled
  in one card** — the composite is a blended regime score; the Index
  sub-section is one instrument's own price facts. This distinction was a
  real design correction mid-project and deserves to be taught, not just
  displayed correctly.

### A4. Instruments (Tracked + Watchlist groups)
- Price, change %, session_closed flag
- Market-cycle phase, drawdown from peak, vs 200DMA (equities/ETFs only)
- Yield cards: yield %, as-of date (no price-phase fields — explain why
  yields don't get one)
- BTC's distinct treatment: "00:00 UTC daily close (closed candles only)",
  365/14/7/200-day windows instead of the 252/10/5-session equity ones

### A5. Seasonality & cycle context
- Month seasonality (the almanac label, e.g. "weak_bear")
- Presidential cycle (year 1-4, midterm framing)
- Intermarket correlations: bond-equity ρ, oil-equity ρ, VIX-equity ρ, and
  their regime labels (deflationary_hedge, decoupled, typical_fear_greed,
  etc.)

### A6. Calendar & news
- Economic calendar (and what a 403/unavailable state means)
- Earnings calendar, and the three-way `earnings_coverage` states
  (upcoming / none_in_window / not_ingested)
- News headlines format

### A7. Data coverage note
- What `not_automated` / `needs_data` / `partial` / `live_static` /
  `partial_live` actually mean — this list looks like a feature gap but is
  actually a transparency disclosure; the Handbook entry should say that
  explicitly.

---

## B. Today's Candidates

### B1. The gates (all six, §3.2)
- Price band (market ≥$2.00, penny $0.30-$2.00)
- History minimum (≥252 bars)
- Day-change band (market +8-25%, penny +10-40% — **and why the upper
  bound exists**, not just the lower)
- RVOL minimum (market ≥3.0×, penny ≥4.0×)
- Dollar volume minimum (market ≥$5M, penny ≥$2M)
- Market cap band (market $300M-$10B, penny ≤$300M)
- What "failed a gate" means (excluded, not scored low) vs. "passed" +
  what a partial pass looks like on the detail page

### B2. List columns
- Bucket (market/penny) — what determines it (price, not size)
- RVOL (rvol_20) — plain definition, why it's the default sort
- Dollar Volume
- RSI(14) — reused from MasterClass Module 3, but this entry should
  explain the platform-specific fact that RSI is a *penalty*, not a
  positive scoring input here (Phase 1 §3.10) — a genuinely
  counter-intuitive design choice worth explaining
- Breakout State (none/approaching/breakout/breakout_from_consolidation)
  — **and the honest finding**: this field measured *inverted* for
  predicting big moves (Phase 1 §10.1.0), which is exactly why it's
  zeroed in the score. This is a must-include, high-value fact.
- 52-Week High % — same treatment: also measured inverted, also zeroed
- Catalyst Tier (A/B/none/null) — why null is the common case
- Market Cap, Market Cap (est.), the `is_proxy` flag — what "estimated"
  means and why it's disclosed rather than presented as reported
- Score (`momentum_score_100`), Score Attainable (the "/75" ceiling, why
  it's not "/100"), `score_status: unvalidated`
- Recent Alert badge — what triggers it, links to Alarm History once
  that entry exists

### B3. Page-level facts
- Universe scanned vs. universe eligible, and why they can differ
- Scan date, `is_stale`, and the stale-scan messaging
- Why sorting by score doesn't imply ranking (the `score_status` marker
  travels with every sort order)

---

## C. Stock Detail (candidate/symbol detail page)

### C1. Top block
- "Passed the gates" panel — same six gates as B1, shown per-symbol with
  actual value vs. band
- Gate failures list (for a symbol that didn't pass)
- "No scanner data for this symbol" state (never-scanned symbols)
- `as_of`, `is_stale`, `is_candidate_today` — what each means for a
  non-candidate or stale symbol

### C2. Primary Facts grid
Last Close, Day Change, Volume, Avg Volume (20-day), Dollar Volume, RVOL
(20-day), Float Shares (est.) + `float_is_proxy`, RSI(14), 52-Week High +
% from peak, Breakout State, Gap %, VWAP Distance, ATR%, Market Cap,
Identified Catalyst.

### C3. Price chart
- Range options (1D/5D/1M/6M/1Y/ALL) and what changes between them
  (intraday 5-min bars vs. daily, adjusted vs. unadjusted)
- The `fallback: no_intraday_data` case — why a symbol sometimes shows
  daily bars on a "1D" request

### C4. Score Breakdown
- Total, Attainable (/75), Allocated (/90) — the three-number relationship
- Every sub-score (rvol, vol_accel, catalyst, float, vwap, breakout,
  high52w) with its weight — **and explicitly which are zeroed and why**
  (breakout and high52w measured inverted)
- Missing inputs list
- Penalties (the three: exhausted momentum, already extended, volume
  decaying) — what each means and why it exists
- The research caveat, verbatim, with the actual measured odds ratio
  (0.991) explained in plain terms

### C5. Technical Analysis section
RSI(14), MACD (histogram + cross), ADX(14) (trend strength, not
direction), Trend + slope, MA Cross (golden/death), ATR(14), BB Squeeze,
VIX Regime (and how this differs from the market-wide VIX band — the
known 20 vs 25 threshold mismatch is worth a footnote once it's fixed),
Pivots (PP/R1/S1), SMC counts (FVGs, Order Blocks, Liquidity Sweeps —
**cross-reference to the Classical Technical Signals entry**, since these
counts feed that section).

### C6. Fundamentals section
Composite score + tier, EPS Strength, Revenue, P/E vs 5Y, FCF Yield,
Gross Margin, Net Margin, PEG, Earnings Surprise, TTM P/E, Market Cap.

### C7. Balance Sheet section
Composite + tier, ROE, ROA, ROIC, Current Ratio, Quick Ratio, Debt/Equity,
Net Debt/EBITDA, EV/EBITDA, Price/Book, Dividend Yield, CapEx Intensity.

### C8. Correlations section
Composite + tier, the four cluster names (Earnings Quality, Valuation vs
Quality, Leverage & Liquidity, Operational) and what each measures, the
five master divergence signals (Bullish Convergence, Hidden Value,
Deterioration Warning, Value Trap, Leverage Cycle Warning) and what each
means, Net Signal, Aligned/Divergent signal lines.

### C9. Qualitative Signals section
Moat Proxy, Insider Activity (+ the honest `insufficient_data` case and
why), News Sentiment (7D/30D), R&D Intensity.

### C10. Sentiment & News
Headline format (title/source/link only — and why no article text, per
the copyright practice already applied everywhere).

### C11. Context vs Benchmark
Benchmark symbol (SPY), Market Cycle (market-wide, cross-reference A3),
Price Phase, Drawdown from Peak, Macro Correlations Regime, Relative
Strength (20D).

### C12. Classical Technical Signals — *already written*
Chart patterns (head & shoulders, inverse H&S, flags, triangles, double
tops/bottoms — cross-reference MasterClass Module 3 for what each *is*;
this entry says what was *tested*), liquidity sweeps, order blocks,
confluence score, BUY_WATCH/TRIM_WATCH, severity badges.

---

## Build order — realistic batches, not one pass

Given the size, split into reviewable batches, same pace as everything
else this session:

| Batch | Covers | Est. entries |
|---|---|---|
| 1 | B1-B3 (Today's Candidates — gates, columns, page facts) | ~10 |
| 2 | C1-C4 (Stock Detail — top block, facts, chart, score breakdown) | ~8 |
| 3 | C5-C7 (Technical, Fundamentals, Balance Sheet grids) | ~25 (many are short, definition-only entries) |
| 4 | C8-C11 (Correlations, Qualitative, Sentiment, Context) | ~15 |
| 5 | A1-A7 (Daily Market Report, full) | ~20 |

Batches 3 and 5 are the largest by entry count but the *shortest* per
entry — most are single-metric definitions (e.g. "ROE," "Current Ratio")
that don't need Handbook's fuller treatment, just a clear one-paragraph
answer to "what is this and what does it tell me." Batches 1, 2, and
part of the Daily Market Report (the Market Cycle split, the gates'
upper-bound rationale) need the fuller, more explanatory treatment this
project has already shown it can do well.
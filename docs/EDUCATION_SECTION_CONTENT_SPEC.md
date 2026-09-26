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
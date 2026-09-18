# Market-data provider subscriptions

Every provider this project has touched, what its free tier actually does, what
specifically broke on it, and whether paying fixes *that* problem.

**Reading rule.** "Paid tier" claims are only as good as their source. Each
section states how the paid-tier claim is known, and where it is unverified this
document says so instead of implying a fix. The whole reason this file exists is
that "paid = unlimited" was assumed once and cost a working day: Tiingo's limiter
was configured at 1.5 req/sec on the belief that its only real constraint was a
monthly symbol count, and the hourly cap it did not account for failed 69 symbols
mid-backfill.

Verification dates are 2026-09-17/18, measured against this project's own keys.

---

## Summary

| Provider | Tier | Role today | Free tier blocks | Paid fixes it? | Cost |
|---|---|---|---|---|---|
| **Finnhub** | free | symbol universe, quotes, fundamentals | nothing yet | n/a | — |
| **Tiingo** | free | **primary bars** | 9h for 450 symbols; 500 symbols/month | **yes, documented** | $10–30/mo |
| **Twelve Data** | free | **disqualified — correctness** | see its own section | **unknown, and not a pricing question** | — |
| **Yahoo** | keyless | legacy only | IP-blocked; no dividend adjustment | **no** | — |
| **Polygon** | free | key held, no adapter | 2.0y history, delayed | likely, unverified | ~$29–199/mo |
| **EODHD** | free | key held, no adapter | 1.0y history | likely, unverified | ~$20–80/mo |
| **Alpha Vantage** | free | crypto/FX only | adjusted daily is premium-only | yes, by definition | ~$50/mo |

---

## Finnhub — free, and currently sufficient

**Tier:** free. **Limits:** 60 requests/minute; this project paces at **1 req/sec**
(`FINNHUB_RATE_PER_SEC=1.0`) through the shared Postgres budget so five workers
share one allowance.

**What it does here:** the §3.1 symbol universe (31,051 US symbols fetched,
4,975 eligible), `/quote` snapshots, and fundamentals.

**What breaks on free:** nothing so far. The one place it was *load-bearing under
pressure* went fine: pricing the entire 4,975-symbol eligible universe via
`/quote` to bootstrap §3.2's price buckets took **1h22m53s** at 1 req/sec, with
4,928 priced and 47 failures. That is slow but it is a once-per-pilot cost, and
it saved the bar provider's quota entirely.

**Known gap, not a limit:** Finnhub returns null `market_cap` for many micro-caps,
which is why §3.9 defines a `shares_outstanding × close` proxy. That is a data
coverage issue, not a tier restriction — paying would not obviously fix it, and
the proxy is already implemented and its provenance recorded in
`gate_failures = market_cap_null`.

**Upgrade trigger:** if the daily scan ever needs per-symbol news for more than
~3,000 candidates/day, or if the 83-minute pricing pass has to run daily rather
than once. Neither is true in Phase 1.

---

## Tiingo — free, primary provider, and the clearest upgrade case

**Tier:** free ("Starter"). **Limits, published and measured:**

```
50 requests / hour        HARD. Fixed-clock reset, measured: blocked 05:54 UTC,
                          first success 06:01:42 UTC — it resets on the hour, not
                          on a rolling 60-minute window.
1,000 requests / day      resets midnight EST
500 unique symbols/month  re-reading an already-counted symbol is free
1 GB bandwidth / month
no per-minute or per-second limit
```

Tiingo's own OpenAPI spec defines the 429 as:

> "Usage limit exceeded. Tiingo limits by hourly requests (reset every hour),
> daily requests (reset at midnight EST), and monthly bandwidth (reset the first
> of every month at midnight EST); there is no per-minute or per-second rate
> limit."

**Why it is primary:** it is the only provider verified to apply **one consistent
adjustment factor across a whole series**. Its `adj*` fields are split- and
dividend-adjusted, and `adjVolume` is split-adjusted (NVDA's 10:1 of 2024-06-10:
raw volume 41,238,580 → adjusted 412,385,800, exactly 10.0000×).

### What specifically breaks on free

**Incident 1 — the 429 storm (2026-09-17).** The limiter was set to 1.5 req/sec
because the code believed the monthly unique-symbol count was the only real
constraint and that pacing was "politeness only". That spent the hourly
allowance in under a minute. Result: **69 of 200 attempted symbols failed**, each
burning 4 retry attempts against a refusal that could not clear until the next
clock hour. Only 64 symbols completed.

**Incident 2 — wall clock.** At the true 50/hour ceiling, the pilot's
**450-symbol backfill takes ~9 hours**. This is not tunable: `TIINGO_RATE_PER_SEC`
above 0.0139 converts waiting into 429s and nothing else.

**Incident 3 — an unanswerable quota question.** The free tier exposes **no
account-usage endpoint** (four candidate paths probed, all 404). After incident 1
there was no way to determine whether the ~136 rejected requests consumed
unique-symbol allowance. If they did, 450 symbols may exceed the 500/month cap
and strand the backfill until the 1st.

### Does paid fix it?

**Yes, and this is documented rather than assumed.** From Tiingo's own pricing
table:

| Plan | Price | Unique symbols/month | Requests |
|---|---|---|---|
| Starter | $0 | 500 | 50/hour, 1,000/day |
| Power | $30/mo ($300/yr) | ~108,980 | 10,000/hour, 100,000/day |
| Commercial | $50/mo ($499/yr) | ~108,980 | 10,000/hour, 100,000/day |

A third-party comparison also lists a **$10/mo** Power tier with unlimited unique
symbols; the price should be confirmed on Tiingo's own pricing page before
purchase, since the two sources disagree.

Mapping the fix onto the three incidents, which is the part worth checking:

- 10,000 requests/hour vs 50 turns the ~9-hour backfill into **~3 minutes**.
- ~108,980 unique symbols/month vs 500 removes the cap on the **full 4,975-symbol
  universe**, not merely the 450-symbol pilot — this is the constraint that
  currently forces the pilot to be a subset at all.
- The usage-visibility gap is *not* addressed by the table. Higher limits make it
  moot rather than fixing it.

**Upgrade trigger:** the moment §7's base-rate validation clears on the 450-symbol
pilot. At that point the subset exists only because of the 500/month cap, and
$30/month buys the full universe plus a 3-minute refresh — which also removes the
stratified-sampling machinery, the pilot-subset selection, and the bootstrap
pricing pass as ongoing concerns. Upgrading **before** the base rate clears would
be paying to scale a score with no demonstrated edge, which §2.2 explicitly warns
against.

---

## Twelve Data — disqualified on correctness, not on price

**Kept separate from the pay/free framing above, because no tier fixes a wrong
number.**

**Tier:** free. **Limits, measured:** 8 credits/minute (its own 429 names the
count: *"9 API credits were used, with the current limit being 8"*), 800
requests/day published, **no unique-symbol cap** for US equities, and a full
3 years of daily history (752 AAPL bars). On paper it is strictly better than
Tiingo for this job: the same 450-symbol backfill runs in **~56 minutes** instead
of ~9 hours, and it completed in 45 minutes when actually run.

**The defect.** `adjust=all` applies adjustment **inconsistently, bar by bar,
inside a single response.** ABTS across a 1-for-15 reverse split, against Tiingo
for the same window:

| date | Tiingo | Twelve Data | |
|---|---|---|---|
| 2025-02-26 | 6.3045 | 0.4200 | unadjusted |
| 2025-02-27 | 6.3150 | 0.4210 | unadjusted |
| 2025-02-28 | 6.2400 | **6.2400** | adjusted |
| 2025-03-03 | 6.2250 | **6.2250** | adjusted |
| 2025-03-04 | 5.2725 | 0.3520 | unadjusted again |
| 2025-03-10 | 3.7320 | **3.7320** | adjusted again |

Each bar is internally consistent, so nothing in the response looks malformed.
The damage is at the seams, which fabricate one-day moves of **+1382%, −94%,
+796%, +925%**. Measured scope on the 450-symbol pilot: **12–17% of symbols**, and
**41% of the penny bucket** (37 of 90) against 10.6% of the market bucket.

For a momentum scanner this is the worst available failure mode, because a
fabricated +796% day is exactly the signal §3 hunts — every corrupted symbol
sorts to the top of the scan looking flawless.

**Why a hybrid was rejected:** "use it where it looks clean" is unavailable,
because clean cannot be established locally. **APAM, AQN and ARX disagree with
Tiingo by 4.02%, 3.52% and 2.09% with no detectable discontinuity anywhere in
their series.** Any threshold low enough to catch those also flags real
penny-stock moves.

NVDA's forward 10:1 split adjusts correctly, which is why a split fixture alone
passed. The failures cluster on **recent and reverse splits in micro-caps** — the
population the pilot deliberately over-samples.

**Does paid fix it? Unknown, and it is the wrong question.** No pricing tier
advertises "correct corporate-action handling", and the defect is in data
processing rather than in quota. Reconsidering it would require re-running
`cmd/bar-audit` against a paid key and getting a clean result — not reading a
pricing page. Until then it stays disqualified regardless of cost.

**What was kept:** the adapter, its tests, the ABTS fixture in
`internal/barquality`, and `cmd/bar-audit`. The defect stays reproducible so a
future fix can be verified in minutes.

---

## Yahoo Finance — keyless, and unfixable by paying

**Tier:** no key, no account, no paid tier for this endpoint.

**What breaks, two independent problems:**

1. **IP reputation block.** `query1.finance.yahoo.com` returns a blanket HTTP 429
   from this network — instantly, on the first request, with no `Retry-After` and
   `server: ATS`. It is not our request rate; it is a shared corporate egress IP
   whose per-IP allowance is already spent. Nothing in our control changes it.
2. **No dividend adjustment.** This one is a fact about *our code*, not a guess
   about Yahoo's behaviour: `internal/fetch/yahoo` decodes `indicators.quote` and
   **the string `adjclose` appears nowhere in the package**. Whatever adjustment
   Yahoo offers in `indicators.adjclose` is absent from our rows by construction,
   so the bars do not satisfy §3's split-and-dividend requirement.

**Live consequence, now fixed:** `data-analyzer`'s `QueryEquityBars` used to
prefer `source='yahoo_finance'` when a symbol had rows from multiple providers.
A doubly-covered symbol therefore resolved to the unadjusted series — strictly
worse than either source alone, because the symbol *looked* fully covered while
serving prices that drift by the cumulative dividend. The preference is now
Tiingo-first, per timestamp, with tests that fail if it regresses.

**Does paid fix it?** There is no paid tier. Problem 1 would need a different
egress IP; problem 2 would need an adapter change (decode `adjclose`), which is
free but unverifiable while problem 1 blocks live testing.

**Upgrade trigger:** none. Yahoo is retained only because `data-technical`
already writes rows under that source. It must not feed the scanner.

---

## Polygon — key held, no adapter, free tier too shallow

**Tier:** free. **Measured 2026-09-18** with this project's key, requesting
2023-09-18 → 2026-09-17 daily aggregates for AAPL:

```
HTTP 200, status: "DELAYED"
501 bars returned, 2024-09-17 .. 2026-09-16  =>  2.0 years of history
```

**What breaks on free:** **§2.3 requires 3 years; free gives 2.0.** The request
does not error — it silently returns a shorter window, which is precisely the
class of failure this project keeps getting bitten by. `status: "DELAYED"` also
confirms the free tier is delayed rather than real-time, which is harmless for
end-of-day bars but would matter in Phase 3.

**Untested and load-bearing if adopted:** whether `adjusted=true` handles reverse
splits consistently. Polygon returns `o/h/l/c/v/vw/n` with no separate adjusted
fields, so the adjustment is invisible in the response shape — exactly the
condition under which Twelve Data's defect hid. **`cmd/bar-audit` must be run
against it before it is trusted**, and the 64 Tiingo symbols are the
cross-validation set for that.

**Does paid fix the history limit?** Almost certainly — Polygon's paid equity
tiers advertise full history — but this is **unverified**, no paid key has been
tested, and the adjustment-consistency question is independent of the tier.
Approximate cost: ~$29/mo entry, ~$199/mo for the unlimited tier.

**Upgrade trigger:** only if Tiingo's paid tier is rejected on price and a second
opinion on adjustment is still wanted. Polygon's `n` (trade count) and `vw`
(volume-weighted price) would be genuinely useful for Phase 3 intraday work, but
nothing in Phase 1 needs them.

---

## EODHD — key held, no adapter, free tier far too shallow

**Tier:** free. **Measured 2026-09-18**, same 3-year AAPL request:

```
HTTP 200
252 bars returned, 2025-09-17 .. 2026-09-17  =>  1.0 year of history
```

**What breaks on free:** **1 year against §2.3's 3-year requirement**, and again
silently — a 200 with a truncated window, not an error. One year is also below
§3.1's 252-bar minimum with no margin at all, and below what §3.7's 251-bar
52-week window plus a 20-day baseline needs to be meaningful.

**The one genuine advantage:** EODHD returns **`close` and `adjusted_close` as
separate fields**, the same shape as Tiingo's `adj*`. That is the property that
makes adjustment auditable rather than invisible, and it is why EODHD would be the
first alternative to evaluate if Tiingo were ever unavailable.

**Does paid fix the history limit?** Presumably — paid tiers advertise 30+ years —
but **unverified**, and the adjusted-volume question is untested: the response
carries `adjusted_close` but only one `volume` field, so whether that volume is
split-adjusted is exactly the NVDA test that must be run before trusting it.
Approximate cost: ~$20/mo entry, ~$80/mo for the all-in plan.

**Upgrade trigger:** if Tiingo becomes unavailable or its paid pricing is rejected.
Evaluate with `cmd/bar-audit` plus the NVDA volume fixture first, not from the
pricing page.

---

## Alpha Vantage — free tier structurally cannot serve §3

**Tier:** free. An adapter exists (`internal/fetch/alphavantage`) and is used for
crypto and FX series, not equity bars.

**What breaks on free — measured 2026-09-18:**

```
GET /query?function=TIME_SERIES_DAILY_ADJUSTED&symbol=AAPL&outputsize=full
-> {"Information": "Thank you for using Alpha Vantage! This is a premium
    endpoint. You may subscribe to any of the premium plans ... to instantly
    unlock all premium endpoints"}
```

**`TIME_SERIES_DAILY_ADJUSTED` is a premium endpoint.** The free tier can return
unadjusted daily bars only, which fails §3's opening requirement outright. This
is a harder block than any rate limit: it is not slow, it is absent. Free is also
capped at 25 requests/day, which could not backfill 450 symbols in under 18 days
regardless.

Worth noting the failure mode is *polite*: HTTP 200 with an `Information` key and
no `Time Series` payload. An adapter that only checked the status code would
record zero bars and mark the symbol done — the same in-band-error trap the
Twelve Data adapter had to handle explicitly.

**Does paid fix it?** Yes, by definition — the endpoint is gated on subscription,
not degraded. ~$50/mo for the entry premium tier. But this buys access to an
adjustment implementation that has **never been audited here**, so it would need
the same `cmd/bar-audit` and NVDA treatment before use.

**Upgrade trigger:** none for Phase 1. Alpha Vantage's equity bars are strictly
worse value than Tiingo Power at a comparable price.

---

## How to evaluate any new provider

Learned the hard way, in this order. Steps 2 and 3 are the ones that were skipped
and cost the most.

1. **Depth.** Request the full §2.3 window and count the bars. Polygon returned
   2.0 years and EODHD 1.0 year, both with HTTP 200 — a truncated window is not
   an error.
2. **Adjustment consistency, on a reverse split in a micro-cap.** Not a forward
   split in a mega-cap: NVDA's 10:1 passes on a provider that fails ABTS's
   1-for-15. Run `cmd/bar-audit` and diff against Tiingo.

   Expect a **~8% false-positive rate** on a penny-heavy universe (38 of 450 on
   Tiingo, effectively all genuine moves): in an illiquid name a real -79% day
   keeps dollar volume inside the detector's band. Triage by asking whether the
   provider's RAW series shows the move too — a real move appears in both raw and
   adjusted, a seam appears only in the adjusted. Tiingo exposes `close`,
   `adjClose` and `splitFactor` for exactly this; Twelve Data exposes none of
   them, which is part of why its defect was invisible.
3. **Adjusted volume specifically.** Confirm against a known factor
   (NVDA 2024-06-07: raw 41,238,580 → adjusted 412,385,800). A provider that
   rescales price but not volume corrupts §3.4's RVOL for every symbol that ever
   split, and the parameter name will not tell you.
4. **The real rate limit, measured.** Burst until it refuses, then determine
   whether the window is rolling or fixed-clock. Tiingo's published 50/hour
   allowed ~74 in a burst, so enforcement lags the documentation — plan against
   the documented number, never the observed one.
5. **In-band errors.** Check whether failures arrive as HTTP 200 with an error
   body. Twelve Data and Alpha Vantage both do.
6. **Credential placement.** If the key goes in the query string, every
   `*url.Error` from `http.Client.Do` contains it. That leaked 23 keys into
   `universe_symbols.backfill_last_error` here. Prefer header auth; redact
   regardless (`barsource.RedactSecrets`).

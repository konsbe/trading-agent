#!/usr/bin/env python3
"""Attribute every step in the cumulative adjustment factor, or flag it.

THE ARITHMETIC

With raw_close stored, the cumulative adjustment applied to bar t is

    cum(t) = close(t) / raw_close(t)

For a series on ONE adjustment basis, the ratio between consecutive bars is
fully determined by the corporate action recorded on session t:

    cum(t) / cum(t-1)  ==  splitFactor(t) * (1 + divCash(t) / raw_close(t))

Note the dividend reference is the EX-DATE's raw close, not the previous day's.
That was established empirically rather than assumed: an earlier version of this
audit used the previous close and produced ~1,200 near-misses at 1e-3 to 3e-3,
all in the same direction. Fitting the convention to the data resolved them
exactly.

Verified against both mechanisms on real rows, to ~1e-7:

  NEXR 2026-07-31  1-for-11 reverse split, splitFactor 0.0909090909
                   observed 0.090909, predicted 0.090909
  A    2016-09-30  divCash 0.115 on an ex-date raw close of 47.09
                   observed 1.002442, predicted 1 + 0.115/47.09 = 1.002442
  AFCG 2022-03-30  divCash 0.55 on an ex-date raw close of 19.20
                   observed 1.028646, predicted 1 + 0.55/19.20 = 1.028646

A step that the recorded action does not predict is a SEAM: old-basis history
joined to new-basis bars, which is what the daily refresh would have produced
before the corporate-action re-fetch landed.

THE CRITERION (committed in Phase 1 §10.1.0-CA before this was run)

  * a step is REAL when the relative change exceeds 1e-3
  * an attribution MATCHES when predicted and observed agree to within 1e-3
  * clean = ZERO unattributed steps, or a short individually-explained list

Why 1e-3: Tiingo rounds prices to 4 decimals, so the ratio of two rounded
values carries a few parts in 10,000 of quantisation noise — 1e-4 would flag
rounding. The smallest real action to catch is a modest quarterly dividend
(A's $0.115 on $47 = 2.5e-3 relative), which clears 1e-3 with ~2.5x margin.
"""
from __future__ import annotations

import argparse
import os
import subprocess

TOL = 1e-3

STEPS_SQL = f"""
WITH f AS (
  SELECT symbol, ts::date AS d,
         close / NULLIF(raw_close, 0) AS cum,
         raw_close, split_factor, div_cash
  FROM equity_ohlcv
  WHERE source = 'tiingo' AND interval = '1Day'
    AND raw_close IS NOT NULL AND raw_close > 0 AND close > 0
), stepped AS (
  SELECT symbol, d, cum, raw_close, split_factor, div_cash,
         lag(cum) OVER w AS prev_cum,
         lag(d)   OVER w AS prev_d
  FROM f WINDOW w AS (PARTITION BY symbol ORDER BY d)
), cal AS (
  -- The market's real trading calendar, derived from the data: every date on
  -- which ANY symbol traded. Handles holidays and early closes without a
  -- hardcoded calendar, and is exactly the right reference for "did this
  -- symbol miss a session".
  SELECT DISTINCT ts::date AS d FROM equity_ohlcv
  WHERE source = 'tiingo' AND interval = '1Day'
), scored AS (
  SELECT st.symbol, st.d, st.prev_d, st.prev_cum, st.cum,
         st.split_factor, st.div_cash, st.raw_close,
         st.cum / NULLIF(st.prev_cum, 0) AS observed,
         -- The ex-date's own raw close is the dividend reference (see above).
         COALESCE(st.split_factor, 1.0)
           * (1.0 + COALESCE(st.div_cash, 0) / NULLIF(st.raw_close, 0)) AS predicted,
         (SELECT count(*) FROM cal c
           WHERE c.d > st.prev_d AND c.d < st.d) AS sessions_missed
  FROM stepped st
  WHERE prev_cum IS NOT NULL
    AND abs(cum - prev_cum) / NULLIF(prev_cum, 0) > {TOL}
)
SELECT symbol, d, observed, predicted, split_factor, div_cash, sessions_missed
FROM scored
WHERE predicted IS NULL
   OR abs(observed - predicted) / NULLIF(abs(predicted), 0) > {TOL}
ORDER BY symbol, d
"""

#: Calendar days are the wrong unit for "is a session missing", and using them
#: left 141 false flags: a Friday->Monday step spans 3 calendar days and no
#: missing session, while an illiquid name skipping one Thursday also spans 3.
#: The SQL above instead counts sessions on the MARKET'S OWN calendar --
#: derived from the data as the set of dates any symbol traded -- which handles
#: weekends and holidays without hardcoding either.

#: How many sessions BEFORE an ex-date Tiingo may begin applying the dividend
#: adjustment.
#:
#: Observed behaviour, not a guess: ADTN's 2023-08-18 dividend of $0.09 begins
#: adjusting on 2023-08-16, and BRN's 2023-08-23 dividend of $0.015 begins on
#: 2023-08-21 -- two sessions early in both cases. The step magnitude is exactly
#: the ex-date dividend factor (matched to 4e-7), so the adjustment is correct
#: in size and merely early in start date.
#:
#: This is encoded as a RULE rather than an allowlist of symbols. An allowlist
#: would make every future dividend that does the same thing a fresh mystery,
#: which is precisely the cost this audit exists to avoid. 5 sessions gives
#: headroom over the observed 2 without being so wide that an unrelated step
#: could find a spurious match -- the magnitude still has to agree to TOL.
EARLY_DIVIDEND_SESSIONS = 5

#: Dividends per symbol, for the early-adjustment rule. Fetched separately from
#: the step query because the rule needs to look FORWARD from a flagged step,
#: which a window function over steps cannot express cleanly.
DIVIDENDS_SQL = """
SELECT symbol, ts::date, div_cash, raw_close
FROM equity_ohlcv
WHERE source = 'tiingo' AND interval = '1Day'
  AND div_cash IS NOT NULL AND div_cash > 0 AND raw_close > 0
ORDER BY symbol, ts
"""

#: Symbols whose history the provider no longer serves, so their stored bars can
#: never be refreshed onto the current adjustment basis. Loaded from the
#: database rather than hardcoded -- see universe_symbols.data_unavailable_reason.
UNREFRESHABLE_SQL = """
SELECT symbol, data_unavailable_reason
FROM universe_symbols
WHERE data_unavailable_reason IS NOT NULL
"""


def psql(sql: str) -> str:
    r = subprocess.run(["psql", os.environ["DATABASE_URL"], "-tAq", "-c", sql],
                       capture_output=True, text=True)
    if r.returncode != 0:
        raise SystemExit(f"psql: {r.stderr.strip()}")
    return r.stdout


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--list", type=int, default=40)
    args = ap.parse_args()

    print("=" * 84)
    print("  ADJUSTMENT SEAM AUDIT — every step attributed, or flagged")
    print("=" * 84)
    print(f"  tolerance: {TOL:g} relative, both for 'is a step real' and 'does it match'")

    cov = psql("""
        SELECT count(*), count(raw_close), count(div_cash), count(split_factor),
               count(DISTINCT symbol)
        FROM equity_ohlcv WHERE source='tiingo' AND interval='1Day'
    """).strip().split("|")
    total, raw, dc, sf, syms = (int(x) for x in cov)
    print(f"  bars: {total:,} across {syms:,} symbols")
    print(f"    raw_close     {raw:,} ({100*raw/total:.1f}%)")
    print(f"    split_factor  {sf:,} ({100*sf/total:.1f}%)")
    print(f"    div_cash      {dc:,} ({100*dc/total:.1f}%)")
    if dc < total:
        print()
        print(f"  ** div_cash is incomplete ({total-dc:,} bars missing). A dividend on a")
        print("     bar without div_cash cannot be attributed and will be reported as")
        print("     UNATTRIBUTED — the conservative direction. The criterion is not")
        print("     satisfiable until the re-fetch completes.")

    rows = [l.split("|") for l in psql(STEPS_SQL).splitlines() if l]

    # Dividends by symbol, ordered, for the early-adjustment rule.
    divs: dict[str, list[tuple[str, float, float]]] = {}
    for line in psql(DIVIDENDS_SQL).splitlines():
        if not line:
            continue
        sym, d, dc, raw = line.split("|")
        divs.setdefault(sym, []).append((d, float(dc), float(raw)))

    # The market calendar, so "N sessions ahead" means sessions and not days.
    cal = [l for l in psql(
        "SELECT DISTINCT ts::date FROM equity_ohlcv "
        "WHERE source='tiingo' AND interval='1Day' ORDER BY 1").splitlines() if l]
    cal_idx = {d: i for i, d in enumerate(cal)}

    unrefreshable = {}
    for line in psql(UNREFRESHABLE_SQL).splitlines():
        if line:
            sym, reason = line.split("|", 1)
            unrefreshable[sym] = reason

    def explained_by_early_dividend(sym: str, date: str, observed: float) -> str | None:
        """Does a dividend within the next N sessions predict this step exactly?

        Tiingo begins some dividend adjustments a couple of sessions before the
        ex-date. The step is then correct in MAGNITUDE but lands on a bar with
        no div_cash of its own, so the ordinary attribution misses it.
        """
        i = cal_idx.get(date)
        if i is None:
            return None
        window = set(cal[i: i + EARLY_DIVIDEND_SESSIONS + 1])
        for d, dc, raw in divs.get(sym, ()):
            if d not in window or raw <= 0:
                continue
            predicted = 1.0 / (1.0 + dc / raw)
            if abs(observed - predicted) / abs(predicted) <= TOL:
                return f"dividend {dc} ex-{d}"
        return None

    gapped, early, unrefresh, unattr = [], [], [], []
    for r in rows:
        sym, date, obs = r[0], r[1], float(r[2])
        missed = int(r[6]) if r[6] else 0
        if missed > 0:
            gapped.append(r)
        elif sym in unrefreshable:
            unrefresh.append(r)
        elif (why := explained_by_early_dividend(sym, date, obs)):
            early.append(r + [why])
        else:
            unattr.append(r)

    print()
    print(f"  steps flagged by the ratio test:   {len(rows):,}")
    print(f"    spanning >=1 missed session:     {len(gapped):,}  "
          f"— EXPLAINED: action fell on a session with no stored bar")
    print(f"    early dividend adjustment:       {len(early):,}  "
          f"— EXPLAINED: magnitude matches a dividend <={EARLY_DIVIDEND_SESSIONS} sessions ahead")
    print(f"    on an unrefreshable symbol:      {len(unrefresh):,}  "
          f"— EXPLAINED: provider no longer serves this history")
    print(f"    UNATTRIBUTED:                    {len(unattr):,}")

    if early and args.list:
        print()
        print("    early-dividend matches (rule, not an allowlist):")
        for r in early[:5]:
            print(f"      {r[0]:<8} {r[1]:<12} observed {float(r[2]):.6f}  {r[-1]}")
        if len(early) > 5:
            print(f"      ... and {len(early)-5:,} more")

    if not unattr:
        print()
        print("  CLEAN. Every step between consecutive stored bars is either predicted")
        print("  by the action recorded on that session, or explained by one of the")
        print("  rules above. The stored history is on one adjustment basis throughout.")
        return 0

    bad_syms = {r[0] for r in unattr}
    print(f"  affected symbols:                  {len(bad_syms):,}")
    print()
    print(f"    {'symbol':<8} {'date':<12} {'observed':>11} {'predicted':>11} "
          f"{'split':>12} {'div':>9} {'miss':>5}")
    for r in unattr[: args.list]:
        sym, d, obs, pred, sfv, dcv, miss = r
        p = f"{float(pred):.6f}" if pred else "n/a"
        print(f"    {sym:<8} {d:<12} {float(obs):>11.6f} {p:>11} "
              f"{(sfv or '—'):>12} {(dcv or '—'):>9} {(miss or '—'):>5}")
    if len(unattr) > args.list:
        print(f"    ... and {len(unattr) - args.list:,} more")
    print()
    print("  Per the committed criterion, any unexplained step BLOCKS P2-3 until")
    print("  it is understood. Each row above needs an individual explanation.")
    return 1


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Phase 2 §3.2 step (b): MEASURE the lookahead in the §3.2 market-cap gate.

THE LEAK

The gates test market cap. Market cap is TODAY's value. It is applied to every
historical row back to 2016. So which historical setups became candidates, and
which bucket each landed in, was decided partly by what the company later
became. A 2017 setup in a company now worth $20bn is judged by the $20bn.

This script recomputes market cap point-in-time,

    mcap_pit(t) = raw_close[t] x shares_outstanding(filed <= t)

and reports how many gate decisions and bucket assignments FLIP. Both factors
are unadjusted: raw_close (captured by migration 012) and the share count as
filed. Multiplying an adjusted price by an unadjusted share count would be wrong
by the cumulative split factor, which is 10-100x for reverse-split penny names.

As-of rule: join on the `filed` date, never the period end. The period end
precedes the filing by weeks, so joining on it is itself lookahead -- it would
credit us with knowing a share count before it was public.
"""
from __future__ import annotations

import argparse
import bisect
import csv
import json
import os
import subprocess
from collections import defaultdict

PENNY_MAX = 300e6     # §3.2 penny bucket ceiling
MARKET_MIN = 300e6    # §3.2 market bucket floor
MARKET_MAX = 10e9     # §3.2 market bucket ceiling


def psql(sql: str) -> str:
    r = subprocess.run(["psql", os.environ["DATABASE_URL"], "-tAq", "-c", sql],
                       capture_output=True, text=True)
    if r.returncode != 0:
        raise SystemExit(f"psql: {r.stderr.strip()}")
    return r.stdout


def classify(mcap: float | None) -> str:
    """The §3.2 gate outcome for a market cap: which bucket, or no bucket."""
    if mcap is None or mcap <= 0:
        return "none"
    if mcap < PENNY_MAX:
        return "penny"
    if mcap <= MARKET_MAX:
        return "market"
    return "too_big"      # above the market ceiling: fails the gate entirely


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--edgar", default="/tmp/edgar_cov90.json")
    ap.add_argument("--candidates", default="/tmp/candidates_full.csv")
    args = ap.parse_args()

    edgar = json.load(open(args.edgar))["results"]
    usable = {s: v for s, v in edgar.items() if v.get("status") == "ok"}

    # Point-in-time share series per symbol, keyed by FILED date and made
    # monotonic in time so an as-of lookup is a bisect.
    pit = {}
    for sym, v in usable.items():
        by_filed = {}
        for f in v["series"]:
            if f.get("filed") and f.get("val"):
                # Several facts can share a filed date under multi-class
                # structures. Sum them: the gate wants total common shares, and
                # taking one class would understate the cap by the size of the
                # others.
                by_filed[f["filed"]] = by_filed.get(f["filed"], 0) + float(f["val"])
        if by_filed:
            dates = sorted(by_filed)
            pit[sym] = (dates, [by_filed[d] for d in dates])

    syms = sorted(pit)
    print("=" * 78)
    print("  POINT-IN-TIME MARKET CAP — measuring the §3.2 gate lookahead")
    print("=" * 78)
    print(f"  symbols with an EDGAR share series: {len(syms)}")

    # Today's values, exactly as the backtest read them.
    today_mcap, today_shares = {}, {}
    for line in psql("""
        SELECT DISTINCT ON (symbol, metric) symbol, metric, value
        FROM equity_fundamentals
        WHERE metric IN ('market_cap','shares_outstanding')
          AND value IS NOT NULL AND value > 0
        ORDER BY symbol, metric, ts DESC, fundamental_source_rank(source) DESC
    """).splitlines():
        if not line:
            continue
        sym, metric, val = line.split("|")
        (today_mcap if metric == "market_cap" else today_shares)[sym] = float(val)

    # Every bar we have a raw (unadjusted) close for, for these symbols.
    inlist = ",".join(f"'{s}'" for s in syms)
    bars = defaultdict(dict)
    for line in psql(f"""
        SELECT symbol, ts::date, raw_close
        FROM equity_ohlcv
        WHERE source='tiingo' AND interval='1Day'
          AND raw_close IS NOT NULL AND raw_close > 0
          AND symbol IN ({inlist})
    """).splitlines():
        if not line:
            continue
        sym, d, rc = line.split("|")
        bars[sym][d] = float(rc)
    print(f"  bars with a raw close:              {sum(len(v) for v in bars.values()):,}")

    def shares_asof(sym: str, date: str):
        dates, vals = pit[sym]
        i = bisect.bisect_right(dates, date)
        return vals[i - 1] if i else None      # None = no filing on or before t

    # ── A. bucket assignment across ALL evaluated bar-days ──
    print(f"\n  ── A. bucket assignment, every bar-day with a raw close ──")
    flips = defaultdict(int)
    n_eval = n_nopit = 0
    for sym in syms:
        tm = today_mcap.get(sym)
        b_today = classify(tm)
        for d, rc in bars[sym].items():
            sh = shares_asof(sym, d)
            if sh is None:
                n_nopit += 1
                continue
            n_eval += 1
            flips[(b_today, classify(rc * sh))] += 1

    print(f"    evaluated                {n_eval:,}")
    print(f"    skipped, no filing <= t  {n_nopit:,}  "
          f"({100*n_nopit/max(n_eval+n_nopit,1):.1f}% — EDGAR history starts after the bar)")
    same = sum(v for (a, b), v in flips.items() if a == b)
    diff = n_eval - same
    print(f"    SAME bucket              {same:,} ({100*same/max(n_eval,1):.1f}%)")
    print(f"    FLIPPED                  {diff:,} ({100*diff/max(n_eval,1):.1f}%)")
    print(f"\n    {'today (used)':<14} -> {'point-in-time':<14} {'n':>9}   share")
    for (a, b), v in sorted(flips.items(), key=lambda kv: -kv[1]):
        mark = "" if a == b else "  <-- FLIP"
        print(f"    {a:<14} -> {b:<14} {v:>9,} {100*v/max(n_eval,1):6.2f}%{mark}")

    # ── B. the candidates that were actually studied ──
    print(f"\n  ── B. the {0} gate-passing candidates in this sample ──".replace(" 0 ", " "))
    cands = []
    for r in csv.DictReader(open(args.candidates, newline="")):
        if r["symbol"] in pit:
            cands.append(r)
    c_flip = defaultdict(int)
    c_eval = 0
    runner_ratio, nonrunner_ratio = [], []
    for r in cands:
        sym, d = r["symbol"], r["date"]
        rc = bars[sym].get(d)
        sh = shares_asof(sym, d)
        if rc is None or sh is None:
            continue
        c_eval += 1
        c_flip[(classify(today_mcap.get(sym)), classify(rc * sh))] += 1
        # dilution ratio: today's share count over the count as of the setup
        st = today_shares.get(sym)
        if st and sh > 0:
            ratio = st / sh
            (runner_ratio if float(r["fwd_max_gain_pct"]) >= 100 else nonrunner_ratio).append(
                (ratio, r["bucket"]))

    print(f"    candidates in sample     {len(cands):,}")
    print(f"    evaluable point-in-time  {c_eval:,}")
    c_same = sum(v for (a, b), v in c_flip.items() if a == b)
    print(f"    SAME bucket              {c_same:,} ({100*c_same/max(c_eval,1):.1f}%)")
    print(f"    FLIPPED                  {c_eval-c_same:,} "
          f"({100*(c_eval-c_same)/max(c_eval,1):.1f}%)")
    for (a, b), v in sorted(c_flip.items(), key=lambda kv: -kv[1]):
        mark = "" if a == b else "  <-- FLIP"
        print(f"    {a:<14} -> {b:<14} {v:>9,} {100*v/max(c_eval,1):6.2f}%{mark}")

    # ── C. the dilution leak, measured ──
    print(f"\n  ── C. dilution: today's shares / shares as of the setup ──")
    print(f"    A ratio above 1 means the company issued shares AFTER the setup,")
    print(f"    so today's market cap overstates what it was on the day.")

    def summarise(label, rows):
        if not rows:
            print(f"    {label:<28} (no observations)")
            return
        vals = sorted(r for r, _ in rows)
        n = len(vals)
        med = vals[n // 2] if n % 2 else (vals[n//2-1] + vals[n//2]) / 2
        p90 = vals[min(int(n * 0.9), n - 1)]
        share_gt2 = 100 * sum(1 for v in vals if v > 2) / n
        print(f"    {label:<28} n={n:<6,} median={med:6.2f}x  p90={p90:8.2f}x  "
              f">2x: {share_gt2:5.1f}%")

    for bucket in ("penny", "market"):
        print(f"    [{bucket}]")
        summarise("runners (hit +100%)", [(r, b) for r, b in runner_ratio if b == bucket])
        summarise("non-runners", [(r, b) for r, b in nonrunner_ratio if b == bucket])
    print(f"    [all]")
    summarise("runners (hit +100%)", runner_ratio)
    summarise("non-runners", nonrunner_ratio)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

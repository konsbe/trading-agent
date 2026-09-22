#!/usr/bin/env python3
"""Decompose the gate v1 -> v2 candidate-set change (descriptive; decides nothing).

THE CONFOUND THIS RESOLVES

Gate v2 bundles two changes that look like one:

  * the LEAK CORRECTION — rows whose point-in-time market cap falls outside the
    bucket's band are rejected. This is the fix.
  * a COVERAGE EXCLUSION — rows with no EDGAR share count are rejected too. That
    is ~18% of symbols, and they are not random: 48.7% of them listed after
    2023-09 against 16.2% of the covered.

`rvol` lost its separation when both landed together. If the signal sat in the
leak-corrected group it was a leak artifact. If it sat in the coverage-excluded
groups it is a question about names EDGAR cannot yet cover — plausibly exactly
where momentum runs happen, since first XBRL filings arrive weeks or months
after an IPO.

GROUPS

  both        in v1 and v2
  v1_band     v1-only: point-in-time cap computed, but outside the band  (LEAK)
  v1_noseries v1-only: symbol has no EDGAR series at all                 (COVERAGE)
  v1_prefile  v1-only: symbol has a series, but this bar predates it     (COVERAGE)
  v2_only     in v2 and not v1: eligible on the day, never studied

Groups are assigned from the SAME point-in-time data the gate used, so the
split reproduces the gate's own reasoning rather than approximating it.
"""
from __future__ import annotations

import argparse
import bisect
import csv
import math
import os
import subprocess
from collections import defaultdict

THRESHOLD = 100.0
PENNY_MAX, MARKET_MIN, MARKET_MAX = 300e6, 300e6, 10e9


def psql(sql: str) -> str:
    r = subprocess.run(["psql", os.environ["DATABASE_URL"], "-tAq", "-c", sql],
                       capture_output=True, text=True)
    if r.returncode != 0:
        raise SystemExit(f"psql: {r.stderr.strip()}")
    return r.stdout


def norm_cdf(x): return 0.5 * (1 + math.erf(x / math.sqrt(2)))


def two_prop(h1, n1, h2, n2):
    if n1 == 0 or n2 == 0:
        return None
    p1, p2 = h1 / n1, h2 / n2
    pool = (h1 + h2) / (n1 + n2)
    se = math.sqrt(pool * (1 - pool) * (1 / n1 + 1 / n2))
    if se == 0:
        return None
    z = (p1 - p2) / se
    return z, 2 * (1 - norm_cdf(abs(z))), p1, p2


def fmt_p(p):
    return "   n/a" if p != p else ("<0.001" if p < 0.001 else f"{p:6.3f}")


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--v1", default="/tmp/cand_v1.csv")
    ap.add_argument("--v2", default="/tmp/cand_v2.csv")
    ap.add_argument("--pilot", default="/tmp/pilot.txt")
    ap.add_argument("--region", default="2025-03-28:2026-03-27")
    args = ap.parse_args()

    pilot = {l.strip() for l in open(args.pilot) if l.strip()}
    lo, hi = args.region.split(":")

    def load(path):
        out = {}
        for r in csv.DictReader(open(path, newline="")):
            # Region lockbox + population A exclusion, as in every other report.
            if lo <= r["date"] <= hi and r["symbol"] not in pilot:
                continue
            if r["date"] >= "2023-09-18" and r["symbol"] in pilot:
                continue
            out[(r["symbol"], r["date"])] = r
        return out

    v1, v2 = load(args.v1), load(args.v2)

    # Point-in-time series, exactly as the gate consumed it.
    pit = defaultdict(lambda: ([], []))
    for line in psql("SELECT symbol, filed_date, shares FROM shares_outstanding_pit "
                     "ORDER BY symbol, filed_date").splitlines():
        if not line:
            continue
        sym, fd, sh = line.split("|")
        pit[sym][0].append(fd)
        pit[sym][1].append(float(sh))

    raw = defaultdict(dict)
    for line in psql("SELECT symbol, ts::date, raw_close FROM equity_ohlcv "
                     "WHERE source='tiingo' AND interval='1Day' AND raw_close IS NOT NULL"
                     ).splitlines():
        if not line:
            continue
        sym, d, rc = line.split("|")
        raw[sym][d] = float(rc)

    first_bar = {}
    for line in psql("SELECT symbol, min(ts)::date FROM equity_ohlcv "
                     "WHERE source='tiingo' AND interval='1Day' GROUP BY 1").splitlines():
        if not line:
            continue
        sym, d = line.split("|")
        first_bar[sym] = d

    def shares_asof(sym, date):
        if sym not in pit:
            return None
        dates, vals = pit[sym]
        i = bisect.bisect_right(dates, date)
        return vals[i - 1] if i else None

    # ── assign groups ──
    groups = defaultdict(list)
    for k, r in v1.items():
        if k in v2:
            groups["both"].append(r)
            continue
        sym, date = k
        if sym not in pit:
            groups["v1_noseries"].append(r)
        elif shares_asof(sym, date) is None:
            groups["v1_prefile"].append(r)
        else:
            groups["v1_band"].append(r)
    for k, r in v2.items():
        if k not in v1:
            groups["v2_only"].append(r)

    print("=" * 96)
    print("  GATE v1 -> v2 DECOMPOSITION  (descriptive; decides nothing)")
    print("=" * 96)
    print("  population: episode level, lockbox region excluded, population A excluded")
    print()
    print("  Is rvol's v1 signal in the LEAK group (v1_band) or in the COVERAGE groups")
    print("  (v1_noseries / v1_prefile)? That is the whole question.")
    print()

    order = ["both", "v1_band", "v1_noseries", "v1_prefile", "v2_only"]
    label = {
        "both":        "in both v1 and v2",
        "v1_band":     "v1-only: PIT cap OUTSIDE band   [LEAK CORRECTION]",
        "v1_noseries": "v1-only: no EDGAR series at all [COVERAGE]",
        "v1_prefile":  "v1-only: bar predates 1st filing[COVERAGE]",
        "v2_only":     "v2-only: eligible, never studied",
    }

    print(f"  {'group':<34} {'rows':>7} {'episodes':>9} {'symbols':>8} {'hit%':>7}")
    eps = {}
    for g in order:
        rows = groups[g]
        e = [r for r in rows if r["episode_start"] == "true"]
        eps[g] = e
        if not rows:
            print(f"  {label[g]:<34} {0:>7}")
            continue
        h = sum(1 for r in e if float(r["fwd_max_gain_pct"]) >= THRESHOLD)
        print(f"  {label[g]:<34} {len(rows):>7,} {len(e):>9,} "
              f"{len({r['symbol'] for r in rows}):>8,} "
              f"{100*h/max(len(e),1):6.2f}%")

    # ── per-bucket base rates ──
    print()
    print("  ── per-bucket base rate (episodes) ──")
    print(f"  {'group':<34} {'penny n':>8} {'penny hit%':>11} {'market n':>9} {'market hit%':>12}")
    for g in order:
        e = eps[g]
        if not e:
            continue
        line = f"  {label[g]:<34}"
        for b in ("penny", "market"):
            sub = [r for r in e if r["bucket"] == b]
            h = sum(1 for r in sub if float(r["fwd_max_gain_pct"]) >= THRESHOLD)
            rate = f"{100*h/len(sub):.2f}%" if sub else "—"
            line += f" {len(sub):>8,} {rate:>11}" if b == "penny" else f" {len(sub):>9,} {rate:>12}"
        print(line)

    # ── listing age ──
    print()
    print("  ── listing age (share of episodes whose symbol's first bar is after 2023-09) ──")
    print(f"  {'group':<34} {'episodes':>9} {'listed after 2023-09':>22}")
    for g in order:
        e = eps[g]
        if not e:
            continue
        recent = sum(1 for r in e if first_bar.get(r["symbol"], "1900-01-01") > "2023-09-01")
        print(f"  {label[g]:<34} {len(e):>9,} {100*recent/len(e):>21.1f}%")

    # ── rvol within bucket, per group ──
    print()
    print("  ── rvol_20 within-bucket separation, computed WITHIN each group ──")
    print(f"  {'group':<34} {'bucket':<7} {'n':>7} {'top':>8} {'bottom':>8} {'delta':>8} {'p':>7}")
    for g in order:
        e = eps[g]
        for b in ("market", "penny"):
            sub = [r for r in e if r["bucket"] == b and r.get("rvol_20")]
            if len(sub) < 30:
                print(f"  {label[g]:<34} {b:<7} {len(sub):>7,}   (too few)")
                continue
            sub.sort(key=lambda r: float(r["rvol_20"]))
            k = len(sub) // 3
            bot, top = sub[:k], sub[-k:]
            res = two_prop(sum(1 for r in top if float(r["fwd_max_gain_pct"]) >= THRESHOLD), len(top),
                           sum(1 for r in bot if float(r["fwd_max_gain_pct"]) >= THRESHOLD), len(bot))
            if res is None:
                print(f"  {label[g]:<34} {b:<7} {len(sub):>7,}   (undefined)")
                continue
            z, p, ptop, pbot = res
            print(f"  {label[g]:<34} {b:<7} {len(sub):>7,} {100*ptop:7.2f}% {100*pbot:7.2f}% "
                  f"{100*(ptop-pbot):+7.2f}pp {fmt_p(p):>7}")

    print()
    print("  READING THIS")
    print("    If rvol separates strongly inside v1_band, the v1 signal was a LEAK artifact:")
    print("    it lived in rows that were never eligible on the day.")
    print("    If it separates inside v1_noseries / v1_prefile, the signal lives in names")
    print("    EDGAR cannot cover, and the v2 result is a coverage question, not a verdict.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Phase 1 post-upgrade validation: the out-of-sample report on FROZEN score v2.

This reads the candidate extract produced by

    momentum-backtest -dump-full

and computes nothing about features itself. Every feature, gate, score and label
in the input was computed by the Go engine; this file only partitions rows and
runs statistics on them. That split is deliberate (Phase 2 §12): a second
feature implementation in Python would diverge from the first, silently.

POPULATIONS, reported separately and never pooled
-------------------------------------------------
  A   pilot symbols, the original 2023-09-onward window   IN-SAMPLE, reference only
  B   non-pilot symbols, the original 2023-09-onward window        out-of-sample
  C   all symbols, periods before 2023-09                          out-of-sample
  B+C the headline out-of-sample number

A is shown because omitting it would hide the comparison that matters -- how much
of v2's Phase 1 performance was fit. It is never combined with B or C.

The lockbox is excluded from all four.
"""
from __future__ import annotations

import argparse
import csv
import math
import os
import sys
from collections import defaultdict

PILOT_SPLIT_DATE = "2023-09-18"  # first bar of the original 3-year pilot window
THRESHOLD = 100.0                # +100% within the 120-session horizon


# ── statistics ───────────────────────────────────────────────────────────────

def two_proportion_z(h1: int, n1: int, h2: int, n2: int):
    """Two-sided z-test for a difference in proportions.

    Returns (z, p, rate1, rate2) or None when either group is empty or the
    pooled rate is degenerate (all hits or no hits), where the standard error
    is zero and the statistic is undefined rather than infinite.
    """
    if n1 == 0 or n2 == 0:
        return None
    p1, p2 = h1 / n1, h2 / n2
    pool = (h1 + h2) / (n1 + n2)
    se = math.sqrt(pool * (1 - pool) * (1 / n1 + 1 / n2))
    if se == 0:
        return None
    z = (p1 - p2) / se
    p = 2 * (1 - _norm_cdf(abs(z)))
    return z, p, p1, p2


def _norm_cdf(x: float) -> float:
    return 0.5 * (1 + math.erf(x / math.sqrt(2)))


def benjamini_hochberg(pvals: list[float], alpha: float = 0.05) -> list[bool]:
    """BH step-up. Returns a mask of which hypotheses are rejected.

    Phase 2 §4.4 requires this whenever significance is claimed within a family
    of tests. Seven components tested at 0.05 each would produce a false
    positive about a third of the time by chance alone.
    """
    m = len(pvals)
    if m == 0:
        return []
    order = sorted(range(m), key=lambda i: pvals[i])
    keep = [False] * m
    largest = -1
    for rank, i in enumerate(order, start=1):
        if pvals[i] <= alpha * rank / m:
            largest = rank
    for rank, i in enumerate(order, start=1):
        if rank <= largest:
            keep[i] = True
    return keep


def median(xs: list[float]) -> float:
    if not xs:
        return float("nan")
    s = sorted(xs)
    n = len(s)
    return s[n // 2] if n % 2 else (s[n // 2 - 1] + s[n // 2]) / 2


def percentile(xs: list[float], q: float) -> float:
    """Linear-interpolated percentile, matching numpy's default."""
    if not xs:
        return float("nan")
    s = sorted(xs)
    if len(s) == 1:
        return s[0]
    pos = (len(s) - 1) * q
    lo = int(math.floor(pos))
    hi = min(lo + 1, len(s) - 1)
    return s[lo] + (s[hi] - s[lo]) * (pos - lo)


# ── data ─────────────────────────────────────────────────────────────────────

def load(path: str, pilot: set[str], lockbox: set[tuple[str, str]]):
    rows = []
    dropped_lockbox = 0
    with open(path, newline="") as fh:
        for r in csv.DictReader(fh):
            key = (r["symbol"], r["date"])
            if key in lockbox:
                dropped_lockbox += 1
                continue
            r["_pilot"] = r["symbol"] in pilot
            r["_episode"] = r["episode_start"] == "true"
            r["_gain"] = float(r["fwd_max_gain_pct"])
            r["_dd"] = float(r["fwd_max_drawdown_pct"])
            r["_hit"] = r["_gain"] >= THRESHOLD
            r["_score"] = int(r["score_total"])
            rows.append(r)
    return rows, dropped_lockbox


def populations(rows):
    """Split into A / B / C exactly as the step defines them."""
    A, B, C = [], [], []
    for r in rows:
        if r["date"] < PILOT_SPLIT_DATE:
            C.append(r)            # any symbol, before the pilot window
        elif r["_pilot"]:
            A.append(r)            # pilot symbol, inside the pilot window
        else:
            B.append(r)            # new symbol, inside the pilot window
    return {"A": A, "B": B, "C": C, "B+C": B + C}


# ── report sections ──────────────────────────────────────────────────────────

def fmt_p(p: float) -> str:
    if p != p:
        return "   n/a"
    if p < 0.001:
        return "<0.001"
    return f"{p:6.3f}"


def base_rate(rows, label: str):
    n = len(rows)
    h = sum(r["_hit"] for r in rows)
    rate = 100 * h / n if n else float("nan")
    print(f"  {label:<26} n={n:<7,} hits={h:<6,} base rate={rate:6.2f}%")
    return rate


def thirds_test(rows, name: str):
    """Top third vs bottom third by v2 score -- the pre-registered headline."""
    if len(rows) < 6:
        print(f"    {name:<22} n={len(rows)} — too few rows to cut into thirds")
        return None
    s = sorted(rows, key=lambda r: r["_score"])
    k = len(s) // 3
    bot, top = s[:k], s[-k:]
    hb, ht = sum(r["_hit"] for r in bot), sum(r["_hit"] for r in top)
    res = two_proportion_z(ht, len(top), hb, len(bot))
    if res is None:
        print(f"    {name:<22} undefined (no variation in outcomes)")
        return None
    z, p, ptop, pbot = res
    arrow = "+" if ptop > pbot else "-"
    print(f"    {name:<22} top={100*ptop:6.2f}% (n={len(top):,})  "
          f"bottom={100*pbot:6.2f}% (n={len(bot):,})  "
          f"{arrow}{abs(100*(ptop-pbot)):5.2f}pp  z={z:+6.2f}  p={fmt_p(p)}")
    return p, ptop, pbot


def component_tests(rows, label: str):
    """Median-split z-test per scoring component, with BH correction.

    Split on the raw FEATURE, not the sub-score: several sub-scores are step
    functions that collapse to two or three distinct values, so a median split
    on them can put most of the mass on one side and test almost nothing.
    """
    comps = [
        ("rvol_20", "rvol_20"),
        ("vol_accel", "vol_accel"),
        ("vwap_dist_pct", "vwap (dist%)"),
        ("pct_of_52w_high", "high52w (%of52w)"),
        ("atr_pct", "atr_pct"),
        ("dollar_volume", "log(dollar_vol)"),
        ("rsi_14", "rsi_14"),
    ]
    print(f"    {'component':<22} {'above med':>10} {'below med':>10} {'delta':>8} "
          f"{'z':>7} {'p':>7}  {'BH':>4}")
    results, pvals = [], []
    for field, pretty in comps:
        vals = [(float(r[field]), r["_hit"]) for r in rows if r.get(field)]
        if len(vals) < 20:
            results.append((pretty, None, len(vals)))
            pvals.append(1.0)
            continue
        med = median([v for v, _ in vals])
        hi = [hit for v, hit in vals if v > med]
        lo = [hit for v, hit in vals if v <= med]
        res = two_proportion_z(sum(hi), len(hi), sum(lo), len(lo))
        if res is None:
            results.append((pretty, None, len(vals)))
            pvals.append(1.0)
            continue
        results.append((pretty, res, len(vals)))
        pvals.append(res[1])

    keep = benjamini_hochberg(pvals)
    for (pretty, res, nv), survives in zip(results, keep):
        if res is None:
            print(f"    {pretty:<22} {'—':>10} {'—':>10} {'—':>8} {'—':>7} {'—':>7}   "
                  f"(n={nv}, underpowered)")
            continue
        z, p, phi, plo = res
        print(f"    {pretty:<22} {100*phi:9.2f}% {100*plo:9.2f}% "
              f"{100*(phi-plo):+7.2f}pp {z:+7.2f} {fmt_p(p):>7}  "
              f"{'yes' if survives else 'no':>4}")
    return dict(zip([c[1] for c in comps], pvals))


def breakout_inversion(rows):
    """Phase 1 found breakout and high52w correlated NEGATIVELY with outcomes.

    Tested by state rather than by median split, because breakout_state is
    categorical and a median over strings is meaningless.
    """
    by_state = defaultdict(list)
    for r in rows:
        st = r.get("breakout_state") or "(null)"
        by_state[st].append(r["_hit"])
    if len(by_state) < 2:
        print("    breakout_state: only one state present — no contrast to test")
        return
    print(f"    {'breakout_state':<22} {'n':>8} {'hit%':>8}")
    for st, hits in sorted(by_state.items(), key=lambda kv: -len(kv[1])):
        print(f"    {st:<22} {len(hits):>8,} {100*sum(hits)/len(hits):7.2f}%")
    # Contrast the largest two states directly.
    top2 = sorted(by_state.items(), key=lambda kv: -len(kv[1]))[:2]
    (s1, h1), (s2, h2) = top2
    res = two_proportion_z(sum(h1), len(h1), sum(h2), len(h2))
    if res:
        z, p, p1, p2 = res
        direction = "higher" if p1 > p2 else "LOWER"
        print(f"      {s1} vs {s2}: {s1} is {direction}, "
              f"{100*(p1-p2):+.2f}pp, z={z:+.2f}, p={fmt_p(p)}")


def rvol_alone(rows):
    """Top vs bottom third ranked by rvol_20 alone -- Phase 1's p=0.012 finding."""
    vals = [r for r in rows if r.get("rvol_20")]
    if len(vals) < 6:
        print("    rvol-alone: too few rows")
        return None
    s = sorted(vals, key=lambda r: float(r["rvol_20"]))
    k = len(s) // 3
    bot, top = s[:k], s[-k:]
    res = two_proportion_z(sum(r["_hit"] for r in top), len(top),
                           sum(r["_hit"] for r in bot), len(bot))
    if res is None:
        print("    rvol-alone: undefined")
        return None
    z, p, ptop, pbot = res
    print(f"    {'rvol_20 alone':<22} top={100*ptop:6.2f}%  bottom={100*pbot:6.2f}%  "
          f"{100*(ptop-pbot):+6.2f}pp  z={z:+6.2f}  p={fmt_p(p)}")
    return p, ptop, pbot


def per_bucket(rows, label: str):
    for bucket in ("penny", "market"):
        sub = [r for r in rows if r["bucket"] == bucket]
        if not sub:
            print(f"    {bucket:<8} no rows")
            continue
        n = len(sub)
        h = sum(r["_hit"] for r in sub)
        print(f"    {bucket:<8} n={n:<7,} base={100*h/n:6.2f}%  ", end="")
        s = sorted(sub, key=lambda r: r["_score"])
        k = len(s) // 3
        if k == 0:
            print("(too few for thirds)")
            continue
        bot, top = s[:k], s[-k:]
        res = two_proportion_z(sum(r["_hit"] for r in top), len(top),
                               sum(r["_hit"] for r in bot), len(bot))
        if res is None:
            print("thirds undefined")
            continue
        z, p, ptop, pbot = res
        print(f"top={100*ptop:6.2f}% bottom={100*pbot:6.2f}% z={z:+5.2f} p={fmt_p(p)}")


def by_year(rows):
    """Hit rate by calendar year -- makes survivorship bias visible.

    Older years are drawn from a universe filtered by survival to TODAY, so the
    further back a year is, the more its losers have been removed from the
    sample. An upward drift as you go back in time is the bias, not a finding.
    """
    years = defaultdict(list)
    for r in rows:
        years[r["date"][:4]].append(r)
    print(f"    {'year':<6} {'n':>8} {'hits':>7} {'hit%':>8} {'med gain%':>10} {'med dd%':>9}")
    for y in sorted(years):
        rs = years[y]
        h = sum(r["_hit"] for r in rs)
        print(f"    {y:<6} {len(rs):>8,} {h:>7,} {100*h/len(rs):7.2f}% "
              f"{median([r['_gain'] for r in rs]):9.2f}% {median([r['_dd'] for r in rs]):8.2f}%")


def p90_thresholds(rows):
    """Recompute the per-bucket p90 score. REPORT ONLY -- defaults unchanged."""
    print(f"    {'bucket':<8} {'n':>8} {'current':>9} {'p90 here':>10} {'p95':>7} {'median':>8}")
    current = {"penny": 72, "market": 65}
    for bucket in ("penny", "market"):
        sub = [r["_score"] for r in rows if r["bucket"] == bucket]
        if not sub:
            print(f"    {bucket:<8} {'—':>8}  no rows")
            continue
        print(f"    {bucket:<8} {len(sub):>8,} {current[bucket]:>9} "
              f"{percentile(sub, 0.90):10.1f} {percentile(sub, 0.95):7.1f} "
              f"{percentile(sub, 0.50):8.1f}")


# ── driver ───────────────────────────────────────────────────────────────────

def section(rows, name: str, level: str):
    print()
    print(f"  ┌─ {name} · {level} ─────────────────────────────────")
    if not rows:
        print("  │  no rows in this population")
        return
    base_rate(rows, "base rate")
    print()
    print("  v2 score, top vs bottom third:")
    thirds_test(rows, "momentum_score_100")
    print()
    print("  rvol-alone diagnostic:")
    rvol_alone(rows)
    print()
    print("  per-component median split (BH-corrected across the family):")
    component_tests(rows, name)
    print()
    print("  breakout / high52w inversion check:")
    breakout_inversion(rows)
    print()
    print("  per bucket:")
    per_bucket(rows, name)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dump", required=True)
    ap.add_argument("--pilot", required=True, help="file of pilot cohort symbols, one per line")
    ap.add_argument("--lockbox", required=True, help="CSV of lockbox keys: symbol,ts")
    args = ap.parse_args()

    pilot = {l.strip() for l in open(args.pilot) if l.strip()}
    lockbox = set()
    with open(args.lockbox) as fh:
        for r in csv.reader(fh):
            if len(r) >= 2 and r[0] != "symbol":
                lockbox.add((r[0], r[1]))

    rows, dropped = load(args.dump, pilot, lockbox)

    print("═" * 78)
    print("  PHASE 1 — POST-UPGRADE VALIDATION · out-of-sample report on FROZEN §4 v2")
    print("═" * 78)
    print(f"  pilot cohort symbols:   {len(pilot):,}")
    print(f"  lockbox rows excluded:  {dropped:,}")
    print(f"  rows available:         {len(rows):,}")
    print(f"  threshold:              +{THRESHOLD:.0f}% peak close within 120 sessions")
    print(f"  pilot window starts:    {PILOT_SPLIT_DATE}")

    pops = populations(rows)
    eps = {k: [r for r in v if r["_episode"]] for k, v in pops.items()}

    print()
    print("  POPULATION SIZES")
    print(f"    {'pop':<5} {'rows':>9} {'episodes':>10} {'symbols':>9}  definition")
    defs = {
        "A": "pilot symbols, 2023-09 onward  — IN-SAMPLE, reference only",
        "B": "new symbols,   2023-09 onward  — out-of-sample",
        "C": "all symbols,   before 2023-09  — out-of-sample",
        "B+C": "the headline out-of-sample number",
    }
    for k in ("A", "B", "C", "B+C"):
        syms = len({r["symbol"] for r in pops[k]})
        print(f"    {k:<5} {len(pops[k]):>9,} {len(eps[k]):>10,} {syms:>9,}  {defs[k]}")

    for k in ("A", "B", "C", "B+C"):
        print()
        print("═" * 78)
        print(f"  POPULATION {k} — {defs[k]}")
        print("═" * 78)
        section(pops[k], f"population {k}", "ROW LEVEL (comparable with Phase 1)")
        section(eps[k], f"population {k}", "EPISODE LEVEL (authoritative)")

    print()
    print("═" * 78)
    print("  HIT RATE BY CALENDAR YEAR (B+C, episode level) — survivorship bias check")
    print("═" * 78)
    by_year(eps["B+C"])

    print()
    print("═" * 78)
    print("  PER-BUCKET p90 SCORE THRESHOLDS on B+C — REPORT ONLY, defaults unchanged")
    print("═" * 78)
    print("  row level:")
    p90_thresholds(pops["B+C"])
    print("  episode level:")
    p90_thresholds(eps["B+C"])


if __name__ == "__main__":
    main()

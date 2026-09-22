#!/usr/bin/env python3
"""Does rvol_20 survive stratification, or is it another composition artifact?

WHY THIS EXISTS

The frozen v2 score passed a pooled out-of-sample test (p = 0.017) and then
turned out to be sorting on bucket membership: penny names hit at 35% against
market names' 9%, the top third held six times more penny names, and 97% of the
"separation" was that mix. The pooled test could not tell ranking from
composition.

`rvol_20` passed the same kind of pooled test. "Independently significant in B
and C" separates by TIME PERIOD, not by bucket, so it does not answer the
question. This script asks it directly.

METHOD

Every rvol split is computed WITHIN its stratum. Splitting globally and then
counting outcomes per stratum would reintroduce exactly the confound being
tested: a global high-rvol group that is disproportionately penny would look
predictive because penny names double more often.

Four tests:
  1. rvol top vs bottom tercile, WITHIN each bucket
  2. rvol top vs bottom tercile, WITHIN each atr_pct tercile
  3. Cochran-Mantel-Haenszel, stratified by bucket
  4. Cochran-Mantel-Haenszel, stratified by bucket x atr tercile

CMH is the right tool here: it pools evidence ACROSS strata while never
comparing across them, which is the precise failure the v2 result exhibited.

Effect sizes are reported next to every p-value. At ~7,000 episodes almost
anything clears p < 0.05, which is why all seven v2 components "resolved".
"""
from __future__ import annotations

import argparse
import csv
import math
from collections import defaultdict

PILOT_SPLIT_DATE = "2023-09-18"
THRESHOLD = 100.0


def norm_cdf(x: float) -> float:
    return 0.5 * (1 + math.erf(x / math.sqrt(2)))


def chi2_sf_1df(x: float) -> float:
    """Upper tail of chi-square with 1 df."""
    if x <= 0:
        return 1.0
    return math.erfc(math.sqrt(x / 2))


def two_proportion_z(h1, n1, h2, n2):
    if n1 == 0 or n2 == 0:
        return None
    p1, p2 = h1 / n1, h2 / n2
    pool = (h1 + h2) / (n1 + n2)
    se = math.sqrt(pool * (1 - pool) * (1 / n1 + 1 / n2))
    if se == 0:
        return None
    z = (p1 - p2) / se
    return z, 2 * (1 - norm_cdf(abs(z))), p1, p2


def terciles(values):
    """Return the two cut points splitting values into three equal-count parts."""
    s = sorted(values)
    n = len(s)
    if n < 3:
        return None
    return s[n // 3], s[2 * n // 3]


def fmt_p(p):
    if p != p:
        return "   n/a"
    return "<0.001" if p < 0.001 else f"{p:6.3f}"


def load(dump, pilot_path, lockbox_path, region=None):
    """Load candidates, excluding population A and the lockbox.

    `region` is (start, end) and excludes by DATE WINDOW rather than by a fixed
    row list. The row list was reserved under gate v1, and gate v2 changes which
    symbol-days pass, so the old list is neither a superset nor a subset of the
    candidates in its own window -- excluding it would leak new gate-v2
    candidates from inside the held-out window into the training data. See
    Phase 2 §4.3 and migration 018.
    """
    pilot = {l.strip() for l in open(pilot_path) if l.strip()}
    lockbox = set()
    if lockbox_path:
        with open(lockbox_path) as fh:
            for r in csv.reader(fh):
                if len(r) >= 2 and r[0] != "symbol":
                    lockbox.add((r[0], r[1]))

    rows = []
    for r in csv.DictReader(open(dump, newline="")):
        if region is not None:
            # Region lockbox: the window x every non-pilot symbol.
            if region[0] <= r["date"] <= region[1] and r["symbol"] not in pilot:
                continue
        elif (r["symbol"], r["date"]) in lockbox:
            continue
        if r["episode_start"] != "true":
            continue
        # Population A (pilot symbols inside the pilot window) is in-sample.
        if r["date"] >= PILOT_SPLIT_DATE and r["symbol"] in pilot:
            continue
        if not r.get("rvol_20") or not r.get("atr_pct"):
            continue
        rows.append({
            "symbol": r["symbol"], "date": r["date"], "bucket": r["bucket"],
            "rvol": float(r["rvol_20"]), "atr": float(r["atr_pct"]),
            "hit": float(r["fwd_max_gain_pct"]) >= THRESHOLD,
        })
    return rows, len(pilot), len(lockbox)


def tercile_contrast(rows, label, indent="    "):
    """rvol top vs bottom tercile, computed within THIS set of rows."""
    cuts = terciles([r["rvol"] for r in rows])
    if cuts is None or len(rows) < 30:
        print(f"{indent}{label:<28} n={len(rows):<6,} too few episodes to split")
        return None
    lo_cut, hi_cut = cuts
    lo = [r for r in rows if r["rvol"] <= lo_cut]
    hi = [r for r in rows if r["rvol"] > hi_cut]
    res = two_proportion_z(sum(r["hit"] for r in hi), len(hi),
                           sum(r["hit"] for r in lo), len(lo))
    if res is None:
        print(f"{indent}{label:<28} n={len(rows):<6,} undefined (no outcome variation)")
        return None
    z, p, ptop, pbot = res
    rr = (ptop / pbot) if pbot > 0 else float("inf")
    print(f"{indent}{label:<28} n={len(rows):<6,} top={100*ptop:6.2f}% bot={100*pbot:6.2f}% "
          f"{100*(ptop-pbot):+6.2f}pp  RR={rr:4.2f}  z={z:+5.2f}  p={fmt_p(p)}")
    return p, ptop - pbot


def cmh(strata, name):
    """Cochran-Mantel-Haenszel on a list of (a, b, c, d) 2x2 tables.

    a = high-rvol hits, b = high-rvol misses, c = low-rvol hits, d = low misses.
    Pools evidence across strata WITHOUT ever comparing across them, which is
    exactly the property the pooled v2 test lacked.
    """
    sum_a = sum_e = sum_v = 0.0
    num_or = den_or = 0.0
    used = 0
    for (a, b, c, d) in strata:
        n1, n2 = a + b, c + d
        m1, m2 = a + c, b + d
        N = n1 + n2
        if N < 2 or n1 == 0 or n2 == 0 or m1 == 0 or m2 == 0:
            continue
        used += 1
        sum_a += a
        sum_e += n1 * m1 / N
        sum_v += n1 * n2 * m1 * m2 / (N * N * (N - 1))
        num_or += a * d / N
        den_or += b * c / N
    if used == 0 or sum_v == 0:
        print(f"    {name}: no usable strata")
        return None
    stat = (abs(sum_a - sum_e) - 0.5) ** 2 / sum_v
    p = chi2_sf_1df(stat)
    or_mh = num_or / den_or if den_or > 0 else float("inf")
    print(f"    {name}")
    print(f"      strata used        {used}")
    print(f"      observed vs expected high-rvol hits   {sum_a:.0f} vs {sum_e:.1f}")
    print(f"      CMH chi-square(1)  {stat:.2f}      p = {fmt_p(p)}")
    print(f"      MH common odds ratio                  {or_mh:.3f}")
    return p, or_mh


def median_split_table(rows):
    """2x2 counts for a median rvol split computed within these rows."""
    if len(rows) < 4:
        return None
    vals = sorted(r["rvol"] for r in rows)
    med = vals[len(vals) // 2] if len(vals) % 2 else (vals[len(vals)//2 - 1] + vals[len(vals)//2]) / 2
    hi = [r for r in rows if r["rvol"] > med]
    lo = [r for r in rows if r["rvol"] <= med]
    if not hi or not lo:
        return None
    a = sum(r["hit"] for r in hi); b = len(hi) - a
    c = sum(r["hit"] for r in lo); d = len(lo) - c
    return a, b, c, d


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dump", required=True)
    ap.add_argument("--pilot", required=True)
    ap.add_argument("--lockbox", default=None)
    ap.add_argument("--lockbox-region", default=None,
                    help="START:END — exclude by region instead of a row list")
    args = ap.parse_args()

    region = tuple(args.lockbox_region.split(":")) if args.lockbox_region else None
    rows, n_pilot, n_lock = load(args.dump, args.pilot, args.lockbox, region)

    print("=" * 78)
    print("  rvol_20 STRATIFICATION CHECK — is it signal, or another composition artifact?")
    print("=" * 78)
    print(f"  population        B+C, episode level, lockbox excluded, population A excluded")
    print(f"  pilot symbols     {n_pilot} (excluded inside the pilot window)")
    if region:
        print(f"  lockbox           REGION {region[0]} .. {region[1]} x non-pilot symbols (excluded)")
    else:
        print(f"  lockbox rows      {n_lock} (excluded)")
    print(f"  episodes          {len(rows):,}")
    print(f"  base rate         {100*sum(r['hit'] for r in rows)/len(rows):.2f}%")
    print()
    print("  Every rvol split below is computed WITHIN its stratum. A global split")
    print("  would reintroduce the confound being tested.")

    # ── reference: the pooled result being challenged ──
    print()
    print("  ── 0. POOLED (the test that failed v2; shown for comparison only) ──")
    tercile_contrast(rows, "rvol, pooled")

    # ── 1. within bucket ──
    print()
    print("  ── 1. WITHIN EACH BUCKET ──")
    by_bucket = defaultdict(list)
    for r in rows:
        by_bucket[r["bucket"]].append(r)
    bucket_results = {}
    for b in sorted(by_bucket, key=lambda k: -len(by_bucket[k])):
        sub = by_bucket[b]
        base = 100 * sum(r["hit"] for r in sub) / len(sub)
        print(f"    [{b}] base rate {base:.2f}%")
        bucket_results[b] = tercile_contrast(sub, f"rvol within {b}", indent="      ")

    # ── 2. within atr terciles ──
    print()
    print("  ── 2. WITHIN atr_pct TERCILES (volatility held roughly constant) ──")
    acuts = terciles([r["atr"] for r in rows])
    alo, ahi = acuts
    print(f"    atr_pct tercile cuts: {alo:.3f} / {ahi:.3f}")
    atr_bands = {
        "atr low":  [r for r in rows if r["atr"] <= alo],
        "atr mid":  [r for r in rows if alo < r["atr"] <= ahi],
        "atr high": [r for r in rows if r["atr"] > ahi],
    }
    for name, sub in atr_bands.items():
        base = 100 * sum(r["hit"] for r in sub) / len(sub) if sub else float("nan")
        print(f"    [{name}] base rate {base:.2f}%")
        tercile_contrast(sub, f"rvol within {name}", indent="      ")

    # ── 3. CMH by bucket ──
    print()
    print("  ── 3. COCHRAN-MANTEL-HAENSZEL, stratified by bucket ──")
    tables = [t for t in (median_split_table(by_bucket[b]) for b in by_bucket) if t]
    cmh(tables, "CMH | bucket")

    # ── 4. CMH by bucket x atr tercile ──
    print()
    print("  ── 4. COCHRAN-MANTEL-HAENSZEL, stratified by bucket x atr tercile ──")
    cells = defaultdict(list)
    for r in rows:
        band = "low" if r["atr"] <= alo else ("mid" if r["atr"] <= ahi else "high")
        cells[(r["bucket"], band)].append(r)
    print(f"    {'stratum':<22} {'n':>7} {'hi-rvol hit%':>13} {'lo-rvol hit%':>13}")
    tables2 = []
    for k in sorted(cells, key=lambda k: (k[0], k[1])):
        sub = cells[k]
        t = median_split_table(sub)
        if not t:
            print(f"    {k[0]+' / atr '+k[1]:<22} {len(sub):>7,}   (too few to split)")
            continue
        a, b, c, d = t
        tables2.append(t)
        print(f"    {k[0]+' / atr '+k[1]:<22} {len(sub):>7,} {100*a/(a+b):12.2f}% {100*c/(c+d):12.2f}%")
    print()
    cmh(tables2, "CMH | bucket x atr tercile")

    # ── verdict ──
    print()
    print("=" * 78)
    print("  VERDICT")
    print("=" * 78)
    for b, res in bucket_results.items():
        if res is None:
            print(f"    {b:<8} inconclusive (too few episodes)")
        else:
            p, d = res
            verdict = "separates" if p < 0.05 and d > 0 else "does NOT separate"
            print(f"    {b:<8} {verdict:<20} ({100*d:+.2f}pp, p={fmt_p(p)})")


if __name__ == "__main__":
    main()

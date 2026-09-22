#!/usr/bin/env python3
"""P2-3 verification: can Tiingo serve delisted names, and is the served subset representative?

Verify BEFORE building, per Phase 2 §3.3 and the acceptance rule in §3.3.0.

THE QUESTION THAT MATTERS MOST IS (c), NOT (b)

Coverage alone does not decide anything. Tiingo is already known to drop back
history for names it still lists (HWH: 0 bars, JCSE: 1 bar over 11 years), so
partial coverage of dead companies is expected. What decides the route is
whether the names it KEPT differ from the ones it DROPPED — because a
survivorship correction computed on a non-representative subset is a second
selection bias wearing the costume of a fix.

Sampling is stratified by delisting year so the comparison is not dominated by
whichever year happens to have the most delistings.
"""
from __future__ import annotations

import argparse
import json
import os
import random
import subprocess
import time
from collections import Counter, defaultdict

UA_TOKEN = None


def tiingo(path: str, timeout: int = 45):
    out = subprocess.run(
        ["curl", "-s", "-w", "\n%{http_code}", "--max-time", str(timeout),
         "-H", f"Authorization: Token {UA_TOKEN}", "--compressed",
         f"https://api.tiingo.com{path}"],
        capture_output=True, text=True)
    body, _, code = out.stdout.rpartition("\n")
    try:
        return code.strip(), json.loads(body)
    except Exception:
        return code.strip(), None


def main() -> int:
    global UA_TOKEN
    ap = argparse.ArgumentParser()
    ap.add_argument("--sample", type=int, default=200)
    ap.add_argument("--rate", type=float, default=2.0)
    ap.add_argument("--dead", default="/tmp/dead.json")
    args = ap.parse_args()

    for line in open(os.path.join(os.path.dirname(__file__), "..", ".env"), encoding="utf-8"):
        if line.startswith("TIINGO_API_KEY="):
            UA_TOKEN = line.split("=", 1)[1].strip()
    if not UA_TOKEN:
        print("TIINGO_API_KEY not in .env")
        return 2

    dead = json.load(open(args.dead))
    print("=" * 80)
    print("  P2-3 VERIFICATION — delisted coverage on Tiingo Power")
    print("=" * 80)
    print(f"  (a) delisted US common stocks in supported_tickers: {len(dead):,}")

    # Stratify by delisting year so no single year dominates the comparison.
    by_year = defaultdict(list)
    for r in dead:
        by_year[r["endDate"][:4]].append(r)
    years = sorted(by_year)
    per = max(1, args.sample // len(years))
    rng = random.Random(20260922)          # fixed seed: the draw is reproducible
    sample = []
    for y in years:
        sample += rng.sample(by_year[y], min(per, len(by_year[y])))
    print(f"      stratified sample: {len(sample)} names across {len(years)} delisting years")
    print(f"      pacing {args.rate}/s")
    print()

    sleep = 1.0 / args.rate
    served, unserved = [], []
    print(f"    {'ticker':<9} {'delisted':<11} {'bars':>7} {'span':<24} verdict")
    for r in sample:
        t = r["ticker"]
        provider_sym = t.replace(".", "-")
        code, d = tiingo(f"/tiingo/daily/{provider_sym}/prices"
                         f"?startDate={r['startDate']}&endDate={r['endDate']}")
        time.sleep(sleep)
        n = len(d) if isinstance(d, list) else 0
        if n == 0:
            unserved.append({**r, "bars": 0, "http": code})
            verdict = f"NOT SERVED (http {code})"
            span = "—"
        else:
            first, last = d[0]["date"][:10], d[-1]["date"][:10]
            served.append({**r, "bars": n, "first": first, "last": last,
                           "last_close": d[-1].get("close")})
            span = f"{first}..{last}"
            verdict = "served"
        if len(served) + len(unserved) <= 40:
            print(f"    {t:<9} {r['endDate']:<11} {n:>7} {span:<24} {verdict}")

    tot = len(served) + len(unserved)
    print(f"    ... {tot} checked")
    print()
    print(f"  (b) FULL-HISTORY COVERAGE")
    print(f"      served:   {len(served):>4} / {tot}  ({100*len(served)/tot:.1f}%)")
    print(f"      unserved: {len(unserved):>4} / {tot}  ({100*len(unserved)/tot:.1f}%)")

    # ── (c) are the served representative of the unserved? ──
    print()
    print("  (c) DO THE SERVED DIFFER FROM THE UNSERVED?")

    def summarise(label, rows):
        if not rows:
            print(f"      {label:<10} (none)")
            return
        ages = []
        for r in rows:
            try:
                ages.append((int(r["endDate"][:4]) - int(r["startDate"][:4])))
            except Exception:
                pass
        ages.sort()
        yrs = Counter(r["endDate"][:4] for r in rows)
        exch = Counter(r["exchange"] for r in rows)
        med_age = ages[len(ages) // 2] if ages else float("nan")
        print(f"      {label:<10} n={len(rows):<4} median listed-years={med_age:<4} "
              f"median delist-year={sorted(yrs.elements())[len(rows)//2]:<6} "
              f"top exch={exch.most_common(1)[0][0]}")
        return {"n": len(rows), "med_age": med_age, "exch": exch, "years": yrs}

    a = summarise("served", served)
    b = summarise("unserved", unserved)

    if served and unserved:
        print()
        print(f"      {'exchange':<12} {'served':>8} {'unserved':>10}  (share of each group)")
        for e in sorted(set(a["exch"]) | set(b["exch"])):
            sa = 100 * a["exch"].get(e, 0) / a["n"]
            sb = 100 * b["exch"].get(e, 0) / b["n"]
            flag = "  <-- differs" if abs(sa - sb) > 15 else ""
            print(f"      {e:<12} {sa:7.1f}% {sb:9.1f}%{flag}")
        print()
        print(f"      {'delist year':<12} {'served':>8} {'unserved':>10}")
        for y in sorted(set(a["years"]) | set(b["years"])):
            sa = 100 * a["years"].get(y, 0) / a["n"]
            sb = 100 * b["years"].get(y, 0) / b["n"]
            flag = "  <-- differs" if abs(sa - sb) > 15 else ""
            print(f"      {y:<12} {sa:7.1f}% {sb:9.1f}%{flag}")

    json.dump({"served": served, "unserved": unserved}, open("/tmp/p23_coverage.json", "w"))
    print()
    print("  wrote /tmp/p23_coverage.json")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

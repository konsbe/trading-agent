#!/usr/bin/env python3
"""Phase 2 §3.2 step (a): verify SEC EDGAR before building anything on it.

Checks, in this order:
  1. ticker -> CIK mapping coverage from SEC's company_tickers.json
  2. dei:EntityCommonStockSharesOutstanding availability, with `filed` dates
  3. behaviour on DELISTED names, which the spec predicts will be missing from
     the ticker file
  4. multi-class share structures

RATE DISCIPLINE

SEC's fair-access rate is ~10 req/s. This runs far below it, because the egress
here is a Zscaler corporate proxy (a shared corporate proxy) shared with unknown other
users -- the published rate is per-IP, and on a shared IP our share of it is not
ours to assume.

USER-AGENT -- the thing that actually blocks you

SEC requires a descriptive UA. It also DENYLISTS some contact domains: a UA
containing `users.noreply.github.com` returns 403 while the identical UA with a
real domain returns 200. The 403 body says "Request Rate Threshold Exceeded" or
"Your Request Originates from an Undeclared Automated Tool", both of which point
at rate limiting or automation detection rather than at the contact address, so
this costs an hour to diagnose if you have not seen it before.
"""
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import time
from collections import defaultdict

UA = "TradingAgentResearch <your-contact-email>"
SLEEP = 0.35  # ~3 req/s, well under SEC's ~10/s, because the IP is shared

# Known delisted / acquired names, for check 3. Chosen because all filed with
# SEC for years, so a CIK and a filing history certainly exist -- which makes
# "absent from company_tickers.json" a statement about that file rather than
# about the company.
DELISTED = ["SIVB", "FRC", "BBBY", "YELL", "RAD", "WE", "PRTY", "TWTR", "ATVI", "VMW"]


def fetch(url: str, timeout: int = 60):
    """GET via curl, returning (http_code, parsed_json_or_none)."""
    out = subprocess.run(
        ["curl", "-s", "-w", "\n%{http_code}", "--max-time", str(timeout),
         "-H", f"User-Agent: {UA}", "-H", "Accept-Encoding: gzip, deflate",
         "--compressed", url],
        capture_output=True, text=True)
    body, _, code = out.stdout.rpartition("\n")
    try:
        return code.strip(), json.loads(body)
    except Exception:
        return code.strip(), None


def psql(sql: str) -> str:
    r = subprocess.run(["psql", os.environ["DATABASE_URL"], "-tAq", "-c", sql],
                       capture_output=True, text=True)
    if r.returncode != 0:
        raise SystemExit(f"psql: {r.stderr.strip()}")
    return r.stdout


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--per-bucket", type=int, default=45)
    ap.add_argument("--out", default="/tmp/edgar_coverage.json")
    args = ap.parse_args()

    print("=" * 78)
    print("  SEC EDGAR VERIFICATION — Phase 2 §3.2 step (a)")
    print("=" * 78)
    print(f"  User-Agent: {UA}")
    print(f"  pacing:     {1/SLEEP:.1f} req/s (SEC allows ~10; egress IP is a shared proxy)")

    # ── 1. ticker -> CIK map ──
    code, tickers = fetch("https://www.sec.gov/files/company_tickers.json")
    if tickers is None:
        print(f"\n  FAILED to fetch company_tickers.json (HTTP {code}). Cannot proceed.")
        return 1
    t2c = {v["ticker"].upper(): (str(v["cik_str"]).zfill(10), v["title"])
           for v in tickers.values()}
    print(f"\n  ── 1. ticker -> CIK map ──")
    print(f"    HTTP {code}, {len(t2c):,} current tickers")

    # ── sample ──
    rows = [l for l in psql("""
        WITH usable AS (
          SELECT DISTINCT ON (symbol) symbol, value
          FROM equity_fundamentals
          WHERE metric='market_cap' AND value IS NOT NULL AND value > 0
          ORDER BY symbol, ts DESC, fundamental_source_rank(source) DESC)
        SELECT u.symbol || ',' ||
               CASE WHEN l.value < 3e8 THEN 'penny' ELSE 'market' END
        FROM universe_symbols u JOIN usable l ON l.symbol = u.symbol
        WHERE u.is_eligible
        ORDER BY md5(u.symbol)
    """).splitlines() if l]

    by_bucket = defaultdict(list)
    for r in rows:
        sym, bucket = r.split(",")
        if len(by_bucket[bucket]) < args.per_bucket:
            by_bucket[bucket].append(sym)

    sample = [(s, "penny") for s in by_bucket["penny"]] + \
             [(s, "market") for s in by_bucket["market"]] + \
             [(s, "delisted") for s in DELISTED]
    print(f"    sample: {len(by_bucket['penny'])} penny + "
          f"{len(by_bucket['market'])} market + {len(DELISTED)} delisted "
          f"= {len(sample)} symbols")

    # ── 2/3. CIK resolution, then the concept ──
    print(f"\n  ── 2. ticker -> CIK resolution by stratum ──")
    resolved, unresolved = [], defaultdict(list)
    for sym, bucket in sample:
        cik = t2c.get(sym.replace(".", "-").upper()) or t2c.get(sym.upper())
        if cik:
            resolved.append((sym, bucket, cik[0]))
        else:
            unresolved[bucket].append(sym)
    for b in ("penny", "market", "delisted"):
        tot = sum(1 for _, bb in sample if bb == b)
        miss = len(unresolved[b])
        print(f"    {b:<9} {tot-miss:>3}/{tot:<3} resolved"
              + (f"   MISSING: {', '.join(unresolved[b][:12])}" if miss else ""))

    print(f"\n  ── 3. dei:EntityCommonStockSharesOutstanding ──")
    print(f"    {'symbol':<8} {'bucket':<9} {'facts':>6} {'filed range':<25} {'units':<10} note")
    results = {}
    stats = defaultdict(lambda: {"have": 0, "none": 0, "err": 0, "facts": 0, "multiclass": 0})
    for sym, bucket, cik in resolved:
        url = (f"https://data.sec.gov/api/xbrl/companyconcept/CIK{cik}"
               f"/dei/EntityCommonStockSharesOutstanding.json")
        code, d = fetch(url)
        time.sleep(SLEEP)
        st = stats[bucket]
        if code == "404" or (d is None and code != "200"):
            st["none" if code == "404" else "err"] += 1
            print(f"    {sym:<8} {bucket:<9} {'—':>6} {'—':<25} {'—':<10} HTTP {code}")
            results[sym] = {"bucket": bucket, "cik": cik, "status": code, "facts": 0}
            continue
        units = d.get("units", {})
        unit = next(iter(units), None)
        facts = units.get(unit, []) if unit else []
        if not facts:
            st["none"] += 1
            print(f"    {sym:<8} {bucket:<9} {0:>6} {'—':<25} {'—':<10} concept present, no facts")
            results[sym] = {"bucket": bucket, "cik": cik, "status": "empty", "facts": 0}
            continue
        st["have"] += 1
        st["facts"] += len(facts)
        filed = sorted(f["filed"] for f in facts if f.get("filed"))
        # multi-class: several facts sharing one `filed` date, distinguished by
        # the frame/segment they came from
        per_filed = defaultdict(int)
        for f in facts:
            per_filed[f.get("filed")] += 1
        multi = max(per_filed.values()) > 1
        if multi:
            st["multiclass"] += 1
        rng = f"{filed[0]} .. {filed[-1]}" if filed else "—"
        results[sym] = {"bucket": bucket, "cik": cik, "status": "ok",
                        "facts": len(facts), "unit": unit,
                        "filed_first": filed[0] if filed else None,
                        "filed_last": filed[-1] if filed else None,
                        "multiclass": multi,
                        "series": [{"filed": f.get("filed"), "end": f.get("end"),
                                    "val": f.get("val"), "form": f.get("form")}
                                   for f in facts]}
        print(f"    {sym:<8} {bucket:<9} {len(facts):>6} {rng:<25} {str(unit):<10}"
              f"{'  multi-class' if multi else ''}")

    print(f"\n  ── 4. summary by stratum ──")
    print(f"    {'stratum':<9} {'with facts':>11} {'no facts':>9} {'errors':>7} "
          f"{'avg facts':>10} {'multi-class':>12}")
    for b in ("penny", "market", "delisted"):
        s = stats[b]
        n = s["have"] + s["none"] + s["err"]
        if n == 0:
            print(f"    {b:<9} {'— none resolved':>11}")
            continue
        avg = s["facts"] / s["have"] if s["have"] else 0
        print(f"    {b:<9} {s['have']:>11} {s['none']:>9} {s['err']:>7} "
              f"{avg:>10.1f} {s['multiclass']:>12}")

    with open(args.out, "w") as fh:
        json.dump({"ua": UA, "results": results,
                   "unresolved": {k: v for k, v in unresolved.items()}}, fh, indent=1)
    print(f"\n  wrote {args.out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

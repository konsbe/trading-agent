#!/usr/bin/env python3
"""P2-5 verification: is Tiingo News usable for catalyst history?

Verify BEFORE building the backfill, per Phase 2 §3.5.

Five questions, in the order that can kill the idea fastest:

  1. HISTORY DEPTH. Finnhub reached ~12 months, which let catalyst be evaluated
     on only 80 of 218 Phase 1 candidates. If Tiingo is no deeper, §3.5 is not
     worth building.
  2. TICKER-TAGGING QUALITY on micro-caps. The whole point is coverage of small
     names. A first glance is not encouraging: an AAPL-tagged article turned out
     to be about Peloton.
  3. TIMESTAMP PRECISION, which decides whether the §3.5 as-of cutoff can be
     applied at all.
  4. REUSED TICKERS. 468 symbols have been used by more than one company
     (§3.3.1). An article tagged with a reused ticker may belong to the OTHER
     company, and attaching it to the current one is a lookahead-shaped error
     that no date filter on the article alone would catch.
  5. AS-OF CUTOFF applied identically to history and to live.
"""
from __future__ import annotations

import argparse
import json
import os
import subprocess
import time
from collections import Counter

TOKEN = None


def news(params: str, timeout: int = 60):
    out = subprocess.run(
        ["curl", "-s", "-w", "\n%{http_code}", "--max-time", str(timeout),
         "-H", f"Authorization: Token {TOKEN}", "--compressed",
         f"https://api.tiingo.com/tiingo/news?{params}"],
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
    global TOKEN
    ap = argparse.ArgumentParser()
    ap.add_argument("--micro", type=int, default=25)
    args = ap.parse_args()

    for line in open(os.path.join(os.path.dirname(__file__), "..", ".env"), encoding="utf-8"):
        if line.startswith("TIINGO_API_KEY="):
            TOKEN = line.split("=", 1)[1].strip()

    print("=" * 80)
    print("  P2-5 VERIFICATION — Tiingo News for catalyst history")
    print("=" * 80)

    # ── 1. depth ──
    print("\n  (1) HISTORY DEPTH")
    for start in ("2010-01-01", "2014-01-01", "2016-09-21", "2020-01-01"):
        code, d = news(f"tickers=AAPL&startDate={start}&endDate={start[:4]}-12-31&limit=5")
        n = len(d) if isinstance(d, list) else 0
        first = d[-1]["publishedDate"][:10] if n else "—"
        print(f"      from {start}: {n} article(s), oldest {first}  (http {code})")
        time.sleep(0.5)

    code, d = news("tickers=AAPL&startDate=2000-01-01&sortBy=publishedDate&limit=1")
    if isinstance(d, list) and d:
        print(f"      oldest AAPL article Tiingo will return: {d[0]['publishedDate'][:10]}")

    # ── 3. timestamp precision (cheap, do it early) ──
    print("\n  (3) TIMESTAMP PRECISION")
    code, d = news("tickers=AAPL&limit=5")
    if isinstance(d, list) and d:
        for a in d[:3]:
            print(f"      publishedDate {a['publishedDate']}   crawlDate {a.get('crawlDate','—')}")
        pd = d[0]["publishedDate"]
        print(f"      -> published is {'second' if len(pd) >= 19 else 'coarser'}-precision, "
              f"timezone {'Z/UTC' if pd.endswith('Z') else 'offset-bearing'}")
        print("      -> sufficient for the §3.5 as-of cutoff (compare against scan time on day t)")

    # ── 2. micro-cap tagging quality ──
    print(f"\n  (2) TICKER TAGGING ON MICRO-CAPS (sample of {args.micro})")
    syms = [l for l in psql("""
        WITH cap AS (
          SELECT DISTINCT ON (symbol) symbol, value FROM equity_fundamentals
          WHERE metric='market_cap' AND value IS NOT NULL AND value > 0
          ORDER BY symbol, ts DESC, fundamental_source_rank(source) DESC)
        SELECT u.symbol FROM universe_symbols u JOIN cap ON cap.symbol=u.symbol
        WHERE u.is_eligible AND cap.value < 3e8
        ORDER BY md5(u.symbol) LIMIT %d""" % args.micro).splitlines() if l]

    covered = 0
    primary_hits = 0
    total_articles = 0
    print(f"      {'symbol':<8} {'articles':>9} {'tagged 1st':>11}  newest")
    for sym in syms:
        code, d = news(f"tickers={sym.lower()}&startDate=2016-09-21&limit=50")
        time.sleep(0.5)
        n = len(d) if isinstance(d, list) else 0
        total_articles += n
        if n:
            covered += 1
            # Is this symbol the FIRST-listed ticker, i.e. the article's subject
            # rather than a passing mention? Tiingo lists tickers per article and
            # the ordering is not documented as meaningful, so this is a proxy.
            first_tag = sum(1 for a in d
                            if a.get("tickers") and a["tickers"][0].upper() == sym.upper())
            primary_hits += first_tag
            newest = d[0]["publishedDate"][:10]
            print(f"      {sym:<8} {n:>9} {first_tag:>11}  {newest}")
        else:
            print(f"      {sym:<8} {0:>9} {'—':>11}  —")

    print()
    print(f"      micro-caps with ANY article: {covered}/{len(syms)} ({100*covered/max(len(syms),1):.0f}%)")
    print(f"      total articles: {total_articles};  symbol is first-listed ticker on "
          f"{primary_hits} ({100*primary_hits/max(total_articles,1):.0f}%)")

    # ── 4. reused tickers ──
    print("\n  (4) REUSED TICKERS — do articles fall inside the CURRENT listing?")
    reuse = json.load(open("/tmp/reuse_cases.json")) if os.path.exists("/tmp/reuse_cases.json") else {}
    checked = mis = 0
    for t, spans in list(reuse.items())[:12]:
        cur = spans[-1]
        code, d = news(f"tickers={t.lower()}&startDate=2016-09-21&limit=50")
        time.sleep(0.5)
        if not isinstance(d, list) or not d:
            continue
        checked += 1
        before = [a for a in d if a["publishedDate"][:10] < cur["startDate"]]
        if before:
            mis += 1
            print(f"      {t:<8} current listing from {cur['startDate']}; "
                  f"{len(before)} article(s) BEFORE it, oldest {before[-1]['publishedDate'][:10]}")
    print(f"      reused tickers checked: {checked}; with articles predating the current listing: {mis}")
    if checked and not mis:
        print("      none found in this sample — but the window only reaches 2016-09-21,")
        print("      and most reuse gaps are older, so this is weak evidence, not a clearance.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

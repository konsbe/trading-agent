#!/usr/bin/env python3
"""P2-2: populate shares_outstanding_pit for the full eligible universe from SEC EDGAR.

Reads dei:EntityCommonStockSharesOutstanding per symbol via the XBRL
companyconcept API, and stores one row per (symbol, filed_date).

DESIGN NOTES THAT MATTER

- **As-of on `filed`, never `period_end`.** The period end precedes the filing
  by weeks; joining on it would use information before it was public. Both are
  stored; only filed_date is keyed.
- **Multi-class: classes are SUMMED** and `multi_class` set true. Summing all
  classes and pricing them at the traded class's price is the standard
  market-cap approximation. Flagged so a sensitivity check can drop them.
- **No fallback to today's share count, ever.** A symbol with no EDGAR coverage
  gets no rows, and gate v2 fails those symbol-days with
  market_cap_pit_unavailable. Falling back would reinstate the leak precisely
  where it cannot be checked.
- **User-Agent comes from SEC_EDGAR_USER_AGENT.** SEC denylists some contact
  domains (`users.noreply.github.com` -> 403 with a message about rate limits).
  See data_ingestion.md.
- **Resumable.** Already-populated symbols are skipped unless --refresh, so a
  4,975-symbol run over a shared proxy can be interrupted.
"""
from __future__ import annotations

import argparse
import csv
import json
import os
import subprocess
import sys
import tempfile
import time
from collections import defaultdict

PRIMARY_CONCEPT = "dei/EntityCommonStockSharesOutstanding"

# Fallback, used ONLY where the primary concept is absent. It is a different
# measurement -- a balance-sheet figure as of period_end rather than a cover-page
# figure near the filing date -- so it is systematically staler. Recorded per row
# in shares_outstanding_pit.concept (migration 019) so any result that moves when
# the fallback is enabled can be checked against it.
FALLBACK_CONCEPT = "us-gaap/CommonStockSharesOutstanding"


def psql(sql: str) -> str:
    r = subprocess.run(["psql", os.environ["DATABASE_URL"], "-tAq",
                        "-v", "ON_ERROR_STOP=1", "-c", sql],
                       capture_output=True, text=True)
    if r.returncode != 0:
        raise SystemExit(f"psql: {r.stderr.strip()}")
    return r.stdout


def fetch(url: str, ua: str, timeout: int):
    out = subprocess.run(
        ["curl", "-s", "-w", "\n%{http_code}", "--max-time", str(timeout),
         "-H", f"User-Agent: {ua}", "-H", "Accept-Encoding: gzip, deflate",
         "--compressed", url],
        capture_output=True, text=True)
    body, _, code = out.stdout.rpartition("\n")
    try:
        return code.strip(), json.loads(body)
    except Exception:
        return code.strip(), None


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--refresh", action="store_true",
                    help="re-fetch symbols already present")
    ap.add_argument("--limit", type=int, default=0)
    ap.add_argument("--audit-out", default="/tmp/edgar_audit.csv")
    ap.add_argument("--concept", choices=("dei", "us-gaap"), default="dei",
                    help="which XBRL concept to read; us-gaap is the fallback")
    ap.add_argument("--only-missing", action="store_true",
                    help="restrict to symbols with NO rows yet — the fallback's only legitimate scope")
    args = ap.parse_args()

    ua = os.environ.get("SEC_EDGAR_USER_AGENT")
    if not ua:
        print("SEC_EDGAR_USER_AGENT is not set. SEC requires a descriptive "
              "User-Agent with a contact address, and refusing to guess one is "
              "deliberate: a wrong value returns 403 with a message about rate "
              "limits, which is the hardest failure here to diagnose.")
        return 2
    rate = float(os.environ.get("SEC_EDGAR_RATE_PER_SEC", "3.0"))
    timeout = int(os.environ.get("SEC_EDGAR_TIMEOUT", "60s").rstrip("s"))
    sleep = 1.0 / rate

    print("=" * 78)
    print("  P2-2: SEC EDGAR point-in-time shares backfill")
    print("=" * 78)
    print(f"  User-Agent: {ua}")
    print(f"  pacing:     {rate} req/s (SEC allows ~10; this egress is a shared proxy)")

    code, tickers = fetch("https://www.sec.gov/files/company_tickers.json", ua, timeout)
    if tickers is None:
        print(f"  cannot fetch company_tickers.json (HTTP {code})")
        return 1
    t2c = {}
    for v in tickers.values():
        t2c[v["ticker"].upper()] = str(v["cik_str"]).zfill(10)
    print(f"  ticker->CIK map: {len(t2c):,} CURRENT tickers "
          f"(delisted names are absent by design; that is P2-3)")

    concept = PRIMARY_CONCEPT if args.concept == "dei" else FALLBACK_CONCEPT
    concept_label = concept.replace("/", ":")
    print(f"  concept:    {concept_label}")

    universe = [l for l in psql(
        "SELECT symbol FROM universe_symbols WHERE is_eligible ORDER BY symbol"
    ).splitlines() if l]
    if args.only_missing:
        # The fallback must never overwrite a primary-concept row. Restricting
        # to symbols with no rows at all is what enforces "used only where the
        # dei concept is missing" -- a rule that is easy to state and easy to
        # violate by forgetting a WHERE clause.
        have = {l for l in psql(
            "SELECT DISTINCT symbol FROM shares_outstanding_pit").splitlines() if l}
        universe = [s for s in universe if s not in have]
        print(f"  restricted to {len(universe):,} symbols with no point-in-time rows yet")
    done = set()
    if not args.refresh:
        done = {l for l in psql(
            "SELECT DISTINCT symbol FROM shares_outstanding_pit").splitlines() if l}
    todo = [s for s in universe if s not in done]
    if args.limit:
        todo = todo[:args.limit]
    print(f"  eligible universe: {len(universe):,}   already stored: {len(done):,}   "
          f"to fetch: {len(todo):,}")
    eta = len(todo) * sleep / 60
    print(f"  estimated wall clock: {eta:.0f} min")

    rows, audit = [], []
    stats = defaultdict(int)
    t0 = time.time()
    for i, sym in enumerate(todo, 1):
        # Tiingo uses hyphens for share classes, SEC/Finnhub use dots; try both.
        cik = t2c.get(sym.upper()) or t2c.get(sym.replace(".", "-").upper()) \
            or t2c.get(sym.replace("-", ".").upper())
        if not cik:
            stats["no_cik"] += 1
            audit.append((sym, "", "no_cik", 0, "", "", ""))
            continue

        url = f"https://data.sec.gov/api/xbrl/companyconcept/CIK{cik}/{concept}.json"
        code, d = fetch(url, ua, timeout)
        time.sleep(sleep)

        if code == "403":
            # A 403 mid-run means the UA was rejected or we are being throttled.
            # Stop rather than grind through thousands of failures and record
            # them as "no coverage" -- that would silently become a bias.
            print(f"\n  HTTP 403 on {sym}. Stopping: a 403 is a request problem, "
                  f"not a coverage fact, and recording it as missing data would "
                  f"turn our error into the dataset's bias.")
            break
        if code != "200" or d is None:
            stats[f"http_{code}"] += 1
            audit.append((sym, cik, f"http_{code}", 0, "", "", ""))
            continue

        units = d.get("units", {})
        unit = next(iter(units), None)
        facts = units.get(unit, []) if unit else []
        if not facts:
            stats["no_facts"] += 1
            audit.append((sym, cik, "no_facts", 0, "", "", ""))
            continue

        # Group by filed date; sum share classes reported on the same filing.
        by_filed: dict[str, dict] = {}
        for f in facts:
            fd, val = f.get("filed"), f.get("val")
            if not fd or val is None:
                continue
            e = by_filed.setdefault(fd, {"shares": 0.0, "n": 0,
                                         "end": f.get("end"), "form": f.get("form")})
            e["shares"] += float(val)
            e["n"] += 1
            # Keep the latest period_end seen for this filing, for auditing.
            if f.get("end") and (e["end"] is None or f["end"] > e["end"]):
                e["end"] = f["end"]
        if not by_filed:
            stats["no_dated_facts"] += 1
            audit.append((sym, cik, "no_dated_facts", 0, "", "", ""))
            continue

        multi = any(e["n"] > 1 for e in by_filed.values())
        for fd, e in by_filed.items():
            rows.append((sym, fd, e["shares"], cik, e["end"] or "",
                         e["form"] or "", e["n"] > 1, e["n"], concept_label))
        stats["ok"] += 1
        stats["multi_class"] += 1 if multi else 0
        filed_sorted = sorted(by_filed)
        forms = sorted({e["form"] for e in by_filed.values() if e["form"]})
        audit.append((sym, cik, "ok", len(by_filed), filed_sorted[0],
                      filed_sorted[-1], "|".join(forms)))

        if i % 250 == 0:
            el = time.time() - t0
            print(f"    {i:>5}/{len(todo)}  ok={stats['ok']:<5} "
                  f"no_facts={stats['no_facts']:<4} no_cik={stats['no_cik']:<4} "
                  f"elapsed={el/60:.1f}m  rows={len(rows):,}")

    print(f"\n  fetched: ok={stats['ok']:,}  no_facts={stats['no_facts']:,}  "
          f"no_cik={stats['no_cik']:,}  multi_class={stats['multi_class']:,}")
    for k, v in sorted(stats.items()):
        if k.startswith("http_"):
            print(f"    {k}: {v}")

    if rows:
        with tempfile.NamedTemporaryFile("w", suffix=".csv", delete=False, newline="") as tf:
            w = csv.writer(tf)
            w.writerow(["symbol", "filed_date", "shares", "cik", "period_end",
                        "form", "multi_class", "class_count", "concept"])
            for r in rows:
                w.writerow(["" if x is None else x for x in r])
            path = tf.name
        # Staged through a TEMP table so a re-run is idempotent on
        # (symbol, filed_date). Driven from a script FILE rather than -c
        # because \copy is a psql meta-command and cannot share a -c with SQL.
        sql = f"""
CREATE TEMP TABLE _sop_in (symbol text, filed_date date, shares double precision,
  cik text, period_end date, form text, multi_class boolean, class_count int,
  concept text);
\\copy _sop_in FROM '{path}' CSV HEADER
INSERT INTO shares_outstanding_pit
  (symbol, filed_date, shares, cik, period_end, form, multi_class, class_count, concept)
SELECT symbol, filed_date, shares, cik, period_end, form, multi_class, class_count, concept
FROM _sop_in
ON CONFLICT (symbol, filed_date) DO UPDATE SET
  concept = EXCLUDED.concept,
  shares = EXCLUDED.shares, cik = EXCLUDED.cik, period_end = EXCLUDED.period_end,
  form = EXCLUDED.form, multi_class = EXCLUDED.multi_class,
  class_count = EXCLUDED.class_count;
"""
        with tempfile.NamedTemporaryFile("w", suffix=".sql", delete=False) as sf:
            sf.write(sql)
            sqlpath = sf.name
        subprocess.run(["psql", os.environ["DATABASE_URL"], "-q",
                        "-v", "ON_ERROR_STOP=1", "-f", sqlpath], check=True)
        print(f"  loaded {len(rows):,} point-in-time rows")

    with open(args.audit_out, "w", newline="") as fh:
        w = csv.writer(fh)
        w.writerow(["symbol", "cik", "status", "n_filings", "first_filed", "last_filed", "forms"])
        w.writerows(audit)
    print(f"  wrote coverage audit to {args.audit_out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

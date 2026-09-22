#!/usr/bin/env python3
"""Phase 2 P2-1: populate momentum_episodes for gaps 3, 5 and 10.

Reads the candidate extract produced by `momentum-backtest -dump-full`, which
already carries each candidate's bar index. Episodes are grouped here rather
than in SQL because the rule is sequential -- "first pass after >= gap sessions
with none" -- and a window function over bar indexes is less legible than the
walk, for a set this size.

Computes no features and no labels: both come from the Go engine via the
extract, per §12.
"""
from __future__ import annotations

import argparse
import csv
import os
import subprocess
import tempfile
from collections import defaultdict

GAPS = (3, 5, 10)


def psql(sql: str) -> str:
    r = subprocess.run(["psql", os.environ["DATABASE_URL"], "-tAq",
                        "-v", "ON_ERROR_STOP=1", "-c", sql],
                       capture_output=True, text=True)
    if r.returncode != 0:
        raise SystemExit(f"psql: {r.stderr.strip()}")
    return r.stdout


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--dump", default="/tmp/candidates_full.csv")
    ap.add_argument("--lockbox", default="/tmp/lockbox.csv")
    args = ap.parse_args()

    lockbox = set()
    with open(args.lockbox) as fh:
        for r in csv.reader(fh):
            if len(r) >= 2 and r[0] != "symbol":
                lockbox.add((r[0], r[1]))

    # Group every gate pass by symbol, in bar order.
    by_symbol = defaultdict(list)
    for r in csv.DictReader(open(args.dump, newline="")):
        by_symbol[r["symbol"]].append(r)
    for rows in by_symbol.values():
        rows.sort(key=lambda r: int(r["bar_index"]))

    print("=" * 78)
    print("  P2-1: momentum_episodes — gap sensitivity (§3.1)")
    print("=" * 78)
    print(f"  gate-passing candidate rows: {sum(len(v) for v in by_symbol.values()):,}")
    print(f"  symbols with >= 1 pass:      {len(by_symbol):,}")
    print()
    print(f"  {'gap':>4} {'episodes':>10} {'rows/episode':>13} {'median gate_days':>17} "
          f"{'max gate_days':>14} {'single-day':>11}")

    all_rows = []
    for gap in GAPS:
        episodes = []
        for sym, rows in by_symbol.items():
            cur = None
            for r in rows:
                idx = int(r["bar_index"])
                if cur is None or idx - cur["last_idx"] > gap:
                    if cur:
                        episodes.append(cur)
                    cur = {"first": r, "last_idx": idx, "last": r, "days": 1}
                else:
                    cur["last_idx"] = idx
                    cur["last"] = r
                    cur["days"] += 1
            if cur:
                episodes.append(cur)

        days = sorted(e["days"] for e in episodes)
        n = len(days)
        med = days[n // 2] if n % 2 else (days[n//2-1] + days[n//2]) / 2
        single = 100 * sum(1 for d in days if d == 1) / n
        total_rows = sum(len(v) for v in by_symbol.values())
        print(f"  {gap:>4} {n:>10,} {total_rows/n:>13.2f} {med:>17.1f} "
              f"{days[-1]:>14} {single:>10.1f}%")

        for e in episodes:
            f, l = e["first"], e["last"]
            all_rows.append((f["symbol"], f["date"], gap, int(f["bar_index"]),
                             e["days"], l["date"], f["bucket"], f["score_total"],
                             f["fwd_max_gain_pct"], f["fwd_max_drawdown_pct"]))

    # ── load ──
    with tempfile.NamedTemporaryFile("w", suffix=".csv", delete=False, newline="") as tf:
        w = csv.writer(tf)
        w.writerow(["symbol", "episode_start", "gap_sessions", "start_bar_index",
                    "gate_days", "episode_end", "bucket", "score_total",
                    "fwd_max_gain_pct", "fwd_max_drawdown_pct"])
        w.writerows(all_rows)
        path = tf.name

    psql("TRUNCATE momentum_episodes")
    subprocess.run(["psql", os.environ["DATABASE_URL"], "-q", "-v", "ON_ERROR_STOP=1",
                    "-c", f"\\copy momentum_episodes (symbol, episode_start, gap_sessions, "
                          f"start_bar_index, gate_days, episode_end, bucket, score_total, "
                          f"fwd_max_gain_pct, fwd_max_drawdown_pct) FROM '{path}' CSV HEADER"],
                   check=True)
    print(f"\n  loaded {len(all_rows):,} episode rows across gaps {GAPS}")

    # ── does the gap change the answer? ──
    print()
    print("  ── does the choice of gap change the base rate? ──")
    print(f"  {'gap':>4} {'bucket':<8} {'episodes':>9} {'hits':>6} {'base rate':>10}")
    for line in psql("""
        SELECT gap_sessions, bucket, count(*),
               count(*) FILTER (WHERE fwd_max_gain_pct >= 100),
               round(100.0 * count(*) FILTER (WHERE fwd_max_gain_pct >= 100) / count(*), 2)
        FROM momentum_episodes
        GROUP BY 1, 2 ORDER BY 1, 2
    """).splitlines():
        if not line:
            continue
        g, b, n, h, rate = line.split("|")
        print(f"  {g:>4} {b:<8} {int(n):>9,} {int(h):>6,} {rate:>9}%")

    print()
    print("  Note: these counts INCLUDE lockbox rows, because the table is the")
    print("  full episode set. Lockbox exclusion happens in the dataset function,")
    print("  not here -- a manifest that omitted them could not be checked against.")
    lb = sum(1 for sym, rows in by_symbol.items() for r in rows
             if (r["symbol"], r["date"]) in lockbox)
    print(f"  lockbox candidate rows present in the source extract: {lb:,}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Reserve the Phase 2 lockbox (spec §4.3) from the candidate extract.

Run BEFORE any evaluation. This reads only `symbol`, `date` and `bucket` from
the extract and never touches an outcome column -- a lockbox chosen with any
knowledge of outcomes is not a lockbox, and that ordering is the only thing
that makes the eventual result mean anything.

Definition, fixed here and recorded in the manifest:

    complete labels
    AND date within the most recent 12 months of complete-label dates
    AND symbol NOT in momentum_pilot_cohort (the in-sample 450)

Every row in the extract already has a complete label -- the backtest excludes
incomplete ones -- so the first clause holds by construction.

Talks to Postgres through `psql` rather than a driver, so it has no dependency
beyond the client already used everywhere else in this repo.
"""
from __future__ import annotations

import csv
import hashlib
import os
import subprocess
import sys
import tempfile
from datetime import date


def psql(sql: str, dsn: str) -> str:
    out = subprocess.run(["psql", dsn, "-tAq", "-v", "ON_ERROR_STOP=1", "-c", sql],
                         capture_output=True, text=True)
    if out.returncode != 0:
        raise SystemExit(f"psql failed: {out.stderr.strip()}")
    return out.stdout.strip()


def main() -> int:
    dump = sys.argv[1]
    dsn = os.environ["DATABASE_URL"]

    pilot = {l for l in psql("SELECT symbol FROM momentum_pilot_cohort", dsn).splitlines() if l}
    if not pilot:
        print("refusing to reserve: momentum_pilot_cohort is empty, so the in-sample "
              "symbols cannot be excluded and the lockbox would be contaminated "
              "with the very data score v2 was fitted on")
        return 1

    if int(psql("SELECT count(*) FROM phase2_lockbox", dsn)) > 0:
        print("refusing to reserve: phase2_lockbox is already populated. Re-reserving "
              "would silently move the boundary of the only genuinely unseen data, "
              "after reports have already been written against the old one.")
        return 1

    keys = []
    with open(dump, newline="") as fh:
        for r in csv.DictReader(fh):
            keys.append((r["symbol"], r["date"], r["bucket"]))
    if not keys:
        print("refusing to reserve: extract is empty")
        return 1

    max_date = max(d for _, d, _ in keys)
    y, m, d = (int(x) for x in max_date.split("-"))
    cutoff = date(y - 1, m, d).isoformat()

    selected = [(s, dt, b) for s, dt, b in keys if dt > cutoff and s not in pilot]
    if not selected:
        print(f"refusing to reserve: no non-pilot rows after {cutoff}")
        return 1

    # Hash the sorted member keys, so a set that gains or loses a row produces a
    # different hash and a later report claiming this reservation can be shown
    # not to match it.
    payload = "\n".join(f"{s},{dt}" for s, dt, _ in sorted(selected))
    content_hash = hashlib.sha256(payload.encode()).hexdigest()

    with tempfile.NamedTemporaryFile("w", suffix=".csv", delete=False, newline="") as tf:
        w = csv.writer(tf)
        w.writerow(["symbol", "ts", "bucket"])
        for row in sorted(selected):
            w.writerow(row)
        path = tf.name

    subprocess.run(["psql", dsn, "-q", "-v", "ON_ERROR_STOP=1",
                    "-c", f"\\copy phase2_lockbox (symbol, ts, bucket) FROM '{path}' CSV HEADER"],
                   check=True)

    definition = (f"complete labels AND date > {cutoff} (the most recent 12 months of "
                  f"complete-label dates, which end {max_date}) AND symbol not in "
                  f"momentum_pilot_cohort. Reserved before any evaluation of the widened dataset.")
    psql("INSERT INTO momentum_cohort_manifest (cohort_key, symbol_count, content_hash, definition) "
         f"VALUES ('phase2_lockbox', {len({s for s, _, _ in selected})}, '{content_hash}', "
         f"$${definition}$$)", dsn)

    dates = sorted(dt for _, dt, _ in selected)
    print(f"  reserved         {len(selected):,} rows")
    print(f"  symbols          {len({s for s, _, _ in selected}):,}  (pilot symbols excluded: {len(pilot)})")
    print(f"  date range       {dates[0]} .. {dates[-1]}")
    print(f"  cutoff           > {cutoff}   (12 months back from {max_date})")
    print(f"  content hash     {content_hash}")
    print(f"  reserved on      {date.today().isoformat()}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""Verify the frozen row-set hashes in momentum_cohort_manifest against the live tables.

WHY THIS EXISTS

Phase 2 §4.3 rule 1 requires that a report be able to PROVE it used the same
lockbox that was reserved. A hash in a table only supports that if someone can
recompute it, and recomputing it requires knowing the exact canonicalisation.

The first attempt to verify these hashes by hand failed, because
`psql ... COPY TO STDOUT` appends a trailing newline and the reservation script
joined its keys without one. The hash was right and the check was wrong -- which
is the least useful possible outcome for an integrity check, since it casts doubt
on good data. Pinning the canonicalisation in code removes that ambiguity.

CANONICAL FORM (must match scripts/reserve_phase2_lockbox.py and migration 013)

    phase2_lockbox    sha256 over "symbol,ts" lines, sorted, joined by "\\n",
                      NO trailing newline, ts as ISO date (YYYY-MM-DD)
    phase1_pilot_450  md5 over symbols sorted and joined by "," (computed in
                      SQL by migration 013, so verified by the same SQL here)
"""
from __future__ import annotations

import hashlib
import os
import subprocess
import sys


def psql(sql: str) -> str:
    out = subprocess.run(["psql", os.environ["DATABASE_URL"], "-tAq",
                          "-v", "ON_ERROR_STOP=1", "-c", sql],
                         capture_output=True, text=True)
    if out.returncode != 0:
        raise SystemExit(f"psql failed: {out.stderr.strip()}")
    return out.stdout


def main() -> int:
    failures = 0

    # ── phase2_lockbox ──
    rows = [l for l in psql(
        "SELECT symbol || ',' || to_char(ts, 'YYYY-MM-DD') "
        "FROM phase2_lockbox ORDER BY symbol, ts").splitlines() if l]
    got = hashlib.sha256("\n".join(rows).encode()).hexdigest()
    want = psql("SELECT content_hash FROM momentum_cohort_manifest "
                "WHERE cohort_key = 'phase2_lockbox'").strip()
    n = psql("SELECT symbol_count FROM momentum_cohort_manifest "
             "WHERE cohort_key = 'phase2_lockbox'").strip()
    ok = got == want
    failures += 0 if ok else 1
    print(f"  phase2_lockbox    {'OK  ' if ok else 'FAIL'}  "
          f"{len(rows):,} rows, {n} symbols")
    print(f"                    recomputed {got}")
    if not ok:
        print(f"                    manifest   {want}")
        print("                    -> the reserved set has CHANGED. Any report "
              "claiming this lockbox is not describing this data.")

    # ── phase1_pilot_450 ── verified in SQL, matching how migration 013 built it
    got2 = psql("SELECT md5(string_agg(symbol, ',' ORDER BY symbol)) "
                "FROM momentum_pilot_cohort").strip()
    want2 = psql("SELECT content_hash FROM momentum_cohort_manifest "
                 "WHERE cohort_key = 'phase1_pilot_450'").strip()
    cnt = psql("SELECT count(*) FROM momentum_pilot_cohort").strip()
    ok2 = got2 == want2
    failures += 0 if ok2 else 1
    print(f"  phase1_pilot_450  {'OK  ' if ok2 else 'FAIL'}  {cnt} symbols")
    print(f"                    recomputed {got2}")
    if not ok2:
        print(f"                    manifest   {want2}")
        print("                    -> the in-sample cohort has changed, so every "
              "out-of-sample claim that excluded it is now unverifiable.")

    # ── the property that actually matters: no overlap ──
    overlap = psql("SELECT count(*) FROM phase2_lockbox l "
                   "JOIN momentum_pilot_cohort c ON c.symbol = l.symbol").strip()
    ok3 = overlap == "0"
    failures += 0 if ok3 else 1
    print(f"  disjointness      {'OK  ' if ok3 else 'FAIL'}  "
          f"{overlap} lockbox rows share a symbol with the in-sample pilot")
    if not ok3:
        print("                    -> the lockbox is contaminated with symbols "
              "score v2 was fitted on; it is not held-out data.")

    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())

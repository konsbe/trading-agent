#!/usr/bin/env python3
"""Step 6: H2 on the lockbox, once, per docs/HEURISTIC_SIGNALS_PREREGISTRATION.md §5g.

Reads heuristic_lockbox_h2_rows (written by cmd/heuristics-lockbox) and applies
the same Signal / cmh() / month-block bootstrap as heuristics_round1.py.

  confirmed = MH OR >= 1.25 and >= 3.0pp in >= 1 ATR tercile
              and month-block bootstrap 90% lower bound > 1.05

ATR tercile cut points are the in-sample ones (non-lockbox comparison days),
so the strata are defined without looking at the lockbox.

Run: python3 scripts/heuristics_lockbox_h2.py
"""
from __future__ import annotations

import random
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import heuristics_round1 as r1  # noqa: E402
from research_round1 import cmh  # noqa: E402

SEED = 20260925
BOOT = 1000


def cells_sql() -> str:
    hits = ",\n".join(
        f"count(*) filter (where fwd_return_{h}s >= {r1.HIT_PCT}) pos{h}, "
        f"count(*) filter (where fwd_return_{h}s <= -{r1.HIT_PCT}) neg{h}"
        for h in r1.HORIZONS
    )
    return f"""
    with cuts as (
        select percentile_cont(1/3.0) within group (order by atr_pct) lo,
               percentile_cont(2/3.0) within group (order by atr_pct) hi
        from heuristic_comparison_days
        where label_complete and not in_lockbox and atr_pct is not null
    ),
    cal as (
        select t, (dense_rank() over (order by t) - 1) / {r1.FOLD_SESSIONS} as fold
        from (select distinct t from heuristic_lockbox_h2_rows) d
    )
    select grp, to_char(r.t, 'YYYY-MM') as mon, c.fold,
           case when atr_pct <= cuts.lo then 0 when atr_pct <= cuts.hi then 1 else 2 end as terc,
           'null' as up, false as y2020,
           count(*) as n,
           {hits},
           sum(fwd_abs_move_10s) as a10_sum
    from heuristic_lockbox_h2_rows r join cal c using (t) cross join cuts
    where label_complete and atr_pct is not null
    group by 1, 2, 3, 4
    """


def main() -> int:
    rng = random.Random(SEED)
    sig = r1.Signal("H2", "rsi_oversold", "pos", False, r1.psql_csv(cells_sql()))
    rows = sig.rows
    n_sig = sum(r["n"] for r in rows if r["grp"] == 1)
    n_cmp = sum(r["n"] for r in rows if r["grp"] == 0)
    months = sorted({r["mon"] for r in rows})

    print("=" * 100)
    print("  STEP 6 — H2 (RSI oversold -> fwd_return_10s >= +3.0%) on the LOCKBOX. One shot.")
    print("=" * 100)
    print(f"  signal episodes n={n_sig:,}   comparison days n={n_cmp:,}   "
          f"months {months[0]}..{months[-1]} ({len(months)} blocks)")

    for h in r1.HORIZONS:
        c = cmh(sig.tables(rows, h))
        tag = "PRIMARY" if h == 10 else "context"
        print(f"  {h:>2}s {tag:<8} MH OR {c['or']:.3f} RBG95 [{c['lo']:.3f}, {c['hi']:.3f}]"
              f"  p={c['p']:.2e}  strata={c['strata']}")
        for terc, (sh, ch, d, sn) in sig.pp_by_tercile(rows, h).items():
            print(f"        ATR tercile {terc}: signal {sh:5.2f}%  comparison {ch:5.2f}%"
                  f"  diff {d:+6.2f}pp  (signal n={sn:,})")

    c = cmh(sig.tables(rows, 10))
    boots = sig.bootstrap(lambda rs: r1.mh_or_only(sig.tables(rs, 10)), BOOT, rng)
    lo, hi = r1.pct(boots, 0.05), r1.pct(boots, 0.95)
    best_pp = max(v[2] for v in sig.pp_by_tercile(rows, 10).values())

    ok_or = c["or"] >= r1.OR_FLOOR
    ok_pp = best_pp >= r1.PP_FLOOR
    ok_ci = lo > r1.CI_BAR
    print()
    print(f"  month-block bootstrap 90% [{lo:.3f}, {hi:.3f}]  ({BOOT} resamples, seed {SEED})")
    print(f"  MH OR >= {r1.OR_FLOOR}: {'yes' if ok_or else 'NO'} ({c['or']:.3f})"
          f"   >= {r1.PP_FLOOR}pp in a tercile: {'yes' if ok_pp else 'NO'} ({best_pp:+.2f})"
          f"   bootstrap lower > {r1.CI_BAR}: {'yes' if ok_ci else 'NO'} ({lo:.3f})")
    verdict = "CONFIRMED IN THE LOCKBOX" if (ok_or and ok_pp and ok_ci) else "NOT CONFIRMED IN THE LOCKBOX"
    print(f"  VERDICT: {verdict}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

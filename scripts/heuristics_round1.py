#!/usr/bin/env python3
"""Heuristic signals, step 5: H1-H10 exactly as pre-registered.

docs/HEURISTIC_SIGNALS_PREREGISTRATION.md §3, with Amendments 1-5. Nothing in
this file chooses a definition; every choice below is quoted from there.

  rows     signal episodes vs comparison days where that signal did not fire,
           label_complete, lockbox excluded (Amendment 5)
  hit      fwd_return_Ns beyond +/-3.0% in the hypothesis direction (Amendment 3)
  strata   126-session fold x ATR tercile (+ uptrend for H8a/H8b/H9/H10)
  test     CMH with the RBG CI, cmh() from research_round1.py (one implementation)
  floor    MH OR >= 1.25 and >= 3.0pp in at least one ATR tercile;
           H5: >= 1.25x lift in fwd_abs_move_10s;
           H9/H10: also above the best same-direction OR among H1-H8
  confirm  floor cleared and BH-adjusted p < 0.05 across the 11 hypotheses
  cluster  month-block bootstrap 90% CI, reported beside the verdict

The lockbox is never read: every query filters `NOT in_lockbox`, and lockbox
rows carry NULL labels by table constraint in any case.

Run: python3 scripts/heuristics_round1.py [--boot 1000] [--seed 20260925]
"""
from __future__ import annotations

import argparse
import csv
import io
import math
import random
import subprocess
import sys
from collections import defaultdict
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from research_round1 import cmh  # noqa: E402  one CMH implementation for the project

FOLD_SESSIONS = 126
OR_FLOOR = 1.25
PP_FLOOR = 3.0
LIFT_FLOOR = 1.25
CI_BAR = 1.05
HIT_PCT = 3.0
ALPHA = 0.05

# (hypothesis, signal table suffix, direction, stratify by uptrend)
PLAN = [
    ("H1",  "rsi_overbought",    "neg", False),
    ("H2",  "rsi_oversold",      "pos", False),
    ("H3",  "macd_bull_cross",   "pos", False),
    ("H4",  "macd_bear_cross",   "neg", False),
    ("H5",  "bb_squeeze",        "mag", False),
    ("H6",  "bearish_pattern",   "neg", False),
    ("H7",  "bullish_pattern",   "pos", False),
    ("H8a", "low_sweep_reclaim", "pos", True),
    ("H8b", "high_sweep_reject", "neg", True),
    ("H9",  "buy_watch_c4",      "pos", True),
    ("H10", "trim_watch_c4",     "neg", True),
]
HORIZONS = (5, 10, 20)


# ── data: per-(month, stratum, group) cells aggregated in Postgres ──────────

def psql_csv(sql: str) -> list[dict]:
    out = subprocess.run(
        ["docker", "exec", "-i", "ta-phase1", "psql", "-U", "postgres", "-d", "trading",
         "-v", "ON_ERROR_STOP=1", "-c", f"COPY ({sql}) TO STDOUT WITH CSV HEADER"],
        capture_output=True, text=True, check=True,
    ).stdout
    return list(csv.DictReader(io.StringIO(out)))


def cells_sql(signal: str) -> str:
    hits = ",\n".join(
        f"count(*) filter (where r{h} >= {HIT_PCT}) pos{h}, "
        f"count(*) filter (where r{h} <= -{HIT_PCT}) neg{h}"
        for h in HORIZONS
    )
    return f"""
    with sessions as (
        select t from heuristic_comparison_days
        union select t from heuristic_ep_{signal}
    ),
    cal as (select t, (dense_rank() over (order by t) - 1) / {FOLD_SESSIONS} as fold from sessions),
    cuts as (
        select percentile_cont(1/3.0) within group (order by atr_pct) lo,
               percentile_cont(2/3.0) within group (order by atr_pct) hi
        from heuristic_comparison_days
        where label_complete and not in_lockbox and atr_pct is not null
    ),
    rows as (
        select 1 as grp, t, atr_pct, uptrend,
               fwd_return_5s r5, fwd_return_10s r10, fwd_return_20s r20, fwd_abs_move_10s a10
        from heuristic_ep_{signal}
        where label_complete and not in_lockbox
        union all
        select 0, t, atr_pct, uptrend,
               fwd_return_5s, fwd_return_10s, fwd_return_20s, fwd_abs_move_10s
        from heuristic_comparison_days
        where label_complete and not in_lockbox and not fired_{signal}
    )
    select r.grp, to_char(r.t, 'YYYY-MM') as mon, c.fold,
           case when r.atr_pct <= cuts.lo then 0 when r.atr_pct <= cuts.hi then 1 else 2 end as terc,
           coalesce(r.uptrend::text, 'null') as up,
           (extract(year from r.t) = 2020) as y2020,
           count(*) as n,
           {hits},
           sum(r.a10) as a10_sum
    from rows r join cal c using (t) cross join cuts
    where r.atr_pct is not null
    group by 1, 2, 3, 4, 5, 6
    """


# ── statistics ───────────────────────────────────────────────────────────────

def bh(pvals: dict[str, float]) -> dict[str, float]:
    items = sorted(pvals.items(), key=lambda kv: kv[1])
    m = len(items)
    q, running = {}, 1.0
    for rank in range(m, 0, -1):
        k, p = items[rank - 1]
        running = min(running, p * m / rank)
        q[k] = running
    return q


def mh_or_only(tables) -> float | None:
    r = s = 0.0
    for a, b, c, d in tables:
        n = a + b + c + d
        if n < 2 or a + b == 0 or c + d == 0:
            continue
        r += a * d / n
        s += b * c / n
    return r / s if s > 0 and r > 0 else None


def pct(xs, q):
    s = sorted(xs)
    return s[min(len(s) - 1, max(0, int(round(q * (len(s) - 1)))))]


class Signal:
    def __init__(self, hyp, signal, direction, by_up, rows):
        self.hyp, self.signal, self.direction, self.by_up = hyp, signal, direction, by_up
        self.rows = []
        for r in rows:
            if by_up and r["up"] == "null":
                continue
            self.rows.append({
                "grp": int(r["grp"]), "mon": r["mon"], "y2020": r["y2020"] == "t",
                "stratum": (int(r["fold"]), int(r["terc"])) + ((r["up"],) if by_up else ()),
                "terc": int(r["terc"]), "n": int(r["n"]),
                **{f"pos{h}": int(r[f"pos{h}"]) for h in HORIZONS},
                **{f"neg{h}": int(r[f"neg{h}"]) for h in HORIZONS},
                "a10": float(r["a10_sum"] or 0.0),
            })

    def hits(self, r, h):
        return r[f"pos{h}"] if self.direction == "pos" else r[f"neg{h}"]

    # directional -------------------------------------------------------------
    def tables(self, rows, h):
        cell = defaultdict(lambda: [0, 0, 0, 0])
        for r in rows:
            k = self.hits(r, h)
            t = cell[r["stratum"]]
            if r["grp"] == 1:
                t[0] += k
                t[1] += r["n"] - k
            else:
                t[2] += k
                t[3] += r["n"] - k
        return [tuple(v) for v in cell.values()]

    def pp_by_tercile(self, rows, h):
        acc = defaultdict(lambda: [0, 0, 0, 0])
        for r in rows:
            k = self.hits(r, h)
            a = acc[r["terc"]]
            if r["grp"] == 1:
                a[0] += k
                a[1] += r["n"]
            else:
                a[2] += k
                a[3] += r["n"]
        out = {}
        for terc, (sh, sn, ch, cn) in sorted(acc.items()):
            if sn and cn:
                out[terc] = (100 * sh / sn, 100 * ch / cn, 100 * sh / sn - 100 * ch / cn, sn)
        return out

    # magnitude (H5) ----------------------------------------------------------
    def lift(self, rows):
        acc = defaultdict(lambda: [0.0, 0, 0.0, 0])
        for r in rows:
            a = acc[r["stratum"]]
            if r["grp"] == 1:
                a[0] += r["a10"]
                a[1] += r["n"]
            else:
                a[2] += r["a10"]
                a[3] += r["n"]
        num = den = 0.0
        for ss, sn, cs, cn in acc.values():
            if sn and cn:
                num += sn * (ss / sn)
                den += sn * (cs / cn)
        return num / den if den > 0 else None

    def lift_by_tercile(self, rows):
        out = {}
        for terc in (0, 1, 2):
            sub = [r for r in rows if r["terc"] == terc]
            out[terc] = self.lift(sub)
        return out

    # bootstrap over calendar months -----------------------------------------
    def bootstrap(self, stat, n_boot, rng):
        by_mon = defaultdict(list)
        for r in self.rows:
            by_mon[r["mon"]].append(r)
        months = sorted(by_mon)
        vals = []
        for _ in range(n_boot):
            sample = []
            for m in rng.choices(months, k=len(months)):
                sample.extend(by_mon[m])
            v = stat(sample)
            if v is not None:
                vals.append(v)
        return vals


# ── driver ───────────────────────────────────────────────────────────────────

def fmt_ci(lo, hi):
    return f"[{lo:.3f}, {hi:.3f}]"


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--boot", type=int, default=1000)
    ap.add_argument("--seed", type=int, default=20260925)
    args = ap.parse_args()
    rng = random.Random(args.seed)

    print("=" * 110)
    print("  HEURISTIC SIGNALS — step 5, pre-registered (§3 + Amendments 1-5). Lockbox NOT read.")
    print("=" * 110)
    print(f"  hit: fwd_return beyond +/-{HIT_PCT}% in the hypothesis direction | strata: "
          f"{FOLD_SESSIONS}-session fold x ATR tercile (+uptrend H8-H10)")
    print(f"  floor: MH OR >= {OR_FLOOR} and >= {PP_FLOOR}pp in >= 1 tercile | H5 lift >= {LIFT_FLOOR}x"
          f" | confirm: floor and BH p < {ALPHA} | bootstrap: {args.boot} month blocks, seed {args.seed}")

    sigs = {}
    for hyp, signal, direction, by_up in PLAN:
        sigs[hyp] = Signal(hyp, signal, direction, by_up, psql_csv(cells_sql(signal)))

    primary = {}
    for hyp, sig in sigs.items():
        rows = sig.rows
        rows_no20 = [r for r in rows if not r["y2020"]]
        n_sig = sum(r["n"] for r in rows if r["grp"] == 1)
        n_cmp = sum(r["n"] for r in rows if r["grp"] == 0)
        res = {"n_sig": n_sig, "n_cmp": n_cmp}
        if sig.direction == "mag":
            res["lift"] = sig.lift(rows)
            res["lift_no20"] = sig.lift(rows_no20)
            res["lift_terc"] = sig.lift_by_tercile(rows)
            boots = [math.log(v) for v in sig.bootstrap(sig.lift, args.boot, rng) if v > 0]
            res["ci90"] = (math.exp(pct(boots, 0.05)), math.exp(pct(boots, 0.95)))
            below = sum(1 for b in boots if b <= 0) / len(boots)
            res["p"] = max(min(1.0, 2 * min(below, 1 - below)), 1 / len(boots))
        else:
            res["cmh"] = {h: cmh(sig.tables(rows, h)) for h in HORIZONS}
            res["cmh_no20"] = cmh(sig.tables(rows_no20, 10))
            res["pp"] = {h: sig.pp_by_tercile(rows, h) for h in HORIZONS}
            boots = sig.bootstrap(lambda rs: mh_or_only(sig.tables(rs, 10)), args.boot, rng)
            res["ci90"] = (pct(boots, 0.05), pct(boots, 0.95))
            res["p"] = res["cmh"][10]["p"] if res["cmh"][10] else 1.0
        primary[hyp] = res

    q = bh({h: r["p"] for h, r in primary.items()})

    best = {}
    for d, hyps in (("pos", ("H2", "H3", "H7", "H8a")), ("neg", ("H1", "H4", "H6", "H8b"))):
        cands = [(primary[h]["cmh"][10]["or"], h) for h in hyps if primary[h]["cmh"][10]]
        best[d] = max(cands)

    # ── per-hypothesis detail ──
    for hyp, sig in sigs.items():
        r = primary[hyp]
        print()
        print("─" * 110)
        print(f"  {hyp}  {sig.signal}  direction={sig.direction}   "
              f"signal episodes n={r['n_sig']:,}   comparison days n={r['n_cmp']:,}")
        if sig.direction == "mag":
            print(f"    lift fwd_abs_move_10s {r['lift']:.3f}x   bootstrap 90% {fmt_ci(*r['ci90'])}"
                  f"   p={r['p']:.4f}   without 2020 {r['lift_no20']:.3f}x")
            for terc, v in r["lift_terc"].items():
                print(f"      ATR tercile {terc}: lift {v:.3f}x" if v else f"      ATR tercile {terc}: —")
            continue
        for h in HORIZONS:
            c = r["cmh"][h]
            tag = "PRIMARY" if h == 10 else "context"
            if c is None:
                print(f"    {h:>2}s {tag:<8} not evaluable")
                continue
            print(f"    {h:>2}s {tag:<8} MH OR {c['or']:.3f} RBG95 {fmt_ci(c['lo'], c['hi'])}"
                  f"  p={c['p']:.2e}  strata={c['strata']}")
            for terc, (sh, ch, d, sn) in r["pp"][h].items():
                print(f"          ATR tercile {terc}: signal hit {sh:5.2f}%  comparison {ch:5.2f}%"
                      f"  diff {d:+6.2f}pp  (signal n={sn:,})")
        c0 = r["cmh_no20"]
        print(f"    10s without 2020: MH OR {c0['or']:.3f} {fmt_ci(c0['lo'], c0['hi'])}" if c0 else
              "    10s without 2020: —")
        print(f"    10s month-block bootstrap 90% {fmt_ci(*r['ci90'])}")

    # ── results table ──
    print()
    print("=" * 110)
    print("  RESULTS — primary horizon 10 sessions, all 11 reported")
    print("=" * 110)
    print(f"  {'hyp':<4} {'signal':<18} {'effect':>7} {'RBG 95% CI':>17} {'boot 90% CI':>17} "
          f"{'best pp':>8} {'p':>9} {'BH q':>9}  verdict")
    for hyp, sig in sigs.items():
        r = primary[hyp]
        extra = ""
        if sig.direction == "mag":
            eff = r["lift"]
            rbg = "—"
            best_pp = "—"
            floor = eff >= LIFT_FLOOR
        else:
            c = r["cmh"][10]
            eff = c["or"]
            rbg = fmt_ci(c["lo"], c["hi"])
            pps = [v[2] for v in r["pp"][10].values()]
            best_pp = f"{max(pps):+.2f}"
            floor = eff >= OR_FLOOR and max(pps) >= PP_FLOOR
            if hyp in ("H9", "H10"):
                d = sig.direction
                bor, bh_ = best[d]
                comp = "H8a" if hyp == "H9" else "H8b"
                cor = primary[comp]["cmh"][10]["or"]
                beats = eff > bor
                extra = (f" | vs best same-direction {bh_} OR {bor:.3f}: {'above' if beats else 'NOT above'}"
                         f" | vs own input {comp} OR {cor:.3f}: {'above' if eff > cor else 'NOT above'}")
                floor = floor and beats
        sig_ok = q[hyp] < ALPHA
        if floor and sig_ok:
            verdict = "CONFIRMED"
        elif floor:
            verdict = "not confirmed (unresolved)"
        elif sig_ok:
            verdict = "not confirmed (below floor)"
        else:
            verdict = "not confirmed"
        ci_note = "boot lower > 1.05" if r["ci90"][0] > CI_BAR else "boot lower <= 1.05"
        print(f"  {hyp:<4} {sig.signal:<18} {eff:>7.3f} {rbg:>17} {fmt_ci(*r['ci90']):>17} "
              f"{best_pp:>8} {r['p']:>9.2e} {q[hyp]:>9.2e}  {verdict}  ({ci_note}){extra}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

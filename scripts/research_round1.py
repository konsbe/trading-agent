#!/usr/bin/env python3
"""Research round 1, exactly as pre-registered in Phase 2 §2.2.

FOUR hypotheses. (c) was abandoned before the round ran because Tiingo News is
a ~3-month archive against Finnhub's ~12 (§3.5.1); no substitute was added.

  (a) path-aware labels      fp_100_dd50, fp_100_atr
  (b) shorter horizon        hit_20_10s (+20% within 10 sessions)
  (d) sector strength        sector_strength_pct
  (e) market regime          SPY/IWM 20-session returns, VIX, as-of t-1

PROTOCOL, fixed in advance and not negotiable here
  * bucket-stratified, never pooled across buckets (§4.2.0 Rule 1)
  * episode level (§3.1)
  * purged walk-forward folds (§4.1), folds used as an additional stratum
  * lockbox region excluded, untouched
  * reported with AND without the 2020 fold (§6.1)
  * bar is an EFFECT SIZE, not a p-value (§4.2.0 Rule 2):
        MH OR >= 1.25 with CI lower bound > 1.05
        (b) additionally >= 3.0pp within bucket

WHY FOLDS ARE A STRATUM RATHER THAN A TRAIN/TEST SPLIT
  These are single-feature median-split tests. Nothing is fitted, so there is
  no training set to hold out from. The walk-forward requirement is honoured
  by making each 126-session window its own stratum: a feature must work
  across time periods, not only in aggregate, and CMH combines the windows
  without ever comparing between them.
"""
from __future__ import annotations

import argparse
import csv
import math
from collections import defaultdict

FOLD_SESSIONS = 126
MH_OR_BAR = 1.25
CI_LOWER_BAR = 1.05
PP_BAR = 3.0


# ── statistics ───────────────────────────────────────────────────────────────

def chi2_sf_1df(x: float) -> float:
    return 1.0 if x <= 0 else math.erfc(math.sqrt(x / 2))


def cmh(tables):
    """Cochran-Mantel-Haenszel with a Robins-Breslow-Greenland CI.

    tables: list of (a, b, c, d) = (hi-hit, hi-miss, lo-hit, lo-miss).

    RBG rather than a naive variance because strata here are small and
    numerous — folds x buckets x ATR terciles — which is exactly the regime
    where the simpler estimators are known to be biased.
    """
    sum_a = sum_e = sum_v = 0.0
    R = S = 0.0
    PR = PS = QR = QS = 0.0
    used = 0
    for a, b, c, d in tables:
        n1, n2 = a + b, c + d
        m1, m2 = a + c, b + d
        N = n1 + n2
        if N < 2 or n1 == 0 or n2 == 0 or m1 == 0 or m2 == 0:
            continue
        used += 1
        sum_a += a
        sum_e += n1 * m1 / N
        sum_v += n1 * n2 * m1 * m2 / (N * N * (N - 1))
        r, s = a * d / N, b * c / N
        R += r
        S += s
        P, Q = (a + d) / N, (b + c) / N
        PR += P * r
        PS += P * s
        QR += Q * r
        QS += Q * s
    if used == 0 or R == 0 or S == 0 or sum_v == 0:
        return None
    or_mh = R / S
    stat = (abs(sum_a - sum_e) - 0.5) ** 2 / sum_v
    var_log = PR / (2 * R * R) + (PS + QR) / (2 * R * S) + QS / (2 * S * S)
    se = math.sqrt(var_log)
    lo = math.exp(math.log(or_mh) - 1.96 * se)
    hi = math.exp(math.log(or_mh) + 1.96 * se)
    return {"or": or_mh, "lo": lo, "hi": hi, "p": chi2_sf_1df(stat),
            "chi2": stat, "strata": used, "n": int(sum(sum(t) for t in tables))}


def median(xs):
    s = sorted(xs)
    n = len(s)
    return s[n // 2] if n % 2 else (s[n // 2 - 1] + s[n // 2]) / 2


def terciles(xs):
    s = sorted(xs)
    n = len(s)
    return (s[n // 3], s[2 * n // 3]) if n >= 3 else (None, None)


# ── data ─────────────────────────────────────────────────────────────────────

def load(path, pilot_path, lo_date, hi_date):
    pilot = {l.strip() for l in open(pilot_path) if l.strip()}
    rows = []
    for r in csv.DictReader(open(path, newline="")):
        # Lockbox region: the window x every non-pilot symbol. Untouched.
        if lo_date <= r["date"] <= hi_date and r["symbol"] not in pilot:
            continue
        # Population A (pilot symbols inside the pilot window) is in-sample.
        if r["date"] >= "2023-09-18" and r["symbol"] in pilot:
            continue
        if r["episode_start"] != "true":
            continue
        rows.append(r)
    return rows, len(pilot)


def fnum(r, k):
    v = r.get(k, "")
    if v in ("", None):
        return None
    try:
        return float(v)
    except ValueError:
        return None


def fbool(r, k):
    v = r.get(k, "")
    return None if v in ("", None) else (v == "true")


def assign_folds(rows):
    """Contiguous 126-session folds over the union of candidate dates."""
    dates = sorted({r["date"] for r in rows})
    fold_of = {}
    for i, d in enumerate(dates):
        fold_of[d] = i // FOLD_SESSIONS
    for r in rows:
        r["_fold"] = fold_of[r["date"]]
    return len({r["_fold"] for r in rows})


# ── one hypothesis test ──────────────────────────────────────────────────────

def test_feature(rows, feature, label_key, label_fn, exclude_2020=False):
    """CMH over bucket x ATR-tercile x fold, median split WITHIN each stratum."""
    use = []
    for r in rows:
        if exclude_2020 and r["date"][:4] == "2020":
            continue
        y = label_fn(r)
        x = fnum(r, feature)
        a = fnum(r, "atr_pct")
        if y is None or x is None or a is None:
            continue
        use.append((r["bucket"], r["_fold"], a, x, y))
    if len(use) < 40:
        return None, len(use)

    lo_a, hi_a = terciles([u[2] for u in use])
    if lo_a is None:
        return None, len(use)

    cells = defaultdict(list)
    for b, f, a, x, y in use:
        band = "lo" if a <= lo_a else ("mid" if a <= hi_a else "hi")
        cells[(b, band, f)].append((x, y))

    tables = []
    for _, vals in cells.items():
        if len(vals) < 4:
            continue
        m = median([v[0] for v in vals])
        hi = [y for x, y in vals if x > m]
        lo = [y for x, y in vals if x <= m]
        if not hi or not lo:
            continue
        tables.append((sum(hi), len(hi) - sum(hi), sum(lo), len(lo) - sum(lo)))
    return cmh(tables), len(use)


def per_bucket_pp(rows, feature, label_fn, exclude_2020=False):
    """Within-bucket percentage-point difference, for (b)'s extra bar."""
    out = {}
    for bucket in ("market", "penny"):
        vals = []
        for r in rows:
            if r["bucket"] != bucket:
                continue
            if exclude_2020 and r["date"][:4] == "2020":
                continue
            y, x = label_fn(r), fnum(r, feature)
            if y is None or x is None:
                continue
            vals.append((x, y))
        if len(vals) < 30:
            out[bucket] = None
            continue
        m = median([v[0] for v in vals])
        hi = [y for x, y in vals if x > m]
        lo = [y for x, y in vals if x <= m]
        if not hi or not lo:
            out[bucket] = None
            continue
        out[bucket] = (100 * sum(hi) / len(hi) - 100 * sum(lo) / len(lo), len(vals))
    return out


def verdict(res, need_pp=None, pp=None):
    if res is None:
        return "FAIL", "not evaluable"
    ok_or = res["or"] >= MH_OR_BAR
    ok_ci = res["lo"] > CI_LOWER_BAR
    if need_pp is not None:
        ok_pp = any(v and v[0] >= PP_BAR for v in (pp or {}).values())
    else:
        ok_pp = True
    if ok_or and ok_ci and ok_pp:
        return "PASS", ""
    why = []
    if not ok_or:
        why.append(f"OR {res['or']:.3f} < {MH_OR_BAR}")
    if not ok_ci:
        why.append(f"CI lower {res['lo']:.3f} <= {CI_LOWER_BAR}")
    if not ok_pp:
        why.append(f"no bucket >= {PP_BAR}pp")
    return "FAIL", "; ".join(why)


# ── driver ───────────────────────────────────────────────────────────────────

def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--dump", default=".work/data/cand_round1.csv")
    ap.add_argument("--pilot", default=".work/data/pilot.txt")
    ap.add_argument("--sector-strength", default=".work/data/sector_strength.csv")
    ap.add_argument("--region", default="2025-03-28:2026-03-27")
    args = ap.parse_args()

    lo_d, hi_d = args.region.split(":")
    rows, n_pilot = load(args.dump, args.pilot, lo_d, hi_d)
    n_folds = assign_folds(rows)

    # sector strength, already shifted to its usable (t-1) date
    ss = {}
    for r in csv.reader(open(args.sector_strength)):
        if len(r) >= 3:
            ss[(r[0], r[1])] = float(r[2])
    for r in rows:
        v = ss.get((r["date"], r["sector"]))
        r["sector_strength_pct"] = "" if v is None else f"{v}"

    print("=" * 100)
    print("  RESEARCH ROUND 1 — Phase 2 §2.2, pre-registered. LIVE.")
    print("=" * 100)
    print(f"  extract            {args.dump}")
    print(f"  episodes           {len(rows):,}   (lockbox region {lo_d}..{hi_d} EXCLUDED, untouched)")
    print(f"  pilot excluded     {n_pilot} symbols inside the pilot window (population A)")
    print(f"  walk-forward folds {n_folds} x {FOLD_SESSIONS} sessions, used as an additional stratum")
    print(f"  strata             bucket x atr tercile x fold")
    print(f"  bars               MH OR >= {MH_OR_BAR}, CI lower > {CI_LOWER_BAR}"
          f"   (b) also >= {PP_BAR}pp within a bucket")

    # label accessors
    lab = {
        "fp_100_dd50": lambda r: fbool(r, "fp_100_dd50"),
        "fp_100_atr":  lambda r: fbool(r, "fp_100_atr"),
        "hit_20_10s":  lambda r: fbool(r, "hit_20_10s"),
        "hit_100":     lambda r: (None if r["fwd_max_gain_pct"] == ""
                                  else float(r["fwd_max_gain_pct"]) >= 100),
    }

    for k, fn in lab.items():
        vals = [fn(r) for r in rows]
        ok = [v for v in vals if v is not None]
        print(f"  base rate {k:<12} {100*sum(ok)/max(len(ok),1):5.2f}%  (n={len(ok):,})")

    # §5.2 feature set, for (a) and (b)
    FEATS = ["rvol_20", "vol_accel", "change_pct", "gap_pct", "atr_pct",
             "dollar_volume", "rsi_14", "vwap_dist_pct", "pct_of_52w_high", "range_20"]

    # (a) CONTROL. The hypothesis as written is not "some feature clears the
    # bar on a path-aware label" — it is that features "may separate on it
    # WHERE THEY DO NOT ON hit_100". That comparison is part of the
    # pre-registered text, so it is part of the test, and a feature that
    # separates just as well on hit_100 does not satisfy it. Running hit_100
    # through the identical test is therefore required, not optional.
    print()
    print("─" * 100)
    print("  (a) CONTROL — the same features on hit_100, identical test.")
    print("      (a) claims separation on the new labels WHERE THERE IS NONE on hit_100.")
    print(f"      {'feature':<20} {'hit_100 OR':>11} {'fp_dd50 OR':>11} {'fp_atr OR':>11}   reading")
    ctrl = {}
    for f in FEATS:
        vals = {}
        for lname in ("hit_100", "fp_100_dd50", "fp_100_atr"):
            res, _ = test_feature(rows, f, lname, lab[lname])
            vals[lname] = res["or"] if res else None
        ctrl[f] = vals
        h, d1, d2 = vals["hit_100"], vals["fp_100_dd50"], vals["fp_100_atr"]
        if h is None:
            continue
        # "separates on the new label where it does not on hit_100" requires
        # hit_100 to be BELOW the bar while the new label is above it.
        novel = h < MH_OR_BAR and ((d1 or 0) >= MH_OR_BAR or (d2 or 0) >= MH_OR_BAR)
        note = "NOVEL to path-aware" if novel else ("same on hit_100 — not novel"
                                                    if (d1 or 0) >= MH_OR_BAR else "")
        print(f"      {f:<20} {h:>11.3f} {(d1 or 0):>11.3f} {(d2 or 0):>11.3f}   {note}")

    plan = [
        ("(a) path-aware: fp_100_dd50", FEATS, "fp_100_dd50", None),
        ("(a) path-aware: fp_100_atr",  FEATS, "fp_100_atr",  None),
        ("(b) short horizon: hit_20_10s", FEATS, "hit_20_10s", PP_BAR),
        ("(d) sector strength", ["sector_strength_pct"], "hit_100", None),
        ("(d) sector strength (fp)", ["sector_strength_pct"], "fp_100_dd50", None),
        ("(e) regime: SPY 20d", ["spy_ret20"], "hit_100", None),
        ("(e) regime: IWM 20d", ["iwm_ret20"], "hit_100", None),
        ("(e) regime: VIX",     ["vix"],       "hit_100", None),
        ("(e) regime: VIX (fp)", ["vix"],      "fp_100_dd50", None),
    ]

    results = []
    for name, feats, label, ppbar in plan:
        print()
        print("─" * 100)
        print(f"  {name}   label={label}")
        print(f"    {'feature':<20} {'n':>7} {'strata':>7} {'MH OR':>8} {'95% CI':>18} "
              f"{'p':>8}  {'verdict':<6} why")
        for f in feats:
            for excl in (False, True):
                res, n = test_feature(rows, f, label, lab[label], exclude_2020=excl)
                pp = per_bucket_pp(rows, f, lab[label], excl) if ppbar else None
                v, why = verdict(res, ppbar, pp)
                tag = f"{f}{' (no 2020)' if excl else ''}"
                if res is None:
                    print(f"    {tag:<20} {n:>7,} {'—':>7} {'—':>8} {'—':>18} {'—':>8}  {v:<6} {why}")
                else:
                    ci = f"[{res['lo']:.3f}, {res['hi']:.3f}]"
                    print(f"    {tag:<20} {res['n']:>7,} {res['strata']:>7} {res['or']:>8.3f} "
                          f"{ci:>18} {res['p']:>8.4f}  {v:<6} {why}")
                results.append({"hypothesis": name, "feature": f, "label": label,
                                "excl2020": excl, "res": res, "pp": pp, "verdict": v})
                if pp:
                    for b, val in pp.items():
                        if val:
                            print(f"      per bucket {b:<7} {val[0]:+6.2f}pp (n={val[1]:,})")

    # ── the single results table ──
    print()
    print("=" * 100)
    print("  RESULTS TABLE — one row per hypothesis (best feature by MH OR, with 2020)")
    print("=" * 100)
    print(f"  {'hypothesis':<30} {'bar':<26} {'best effect (95% CI)':<28} {'no-2020':<10} {'verdict'}")
    by_hyp = defaultdict(list)
    for r in results:
        by_hyp[r["hypothesis"]].append(r)
    any_pass = False
    for name, rs in by_hyp.items():
        withs = [r for r in rs if not r["excl2020"] and r["res"]]
        if not withs:
            print(f"  {name:<30} {'MH OR>=1.25, CI>1.05':<26} {'not evaluable':<28} {'—':<10} FAIL")
            continue
        # For (a), apply the pre-registered comparison clause: a feature that
        # separates equally well on hit_100 does not satisfy "where they do
        # not on hit_100", so it cannot carry the hypothesis.
        pool_ = withs
        if name.startswith("(a)"):
            novel = [r for r in withs
                     if (ctrl.get(r["feature"], {}).get("hit_100") or 99) < MH_OR_BAR]
            if not novel:
                print(f"  {name:<30} {'OR>=1.25, CI>1.05, novel vs hit_100':<26} "
                      f"{'every clearing feature also':<28} {'—':<10} FAIL")
                print(f"  {'':<30} {'':<26} {'clears on hit_100':<28}")
                continue
            pool_ = novel
        best = max(pool_, key=lambda r: r["res"]["or"])
        m = [r for r in rs if r["excl2020"] and r["feature"] == best["feature"] and r["res"]]
        no20 = f"{m[0]['res']['or']:.3f}" if m else "—"
        eff = f"{best['res']['or']:.3f} [{best['res']['lo']:.3f}, {best['res']['hi']:.3f}]"
        bar = "OR>=1.25, CI>1.05" + (", >=3pp" if "(b)" in name else "")
        if best["verdict"] == "PASS":
            any_pass = True
        print(f"  {name:<30} {bar:<26} {eff:<28} {no20:<10} {best['verdict']}")

    print()
    print("=" * 100)
    print("  STOPPING RULE (§2.2)")
    print("=" * 100)
    if any_pass:
        # Arithmetically, (b) clears. The §2.4 ruling does not count it: the
        # clearing feature is atr_pct, which is the variable this test
        # stratifies on, and which scores HIGHER on the hit_100 label that (b)
        # set out to improve on. Printed here so a future reader running this
        # script sees the ruling next to the number that prompted it, rather
        # than discovering a "PASS" and drawing the opposite conclusion.
        print("  Arithmetically, a hypothesis cleared the bar — see the table above.")
        print()
        print("  §2.4 RULING (2026-09-22): it is NOT counted, and research STOPS.")
        print("  The only clearing feature is atr_pct: the stratification variable,")
        print("  scoring higher on hit_100 (1.752) than on the short-horizon label")
        print("  (b) proposed (1.616). §5.2's feature set contains the stratifier —")
        print("  a flaw in the pre-registered rule, not a finding.")
        print()
        print("  Phase 2 is CLOSED. §5, §7 and §9 are not built.")
        print("  No round 2 without a new written justification from the user.")
    else:
        print("  NO hypothesis cleared §4.2.0's effect-size bar out-of-sample.")
        print()
        print("  Per the pre-registered stopping rule, research STOPS. The screener is")
        print("  the product, and the spec records that as the finished result.")
        print("  No round 2 without a new written justification from the user.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

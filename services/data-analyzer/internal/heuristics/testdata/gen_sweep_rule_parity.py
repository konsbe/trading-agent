#!/usr/bin/env python3
"""Regenerate sweep_rule_parity.json by running the LIVE Python rule.

    python3 services/data-analyzer/internal/heuristics/testdata/gen_sweep_rule_parity.py

Every case is built as the indicators/macro dicts the actions engine passes to
services/analyst-bot/actions/rules/liquidity_sweep.py:evaluate, run through
that function, and its (action, confluence) recorded. The Go port
(internal/heuristics EvaluateSweepRule) must reproduce every row.
"""
from __future__ import annotations

import itertools
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", "..", "..", "..", ".."))
sys.path.insert(0, os.path.join(REPO, "services", "analyst-bot"))

from actions.rules import liquidity_sweep  # noqa: E402

LEVEL = 100.0

# (label, last_sweep dict or None). bar_close relative to swept_level 100.
SWEEPS = [("no_liq_key", "MISSING"), ("last_sweep_null", None)]
for kind in ("low_sweep", "high_sweep", ""):
    for close_label, bar_close in (("above", 101.5), ("below", 98.5), ("equal", LEVEL), ("close_none", None)):
        SWEEPS.append((f"{kind or 'nokind'}_{close_label}",
                       {"kind": kind, "bar_close": bar_close, "swept_level": LEVEL}))

OBS = [
    ("ob_missing", "MISSING"),
    ("ob_none", {"last_bullish_ob": None, "last_bearish_ob": None}),
    ("ob_bull", {"last_bullish_ob": {"bar_index": 1, "high": 1.0, "low": 0.5, "open": 1.0, "close": 0.6}, "last_bearish_ob": None}),
    ("ob_bear", {"last_bullish_ob": None, "last_bearish_ob": {"bar_index": 2, "high": 1.0, "low": 0.5, "open": 0.6, "close": 1.0}}),
    ("ob_both", {"last_bullish_ob": {"bar_index": 1, "high": 1.0, "low": 0.5, "open": 1.0, "close": 0.6},
                 "last_bearish_ob": {"bar_index": 2, "high": 1.0, "low": 0.5, "open": 0.6, "close": 1.0}}),
]
TRENDS = ["MISSING", "up", "down", "sideways"]
VIX = ["MISSING", "extreme_fear", "elevated", "normal", "complacency"]


def main() -> None:
    cases = []
    for (sl, sweep), (ol, ob), trend, vix in itertools.product(SWEEPS, OBS, TRENDS, VIX):
        indicators: dict = {}
        if sweep != "MISSING":
            indicators["liquidity_sweep_sw3"] = {"value": 1.0, "payload": {"total_sweeps": 1, "last_sweep": sweep}}
        if ob != "MISSING":
            indicators["order_blocks_sw3_imp1.5"] = {"value": 0.0, "payload": ob}
        if trend != "MISSING":
            indicators["trend"] = {"value": 0.0, "payload": {"direction": trend}}
        macro = {} if vix == "MISSING" else {"vix_regime": vix}

        action, _reasons, confluence = liquidity_sweep.evaluate(
            symbol="TEST", alert_type="liquidity_sweep", indicators=indicators, fa={}, macro=macro,
        )

        ls = sweep if isinstance(sweep, dict) else {}
        obd = ob if isinstance(ob, dict) else {}
        cases.append({
            "name": f"{sl}/{ol}/trend_{trend}/vix_{vix}",
            # Inputs in the Go SweepRuleInput shape (Python `or` defaults applied).
            "kind": ls.get("kind") or "",
            "bar_close": ls.get("bar_close"),
            "swept_level": ls.get("swept_level"),
            "bullish_ob": bool(obd.get("last_bullish_ob")),
            "bearish_ob": bool(obd.get("last_bearish_ob")),
            "trend_dir": "" if trend == "MISSING" else trend,
            "vix_regime": macro.get("vix_regime") or "",
            "want_action": action,
            "want_confluence": confluence,
        })

    out = os.path.join(HERE, "sweep_rule_parity.json")
    with open(out, "w") as f:
        json.dump({"generator": "gen_sweep_rule_parity.py", "cases": cases}, f, indent=1)
        f.write("\n")
    print(f"wrote {len(cases)} cases to {out}")


if __name__ == "__main__":
    main()

"""
Actions engine — central dispatcher for the #actions channel.

After an alert fires and has been posted to #alerts, the alert scan job
calls process_alert(). The engine:
  1. Loads technical indicators, FA derived data, and VIX from the DB.
  2. Routes to the appropriate rule module.
  3. Builds the action embed via the formatter.
  4. Sends the embed to #actions via the notifier.

Adding a new alert type:
  - Create actions/rules/<type>.py with an evaluate() function.
  - Register it in RULE_MAP below.
"""
from __future__ import annotations

import logging
from typing import TYPE_CHECKING, Optional

import asyncpg

from actions import formatter as action_formatter
from actions.rules import rsi, bb_squeeze, vix, fa_flip, liquidity_sweep
from db.queries import technical, fundamental as fa_queries
from db.queries import ohlcv

if TYPE_CHECKING:
    from reports.models import AlertEvent
    from notifier.discord.notifier import DiscordNotifier

log = logging.getLogger(__name__)

RULE_MAP = {
    "rsi_oversold":    rsi.evaluate,
    "rsi_overbought":  rsi.evaluate,
    "bb_squeeze":      bb_squeeze.evaluate,
    "vix_elevated":    vix.evaluate,
    "fa_tier_flip":    fa_flip.evaluate,
    "liquidity_sweep": liquidity_sweep.evaluate,
}


async def process_alert(
    alert: "AlertEvent",
    pool: asyncpg.Pool,
    notifier: "DiscordNotifier",
    actions_channel_id: Optional[int],
    min_confluence: int,
    equity_interval: str = "1Day",
) -> None:
    """
    Load context, evaluate the matching rule, and post to #actions.
    Called after an alert has already been posted to #alerts.
    """
    if not actions_channel_id:
        log.debug("actions channel not configured — skipping actions engine")
        return

    rule_fn = RULE_MAP.get(alert.kind)
    if not rule_fn:
        log.debug("no action rule for alert kind=%s", alert.kind)
        return

    symbol = alert.symbol
    exchange = alert.exchange
    interval = alert.interval or equity_interval

    # ── Load context from DB ──────────────────────────────────────────────────
    try:
        indicators = await technical.latest_indicators(pool, symbol, exchange, interval)
    except Exception as exc:
        log.warning("actions engine: technical indicators fetch failed symbol=%s: %s", symbol, exc)
        indicators = {}

    try:
        fa = await fa_queries.latest_derived(pool, symbol)
    except Exception as exc:
        log.warning("actions engine: FA derived fetch failed symbol=%s: %s", symbol, exc)
        fa = {}

    # Inject previous FA tier from alert payload for fa_tier_flip rule
    if alert.kind == "fa_tier_flip":
        fa["_prev_tier"] = (alert.payload or {}).get("prev_tier", "")

    # ── VIX macro context ─────────────────────────────────────────────────────
    macro: dict = {}
    try:
        vix_val = await ohlcv.latest_macro(pool, "VIXCLS")
        if vix_val is not None:
            macro["vix"] = float(vix_val)
            # Classify into regime using fixed thresholds (aligned with config defaults)
            if float(vix_val) > 35:
                macro["vix_regime"] = "extreme_fear"
            elif float(vix_val) > 20:
                macro["vix_regime"] = "elevated"
            elif float(vix_val) < 12:
                macro["vix_regime"] = "complacency"
            else:
                macro["vix_regime"] = "normal"
    except Exception as exc:
        log.debug("actions engine: VIX fetch failed: %s", exc)
        # Fallback to vix_regime from TA indicators if available
        vr = (indicators.get("vix_regime") or {})
        if vr.get("value") is not None:
            macro["vix"] = float(vr["value"])
        regime_payload = (vr.get("payload") or {})
        if regime_payload.get("regime"):
            macro["vix_regime"] = regime_payload["regime"]

    # ── Evaluate rule ─────────────────────────────────────────────────────────
    try:
        action, reasons, confluence = rule_fn(
            symbol=symbol,
            alert_type=alert.kind,
            indicators=indicators,
            fa=fa,
            macro=macro,
        )
    except Exception as exc:
        log.error("actions engine: rule evaluation failed kind=%s symbol=%s: %s", alert.kind, symbol, exc)
        return

    # ── Build and send embed — only post if action is directed (not WATCH/HOLD_WATCH) ──
    watch_only_actions = {"WATCH", "HOLD_WATCH"}
    if action in watch_only_actions:
        log.debug(
            "actions engine: skipping watch-only action kind=%s symbol=%s action=%s",
            alert.kind, symbol, action,
        )
        return

    try:
        embed = action_formatter.build_action_embed(
            symbol=symbol,
            alert_type=alert.kind,
            action=action,
            reasons=reasons,
            confluence=confluence,
            min_confluence=min_confluence,
            indicators=indicators,
            fa=fa,
            macro=macro,
        )
        await notifier.send_action(actions_channel_id, embed)
        log.info(
            "action posted kind=%s symbol=%s action=%s confluence=%d",
            alert.kind, symbol, action, confluence,
        )
    except Exception as exc:
        log.error("actions engine: send failed kind=%s symbol=%s: %s", alert.kind, symbol, exc)

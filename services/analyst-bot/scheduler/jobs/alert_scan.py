"""
Alert scan job — runs every BOT_ALERT_SCAN_INTERVAL seconds.

Checks all symbols for threshold breaches using Redis-deduplicated alert events,
then dispatches each alert to all registered notifiers.

After posting an alert to #alerts, the actions engine is called to post
actionable trade guidance to #actions (if BOT_ACTIONS_ENABLED=true).
"""
from __future__ import annotations

import logging
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from config import BotConfig
    from notifier.base import BaseNotifier
    from notifier.discord.notifier import DiscordNotifier
    from reports.builder import ReportBuilder

log = logging.getLogger(__name__)


class AlertScanJob:
    def __init__(
        self,
        cfg: "BotConfig",
        builder: "ReportBuilder",
        notifiers: "list[BaseNotifier]",
        pool=None,
    ) -> None:
        self._cfg = cfg
        self._builder = builder
        self._notifiers = notifiers
        self._pool = pool  # injected by scheduler for actions engine

    async def run(self) -> None:
        log.debug("running alert scan")
        try:
            alerts = await self._builder.scan_alerts(
                equity_symbols=self._cfg.equity_symbols,
                crypto_symbols=self._cfg.crypto_symbols,
                rsi_oversold=self._cfg.bot_rsi_oversold,
                rsi_overbought=self._cfg.bot_rsi_overbought,
                vix_alert_threshold=self._cfg.bot_vix_alert_threshold,
                alert_cooldown_secs=self._cfg.bot_alert_cooldown_secs,
            )
            if not alerts:
                return
            log.info("alert scan found %d alerts", len(alerts))
            for alert in alerts:
                # Post to #alerts via all registered notifiers
                for notifier in self._notifiers:
                    try:
                        await notifier.send_alert(alert)
                    except Exception as exc:
                        log.error("notifier %s failed alert: %s", notifier.name, exc)

                # Post actionable guidance to #actions (Discord only)
                if self._cfg.bot_actions_enabled and self._pool is not None:
                    from actions.engine import process_alert
                    discord_notifier: "DiscordNotifier | None" = next(
                        (n for n in self._notifiers if n.name == "discord"), None
                    )
                    if discord_notifier is not None:
                        try:
                            await process_alert(
                                alert=alert,
                                pool=self._pool,
                                notifier=discord_notifier,
                                actions_channel_id=self._cfg.discord_actions_channel_id,
                                min_confluence=self._cfg.bot_actions_min_confluence,
                                equity_interval=self._cfg.bot_equity_interval,
                            )
                        except Exception as exc:
                            log.error("actions engine failed kind=%s symbol=%s: %s", alert.kind, alert.symbol, exc)

        except Exception as exc:
            log.error("alert scan job failed: %s", exc)

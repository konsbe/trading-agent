"""
Alert scan job — runs every BOT_ALERT_SCAN_INTERVAL seconds.

Checks all symbols for threshold breaches using Redis-deduplicated alert events,
then dispatches each alert to all registered notifiers.
"""
from __future__ import annotations

import logging
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from config import BotConfig
    from notifier.base import BaseNotifier
    from reports.builder import ReportBuilder

log = logging.getLogger(__name__)


class AlertScanJob:
    def __init__(
        self,
        cfg: "BotConfig",
        builder: "ReportBuilder",
        notifiers: "list[BaseNotifier]",
    ) -> None:
        self._cfg = cfg
        self._builder = builder
        self._notifiers = notifiers

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
                for notifier in self._notifiers:
                    try:
                        await notifier.send_alert(alert)
                    except Exception as exc:
                        log.error("notifier %s failed alert: %s", notifier.name, exc)
        except Exception as exc:
            log.error("alert scan job failed: %s", exc)

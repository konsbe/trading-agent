"""
Alert scan job — runs every BOT_ALERT_SCAN_INTERVAL seconds.

Detects alert ONSETS (ReportBuilder.scan_alerts) and dispatches each to all
registered notifiers.

Each onset is stored at most once: before posting, the job claims its
fired_alerts row, unique per symbol, kind and bar_date (migration 031). A
conflicting claim means the onset was already posted, so it is skipped; a post
no notifier confirms releases the claim so the next scan retries it; a claim
that cannot be written (database down) is not posted. Only a confirmed post
keeps its row, starts the backstop cooldown and reaches the actions engine
(#actions, if BOT_ACTIONS_ENABLED=true).
"""
from __future__ import annotations

import logging
from datetime import date, datetime, timezone
from typing import TYPE_CHECKING

from db import cache as _cache
from db.queries import alerts as alerts_q
from reports.alert_severity import exchange_type, fired_alert_severity
from reports.builder import FA_TIER_MEMORY_SECS

if TYPE_CHECKING:
    from config import BotConfig
    from notifier.base import BaseNotifier
    from notifier.discord.notifier import DiscordNotifier
    from reports.builder import ReportBuilder
    from reports.models import AlertEvent

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
                vix_alert_threshold=self._cfg.bot_vix_alert_threshold,
            )
            if not alerts:
                return
            log.info("alert scan found %d alerts", len(alerts))
            for alert in alerts:
                claim = await self._claim(alert)
                if claim is _ALREADY_POSTED:
                    log.info("onset already posted; skipped kind=%s symbol=%s bar=%s",
                             alert.kind, alert.symbol, alert.bar_date)
                    await _cache.set_flag(alert.cache_key, self._cfg.bot_alert_cooldown_secs)
                    continue
                if claim is _CLAIM_FAILED:
                    continue
                # Post to #alerts via all registered notifiers
                posted = False
                for notifier in self._notifiers:
                    try:
                        posted = await notifier.send_alert(alert) or posted
                    except Exception as exc:
                        log.error("notifier %s failed alert: %s", notifier.name, exc)

                if not posted:
                    log.warning(
                        "alert not posted by any notifier; claim released, retried next scan kind=%s symbol=%s",
                        alert.kind, alert.symbol,
                    )
                    await self._release(claim)
                    continue
                await self._record_posted(alert)

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

    async def _claim(self, alert: "AlertEvent"):
        """The claimed row id; _ALREADY_POSTED when this onset has a row;
        _CLAIM_FAILED when it cannot be written; None without a pool."""
        if self._pool is None:
            log.warning("no DB pool; fired_alerts not written kind=%s symbol=%s", alert.kind, alert.symbol)
            return None
        if not alert.bar_date:
            log.error("alert without bar_date not posted kind=%s symbol=%s", alert.kind, alert.symbol)
            return _CLAIM_FAILED
        try:
            row_id = await alerts_q.claim_fired_alert(
                self._pool,
                symbol=alert.symbol,
                exchange_type=exchange_type(alert),
                alert_type=alert.kind,
                interval=alert.interval,
                value=alert.value,
                severity=fired_alert_severity(alert),
                message=alert.message,
                fired_at=datetime.now(timezone.utc),
                bar_date=date.fromisoformat(alert.bar_date),
            )
        except Exception as exc:
            log.error("fired_alerts claim failed; not posted kind=%s symbol=%s: %s", alert.kind, alert.symbol, exc)
            return _CLAIM_FAILED
        return _ALREADY_POSTED if row_id is None else row_id

    async def _release(self, claim) -> None:
        if not isinstance(claim, int) or self._pool is None:
            return
        try:
            await alerts_q.release_fired_alert(self._pool, claim)
        except Exception as exc:
            log.error("fired_alerts claim %s not released: %s", claim, exc)

    async def _record_posted(self, alert: "AlertEvent") -> None:
        """Backstop cooldown and the FA tier memory, after a confirmed post."""
        await _cache.set_flag(alert.cache_key, self._cfg.bot_alert_cooldown_secs)
        new_tier = alert.payload.get("new_tier") if alert.kind == "fa_tier_flip" else None
        if new_tier:
            await _cache.set(f"fa_tier:{alert.symbol}", new_tier, FA_TIER_MEMORY_SECS)


_ALREADY_POSTED = object()
_CLAIM_FAILED = object()

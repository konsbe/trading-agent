"""
Discord implementation of BaseNotifier.

Sends messages to Discord channels via the bot's channel references.
The bot client is injected on construction — this notifier is only usable
after the bot is connected (on_ready).

For platforms without an interactive bot component (Telegram, e-mail, etc.)
the notifier would use a webhook or API client instead.
"""
from __future__ import annotations

import logging
from typing import Optional

import discord

from notifier.base import BaseNotifier
from notifier.discord import formatter
from reports.models import AlertEvent, DailyReport, SymbolReport

log = logging.getLogger(__name__)

# Discord allows max 10 embeds per message
_MAX_EMBEDS_PER_MSG = 10


class DiscordNotifier(BaseNotifier):
    def __init__(
        self,
        bot: discord.Bot,
        daily_report_channel_id: Optional[int],
        alerts_channel_id: Optional[int],
    ) -> None:
        self._bot = bot
        self._daily_report_channel_id = daily_report_channel_id
        self._alerts_channel_id = alerts_channel_id

    @property
    def name(self) -> str:
        return "discord"

    async def send_daily_report(self, report: DailyReport) -> None:
        channel = self._get_channel(self._daily_report_channel_id)
        if not channel:
            log.warning("daily_report_channel_id not set or channel not found")
            return
        try:
            embeds = formatter.daily_report_embeds(report)
            await self._send_embed_batch(channel, embeds)
            log.info("daily report sent to Discord (%d embeds)", len(embeds))
        except Exception as exc:
            log.error("failed to send daily report to Discord: %s", exc)

    async def send_symbol_report(self, report: SymbolReport) -> None:
        """Used by slash commands — the command handler passes the channel directly."""
        raise NotImplementedError(
            "Discord slash commands send via ApplicationContext; "
            "use formatter.symbol_report_embeds() directly in the command handler."
        )

    async def send_alert(self, alert: AlertEvent) -> None:
        channel = self._get_channel(self._alerts_channel_id)
        if not channel:
            log.debug("alerts_channel_id not set — alert not sent: %s", alert.kind)
            return
        try:
            embed = formatter.alert_embed(alert)
            await channel.send(embed=embed)
            log.info("alert sent kind=%s symbol=%s", alert.kind, alert.symbol)
        except Exception as exc:
            log.error("failed to send alert to Discord: %s", exc)

    # ── Helpers ───────────────────────────────────────────────────────────────

    def _get_channel(self, channel_id: Optional[int]) -> Optional[discord.TextChannel]:
        if not channel_id:
            return None
        channel = self._bot.get_channel(channel_id)
        if not isinstance(channel, discord.TextChannel):
            log.warning("channel %s not found or not a text channel", channel_id)
            return None
        return channel

    async def _send_embed_batch(
        self,
        channel: discord.TextChannel,
        embeds: list[discord.Embed],
    ) -> None:
        """Split embeds into batches of 10 (Discord limit) and send each batch."""
        for i in range(0, len(embeds), _MAX_EMBEDS_PER_MSG):
            batch = embeds[i : i + _MAX_EMBEDS_PER_MSG]
            await channel.send(embeds=batch)

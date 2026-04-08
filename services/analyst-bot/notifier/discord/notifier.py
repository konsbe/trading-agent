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

# Discord allows max 10 embeds per message and 6000 total characters per message.
_MAX_EMBEDS_PER_MSG = 10
_MAX_EMBED_CHARS_PER_MSG = 6000


def _embed_char_count(embed: discord.Embed) -> int:
    """Approximate Discord's character count for a single embed."""
    total = 0
    if embed.title:
        total += len(embed.title)
    if embed.description:
        total += len(embed.description)
    if embed.footer and embed.footer.text:
        total += len(embed.footer.text)
    if embed.author and embed.author.name:
        total += len(embed.author.name)
    for field in embed.fields:
        total += len(field.name or "") + len(field.value or "")
    return total


def _split_embed_batches(embeds: list[discord.Embed]) -> list[list[discord.Embed]]:
    """Split embeds into sendable batches respecting both Discord limits."""
    batches: list[list[discord.Embed]] = []
    current: list[discord.Embed] = []
    current_chars = 0
    for embed in embeds:
        size = _embed_char_count(embed)
        if current and (
            len(current) >= _MAX_EMBEDS_PER_MSG
            or current_chars + size > _MAX_EMBED_CHARS_PER_MSG
        ):
            batches.append(current)
            current = []
            current_chars = 0
        current.append(embed)
        current_chars += size
    if current:
        batches.append(current)
    return batches


class DiscordNotifier(BaseNotifier):
    def __init__(
        self,
        bot: discord.Bot,
        daily_report_channel_id: Optional[int],
        alerts_channel_id: Optional[int],
        actions_channel_id: Optional[int] = None,
    ) -> None:
        self._bot = bot
        self._daily_report_channel_id = daily_report_channel_id
        self._alerts_channel_id = alerts_channel_id
        self._actions_channel_id = actions_channel_id

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

    async def send_action(self, channel_id: Optional[int], embed: discord.Embed) -> None:
        """Send an action embed to the #actions channel."""
        channel = self._get_channel(channel_id or self._actions_channel_id)
        if not channel:
            log.debug("actions channel not found — action embed not sent")
            return
        try:
            await channel.send(embed=embed)
        except Exception as exc:
            log.error("failed to send action to Discord: %s", exc)

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
        """Split embeds into batches respecting Discord's 10-embed and 6000-char limits."""
        for batch in _split_embed_batches(embeds):
            await channel.send(embeds=batch)

"""
Telegram notifier — PLACEHOLDER (not yet implemented).

To implement:
  1. pip install python-telegram-bot (async version >=20)
  2. Create a Telegram Bot via @BotFather — get TELEGRAM_BOT_TOKEN + TELEGRAM_CHAT_ID
  3. Implement formatter.py to build Telegram MarkdownV2 messages from report models
  4. Implement send_daily_report / send_symbol_report / send_alert below
  5. Register this notifier in main.py alongside DiscordNotifier

The formatter should output plain text / MarkdownV2 strings rather than
discord.Embed objects. Everything else in the pipeline stays the same because
this notifier only receives platform-agnostic report models.
"""
from __future__ import annotations

import logging

from notifier.base import BaseNotifier
from reports.models import AlertEvent, DailyReport, SymbolReport

log = logging.getLogger(__name__)


class TelegramNotifier(BaseNotifier):
    @property
    def name(self) -> str:
        return "telegram"

    async def send_daily_report(self, report: DailyReport) -> None:
        log.info("TelegramNotifier.send_daily_report — not implemented yet")

    async def send_symbol_report(self, report: SymbolReport) -> None:
        log.info("TelegramNotifier.send_symbol_report — not implemented yet")

    async def send_alert(self, alert: AlertEvent) -> None:
        log.info("TelegramNotifier.send_alert — not implemented yet")

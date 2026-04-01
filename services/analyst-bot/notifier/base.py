"""
BaseNotifier — abstract interface for all messaging platform implementations.

Any new platform (Telegram, X, Facebook, e-mail, Slack …) must implement this
interface. The scheduler jobs and bot commands call this interface, never any
platform-specific code directly.

To add a new platform:
  1. Create services/analyst-bot/notifier/<platform>/__init__.py
  2. Create services/analyst-bot/notifier/<platform>/notifier.py
     — Subclass BaseNotifier and implement all abstract methods
  3. Create services/analyst-bot/notifier/<platform>/formatter.py
     — Converts report models to that platform's message format
  4. Add the platform's notifier to main.py notifier list
"""
from __future__ import annotations

from abc import ABC, abstractmethod

from reports.models import AlertEvent, DailyReport, SymbolReport


class BaseNotifier(ABC):
    """
    Every notifier must implement these three methods.
    A notifier should never raise — it should log errors and return gracefully
    so that other notifiers in the chain still execute.
    """

    @abstractmethod
    async def send_daily_report(self, report: DailyReport) -> None:
        """Post the full daily market report to the appropriate channel/recipient."""
        ...

    @abstractmethod
    async def send_symbol_report(self, report: SymbolReport) -> None:
        """Post a single-symbol on-demand analysis report."""
        ...

    @abstractmethod
    async def send_alert(self, alert: AlertEvent) -> None:
        """Post a single threshold-breach alert."""
        ...

    @property
    @abstractmethod
    def name(self) -> str:
        """Human-readable platform name for logging (e.g. 'discord', 'telegram')."""
        ...

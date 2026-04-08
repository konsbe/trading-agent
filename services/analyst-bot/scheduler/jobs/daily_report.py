"""
Daily report job — builds a full market report and dispatches it to all notifiers.

Runs on a cron schedule (default 07:00 UTC via BOT_DAILY_REPORT_CRON).
Also triggered on-demand via the /report slash command.
"""
from __future__ import annotations

import logging
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from config import BotConfig
    from notifier.base import BaseNotifier
    from reports.builder import ReportBuilder

log = logging.getLogger(__name__)


class DailyReportJob:
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
        log.info("running daily report job")
        try:
            report = await self._builder.build_daily_report(
                self._cfg.equity_symbols,
                self._cfg.crypto_symbols,
                macro_intel_equity_symbols=self._cfg.equity_symbols,
            )
            for notifier in self._notifiers:
                try:
                    await notifier.send_daily_report(report)
                except NotImplementedError:
                    pass  # Some notifiers (e.g. Discord slash-only) skip scheduled sends
                except Exception as exc:
                    log.error("notifier %s failed daily report: %s", notifier.name, exc)
        except Exception as exc:
            log.error("daily report job failed: %s", exc)

"""
APScheduler setup — AsyncIOScheduler running in the bot's event loop.

Jobs are registered here and scheduled using cron or interval triggers.
Adding a new scheduled job:
  1. Create services/analyst-bot/scheduler/jobs/<name>.py with a `run()` coroutine
  2. Import and register it in `build_scheduler()` below
"""
from __future__ import annotations

import logging
from typing import TYPE_CHECKING

from apscheduler.schedulers.asyncio import AsyncIOScheduler
from apscheduler.triggers.cron import CronTrigger
from apscheduler.triggers.interval import IntervalTrigger

if TYPE_CHECKING:
    import asyncpg

    from config import BotConfig
    from notifier.base import BaseNotifier
    from reports.builder import ReportBuilder

log = logging.getLogger(__name__)


def build_scheduler(
    cfg: "BotConfig",
    builder: "ReportBuilder",
    notifiers: "list[BaseNotifier]",
    db_pool: "asyncpg.Pool | None" = None,
) -> AsyncIOScheduler:
    """
    Construct and configure the scheduler. Does NOT start it yet —
    `scheduler.start()` is called in TradingBot.setup_hook().
    """
    from scheduler.jobs.daily_report import DailyReportJob
    from scheduler.jobs.alert_scan import AlertScanJob

    scheduler = AsyncIOScheduler(timezone="UTC")

    daily_job = DailyReportJob(cfg, builder, notifiers)
    alert_job = AlertScanJob(cfg, builder, notifiers, pool=db_pool)

    # Daily market report — cron from .env (default 07:00 UTC)
    try:
        parts = cfg.bot_daily_report_cron.split()
        scheduler.add_job(
            daily_job.run,
            trigger=CronTrigger(
                minute=parts[0],
                hour=parts[1],
                day=parts[2],
                month=parts[3],
                day_of_week=parts[4],
                timezone="UTC",
            ),
            id="daily_report",
            name="Daily Market Report",
            replace_existing=True,
        )
        log.info("daily report scheduled: %s", cfg.bot_daily_report_cron)
    except Exception as exc:
        log.error("failed to schedule daily report: %s", exc)

    # Alert scanner — interval from .env (default 300s)
    scheduler.add_job(
        alert_job.run,
        trigger=IntervalTrigger(seconds=cfg.bot_alert_scan_interval),
        id="alert_scan",
        name="Alert Scanner",
        replace_existing=True,
    )
    log.info("alert scan scheduled every %ds", cfg.bot_alert_scan_interval)

    if (
        db_pool is not None
        and cfg.bot_fomc_narrative_enable
        and cfg.openai_api_key.strip()
        and cfg.fomc_statement_url.strip()
    ):
        from scheduler.jobs.fomc_narrative import FomcNarrativeJob

        fomc_job = FomcNarrativeJob(cfg, db_pool)
        try:
            parts = cfg.bot_fomc_narrative_cron.split()
            scheduler.add_job(
                fomc_job.run,
                trigger=CronTrigger(
                    minute=parts[0],
                    hour=parts[1],
                    day=parts[2],
                    month=parts[3],
                    day_of_week=parts[4],
                    timezone="UTC",
                ),
                id="fomc_narrative",
                name="FOMC narrative (OpenAI)",
                replace_existing=True,
            )
            log.info("FOMC narrative scheduled: %s", cfg.bot_fomc_narrative_cron)
        except Exception as exc:
            log.error("failed to schedule FOMC narrative: %s", exc)

    return scheduler

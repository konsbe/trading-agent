"""
Centralised configuration for analyst-bot.

All settings are read from environment variables (or .env via pydantic-settings).
The .env file is shared with data-ingestion and data-analyzer, so unknown fields
from those services are silently ignored.

List-valued settings (symbols) are stored as plain comma-separated strings and
exposed as list properties. This avoids the pydantic-settings v2 behaviour of
trying json.loads() on any List[str] field read from env vars.

Adding a new messaging platform (Telegram, X, etc.) means adding a new section
here — the rest of the codebase just reads cfg.<PLATFORM>_*.
"""
from __future__ import annotations

from typing import Optional

from pydantic_settings import BaseSettings, SettingsConfigDict


def _csv(value: str) -> list[str]:
    return [s.strip() for s in value.split(",") if s.strip()]


class BotConfig(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",  # shared .env has many unrelated keys
    )

    # ── Shared infrastructure ─────────────────────────────────────────────────
    database_url: str = "postgres://postgres:postgres@localhost:5432/trading?sslmode=disable"
    redis_url: str = "redis://localhost:6379/0"
    log_level: str = "INFO"

    # ── Symbols & intervals — stored as CSV strings, parsed via properties ────
    # pydantic-settings v2 tries json.loads() on List[str] fields; using str
    # avoids that entirely and accepts plain BOT_EQUITY_SYMBOLS=AAPL,MSFT,SPY
    bot_equity_symbols: str = "AAPL,MSFT,SPY"
    bot_crypto_symbols: str = "BTCUSDT,ETHUSDT"
    bot_equity_interval: str = "1Day"
    bot_crypto_interval: str = "1d"
    # Same env as macro-analysis MARKET_CYCLE_SYMBOL — benchmark for cycle + /analyze RS context
    market_cycle_symbol: str = "SPY"

    @property
    def equity_symbols(self) -> list[str]:
        return _csv(self.bot_equity_symbols)

    @property
    def crypto_symbols(self) -> list[str]:
        return _csv(self.bot_crypto_symbols)

    # ── Discord ───────────────────────────────────────────────────────────────
    discord_bot_token: str = ""
    discord_guild_id: Optional[int] = None
    discord_daily_report_channel_id: Optional[int] = None
    discord_alerts_channel_id: Optional[int] = None
    discord_commands_channel_id: Optional[int] = None

    # ── Telegram (future — not yet wired) ────────────────────────────────────
    telegram_bot_token: str = ""
    telegram_chat_id: str = ""

    # ── Scheduler ─────────────────────────────────────────────────────────────
    bot_daily_report_cron: str = "0 7 * * *"    # 07:00 UTC every day
    bot_weekly_digest_cron: str = "0 8 * * 1"   # Monday 08:00 UTC
    bot_alert_scan_interval: int = 300           # seconds between alert scans

    # Send the daily report automatically whenever the bot starts up.
    # Useful after restarts/deploys so the channel always has a fresh report.
    # Set to false to suppress the startup send (e.g. in dev).
    bot_report_on_startup: bool = True
    # Seconds to wait after on_ready before sending the startup report.
    # Gives the scheduler and connection pool time to fully settle.
    bot_report_on_startup_delay: int = 30

    # ── Alert thresholds ─────────────────────────────────────────────────────
    bot_rsi_oversold: float = 30.0
    bot_rsi_overbought: float = 70.0
    bot_vix_alert_threshold: float = 25.0
    bot_bb_squeeze_alert: bool = True
    bot_alert_cooldown_secs: int = 14400         # 4 hours per alert key

    # ── Redis cache TTLs (seconds) ────────────────────────────────────────────
    bot_cache_price_ttl: int = 300               # 5 min
    bot_cache_analyze_ttl: int = 600             # 10 min
    bot_cache_daily_report_ttl: int = 3600       # 1 hour

    # ── Report formatting ─────────────────────────────────────────────────────
    bot_news_headlines_limit: int = 5
    bot_report_max_symbols: int = 10

    # ── Market operations (mo_reference_snapshot + per-symbol ATR% / volume vs median) ─
    bot_market_ops_enable: bool = True
    bot_market_ops_volume_lookback: int = 60
    bot_market_ops_atr_period: int = 14
    bot_market_ops_atr_pct_elevated: float = 3.0
    bot_market_ops_volume_ratio_elevated: float = 1.8
    # VIX bands for market-ops display (align with MARKET_OPS_VIX_* on market-operations worker).
    bot_market_ops_vix_low_max: float = 12.0    # aligned with TECHNICAL_VIX_COMPLACENCY_THRESHOLD
    bot_market_ops_vix_normal_max: float = 20.0  # aligned with TECHNICAL_VIX_ELEVATED_THRESHOLD
    bot_market_ops_vix_elevated_max: float = 35.0  # aligned with TECHNICAL_VIX_FEAR_THRESHOLD

    # ── Optional FOMC / policy narrative (OpenAI → narrative_scores) ───────────
    bot_fomc_narrative_enable: bool = False
    bot_fomc_narrative_cron: str = "0 18 * * 3"  # Wednesday 18:00 UTC (tune around FOMC)
    openai_api_key: str = ""
    openai_model: str = "gpt-4o-mini"
    fomc_statement_url: str = ""  # HTML page URL; text is stripped server-side


def load() -> BotConfig:
    return BotConfig()

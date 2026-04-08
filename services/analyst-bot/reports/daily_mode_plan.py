"""
Resolve /report slash-command mode → equity list, crypto list, and optional FRED/dashboard add-ons.
"""
from __future__ import annotations

from dataclasses import dataclass, field

from config import BotConfig


def _csv(value: str) -> list[str]:
    return [s.strip() for s in value.split(",") if s.strip()]


def clean_equity_symbols(raw: list[str]) -> list[str]:
    """Drop invalid tickers (spaces, slashes); map TSMC → TSM."""
    out: list[str] = []
    seen: set[str] = set()
    for s in raw:
        x = s.strip().upper()
        if not x or " " in x or "/" in x:
            continue
        if x == "TSMC":
            x = "TSM"
        if x in seen:
            continue
        seen.add(x)
        out.append(x)
    return out


def normalize_crypto_symbol(token: str) -> str | None:
    """Map bare coin tickers to Binance-style USDT pairs. Returns None to skip."""
    t = token.strip().upper()
    if not t or t == "MUTM":
        return None
    aliases = {
        "TIA": "TIAUSDT",
        "LINK": "LINKUSDT",
        "KSM": "KSMUSDT",
        "TAO": "TAOUSDT",
        "RNDR": "RENDERUSDT",
        "SOL": "SOLUSDT",
        "BTC": "BTCUSDT",
        "ETH": "ETHUSDT",
    }
    if t in aliases:
        return aliases[t]
    if t.endswith("USDT"):
        return t
    if len(t) <= 6 and t.isalnum():
        return f"{t}USDT"
    return None


def normalize_crypto_list(tokens: list[str]) -> list[str]:
    out: list[str] = []
    seen: set[str] = set()
    for tok in tokens:
        n = normalize_crypto_symbol(tok)
        if n and n not in seen:
            seen.add(n)
            out.append(n)
    return out


@dataclass
class DailyReportModePlan:
    equity_symbols: list[str]
    crypto_symbols: list[str]
    mode_tag: str = ""
    #: Latest-value table after per-symbol section (commodities / macro_fred)
    fred_panel_ids: list[str] = field(default_factory=list)
    include_dashboard: bool = False


def plan_daily_report(mode: str, cfg: BotConfig) -> DailyReportModePlan:
    mode = (mode or "standard").strip().lower()
    full_eq = cfg.equity_symbols
    full_cr = cfg.crypto_symbols

    if mode in ("standard", "all"):
        return DailyReportModePlan(full_eq, full_cr, "")

    if mode == "etfs":
        eq = clean_equity_symbols(_csv(cfg.bot_report_etf_symbols))
        return DailyReportModePlan(eq, [], "ETFs")

    if mode == "commodities":
        eq = clean_equity_symbols(_csv(cfg.bot_report_commodity_equity_symbols))
        fred = _csv(cfg.bot_report_commodity_fred_series)
        return DailyReportModePlan(eq, [], "Commodities", fred_panel_ids=fred)

    if mode == "macro_fred":
        fred = _csv(cfg.bot_report_macro_fred_series)
        return DailyReportModePlan([], [], "FRED series", fred_panel_ids=fred)

    if mode == "crypto":
        if cfg.bot_report_crypto_symbols.strip():
            raw = _csv(cfg.bot_report_crypto_symbols)
        else:
            raw = list(full_cr)
        return DailyReportModePlan([], normalize_crypto_list(raw), "Crypto")

    if mode == "equity":
        if cfg.bot_report_equity_symbols.strip():
            eq = clean_equity_symbols(_csv(cfg.bot_report_equity_symbols))
        else:
            eq = list(full_eq)
        return DailyReportModePlan(eq, [], "Equities")

    if mode == "dashboard":
        return DailyReportModePlan([], [], "Dashboard", include_dashboard=True)

    return DailyReportModePlan(full_eq, full_cr, "")

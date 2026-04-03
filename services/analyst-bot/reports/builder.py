"""
ReportBuilder — queries the DB and assembles platform-agnostic report models.

All DB queries are async. Redis cache is checked before hitting the DB for
expensive operations. The builder is stateless — pass the pool and cache client
on each call.

This is the only module that knows about DB schema details. Notifiers and bot
cogs receive pre-built model objects and never query the DB directly.
"""
from __future__ import annotations

import json
import logging
from datetime import datetime, timezone
from typing import Optional

import asyncpg

from db import cache as _cache
from db.queries import fundamental, macro_intel, news, ohlcv, sentiment, technical
from reports.models import (
    AdditionalAnalysisSnapshot,
    AlertEvent,
    AnalyzeContextSnapshot,
    DailyReport,
    EconomicCalendarBrief,
    EarningsCalendarBrief,
    FundamentalSnapshot,
    MacroIntelSnapshot,
    MacroSnapshot,
    MarketCycleSnapshot,
    MarketOpsSlice,
    NewsHeadline,
    PriceSnapshot,
    SentimentSnapshot,
    SymbolReport,
    TechnicalSnapshot,
)

log = logging.getLogger(__name__)


def _im_branch(im: dict, key: str) -> dict:
    v = im.get(key)
    return v if isinstance(v, dict) else {}


def _fill_roll_branch(snap: AdditionalAnalysisSnapshot, d: dict, prefix: str) -> None:
    """prefix: bond_equity | oil_equity | vix_equity — sets corr, regime, label, insufficient."""
    ins = bool(d.get("insufficient_data"))
    setattr(snap, f"{prefix}_insufficient", ins)
    if d.get("correlation_60d") is not None:
        setattr(snap, f"{prefix}_corr_60d", float(d["correlation_60d"]))
    r = d.get("regime")
    if isinstance(r, str):
        setattr(snap, f"{prefix}_regime", r)
    lbl = d.get("label")
    if isinstance(lbl, str):
        setattr(snap, f"{prefix}_label", lbl)
    if prefix == "bond_equity" and d.get("observations_used") is not None:
        snap.bond_equity_observations = int(d["observations_used"])


def _parse_additional_snapshot(raw: dict) -> Optional[AdditionalAnalysisSnapshot]:
    """Map aa_reference_snapshot payload → model."""
    im = raw.get("intermarket") if isinstance(raw.get("intermarket"), dict) else {}
    be = _im_branch(im, "bond_equity_60d")
    oil = _im_branch(im, "oil_equity_60d")
    vix = _im_branch(im, "vix_equity_60d")
    seas = raw.get("seasonality") if isinstance(raw.get("seasonality"), dict) else {}
    pres = raw.get("presidential_cycle") if isinstance(raw.get("presidential_cycle"), dict) else {}

    snap = AdditionalAnalysisSnapshot()
    _fill_roll_branch(snap, be, "bond_equity")
    _fill_roll_branch(snap, oil, "oil_equity")
    _fill_roll_branch(snap, vix, "vix_equity")

    if isinstance(seas.get("month_name"), str):
        snap.seasonality_month_name = seas["month_name"]
    if isinstance(seas.get("bias"), str):
        snap.seasonality_bias = seas["bias"]
    if isinstance(seas.get("note"), str):
        snap.seasonality_note = seas["note"]
    if pres.get("cycle_year") is not None:
        snap.presidential_cycle_year = int(pres["cycle_year"])
    if isinstance(pres.get("label"), str):
        snap.presidential_label = pres["label"]
    if isinstance(pres.get("note"), str):
        snap.presidential_note = pres["note"]

    mods = raw.get("reference_modules")
    if isinstance(mods, dict):
        for k in sorted(mods.keys()):
            ent = mods[k]
            if isinstance(ent, dict):
                snap.reference_coverage_lines.append(
                    f"{k}: {ent.get('status', '—')} — {ent.get('hint', '')}"
                )
    return snap


def _format_additional_summary_line(aa: dict) -> Optional[str]:
    parts: list[str] = []
    im = aa.get("intermarket") if isinstance(aa.get("intermarket"), dict) else {}
    for key, label in (
        ("bond_equity_60d", "bond–equity"),
        ("oil_equity_60d", "oil–equity"),
        ("vix_equity_60d", "VIX–equity"),
    ):
        br = _im_branch(im, key)
        if not br.get("insufficient_data") and br.get("correlation_60d") is not None:
            reg = br.get("regime") or "—"
            parts.append(f"{label} 60d ρ≈{float(br['correlation_60d']):+.2f} ({reg})")
    seas = aa.get("seasonality")
    if isinstance(seas, dict) and seas.get("month_name"):
        parts.append(f"{seas['month_name']}: {seas.get('bias', '—')}")
    pres = aa.get("presidential_cycle")
    if isinstance(pres, dict) and pres.get("cycle_year") is not None:
        parts.append(f"election cycle yr{pres['cycle_year']} ({pres.get('label', '—')})")
    return " · ".join(parts) if parts else None


class ReportBuilder:
    def __init__(
        self,
        pool: asyncpg.Pool,
        equity_interval: str = "1Day",
        crypto_interval: str = "1d",
        news_limit: int = 5,
        price_cache_ttl: int = 300,
        analyze_cache_ttl: int = 600,
        benchmark_symbol: str = "SPY",
        *,
        market_ops_enable: bool = True,
        market_ops_volume_lookback: int = 60,
        market_ops_atr_period: int = 14,
        market_ops_atr_pct_elevated: float = 3.0,
        market_ops_volume_ratio_elevated: float = 1.8,
        market_ops_vix_low_max: float = 15.0,
        market_ops_vix_normal_max: float = 25.0,
        market_ops_vix_elevated_max: float = 35.0,
    ) -> None:
        self._pool = pool
        self._equity_interval = equity_interval
        self._crypto_interval = crypto_interval
        self._news_limit = news_limit
        self._price_cache_ttl = price_cache_ttl
        self._analyze_cache_ttl = analyze_cache_ttl
        self._benchmark_symbol = (benchmark_symbol or "SPY").strip().upper() or "SPY"
        self._market_ops_enable = market_ops_enable
        self._market_ops_volume_lookback = max(5, market_ops_volume_lookback)
        self._market_ops_atr_period = max(1, market_ops_atr_period)
        self._market_ops_atr_pct_elevated = market_ops_atr_pct_elevated
        self._market_ops_volume_ratio_elevated = market_ops_volume_ratio_elevated
        self._market_ops_vix_low_max = market_ops_vix_low_max
        self._market_ops_vix_normal_max = market_ops_vix_normal_max
        self._market_ops_vix_elevated_max = market_ops_vix_elevated_max

    # ── Public entry points ───────────────────────────────────────────────────

    async def build_symbol_report(
        self,
        symbol: str,
        asset_type: str = "equity",
        use_cache: bool = True,
    ) -> SymbolReport:
        cache_key = f"analyze:{symbol}:{asset_type}"
        if use_cache:
            cached = await _cache.get_json(cache_key)
            if cached:
                log.debug("cache hit %s", cache_key)
                return self._deserialise_symbol_report(cached)

        report = SymbolReport(symbol=symbol, asset_type=asset_type)
        report.price = await self._build_price(symbol, asset_type)
        report.technical = await self._build_technical(symbol, asset_type)
        if asset_type == "equity":
            report.fundamental = await self._build_fundamental(symbol)
        report.sentiment = await self._build_sentiment(symbol)
        report.news = await self._build_news(symbol, asset_type)
        report.analyze_context = await self._build_analyze_context(symbol, asset_type)
        report.market_ops = await self._build_market_ops_slice(
            symbol, asset_type, report.price, report.technical
        )

        if use_cache:
            await _cache.set(cache_key, self._serialise_symbol_report(report), self._analyze_cache_ttl)

        return report

    async def build_market_ops_view(
        self,
        symbol: Optional[str] = None,
        asset_type: str = "equity",
    ) -> Optional[MarketOpsSlice]:
        """Latest global snapshot; optional per-symbol execution strip for /marketops."""
        if not self._market_ops_enable:
            return None
        sym = symbol.strip().upper() if symbol else None
        price = await self._build_price(sym, asset_type) if sym else None
        tech = await self._build_technical(sym, asset_type) if sym else None
        return await self._build_market_ops_slice(
            sym or "", asset_type, price, tech, include_symbol=bool(sym)
        )

    async def build_daily_report(
        self,
        equity_symbols: list[str],
        crypto_symbols: list[str],
    ) -> DailyReport:
        report = DailyReport(generated_at=datetime.now(timezone.utc))
        for sym in equity_symbols:
            sr = await self.build_symbol_report(sym, "equity", use_cache=False)
            report.symbols.append(sr)
        for sym in crypto_symbols:
            sr = await self.build_symbol_report(sym, "crypto", use_cache=False)
            report.symbols.append(sr)
        report.macro = await self._build_macro()
        report.macro_intel = await self._build_macro_intel(equity_symbols)
        return report

    async def scan_alerts(
        self,
        equity_symbols: list[str],
        crypto_symbols: list[str],
        rsi_oversold: float = 30.0,
        rsi_overbought: float = 70.0,
        vix_alert_threshold: float = 25.0,
        alert_cooldown_secs: int = 14400,
    ) -> list[AlertEvent]:
        alerts: list[AlertEvent] = []

        async def _check(symbol: str, asset_type: str) -> None:
            exchange = "equity" if asset_type == "equity" else "binance"
            interval = self._equity_interval if asset_type == "equity" else self._crypto_interval
            indicators = await technical.latest_indicators(self._pool, symbol, exchange, interval)

            # ── RSI ──────────────────────────────────────────────────────────
            rsi_key = f"rsi_{14}"
            if rsi_key in indicators:
                rsi_val = indicators[rsi_key]["value"]
                if rsi_val is not None:
                    if rsi_val < rsi_oversold:
                        ck = f"alert:rsi_oversold:{symbol}:{interval}"
                        if not await _cache.exists(ck):
                            alerts.append(AlertEvent(
                                kind="rsi_oversold",
                                symbol=symbol, exchange=exchange, interval=interval,
                                message=f"RSI {rsi_val:.1f} — oversold (<{rsi_oversold})",
                                severity="warning", value=rsi_val, cache_key=ck,
                            ))
                            await _cache.set_flag(ck, alert_cooldown_secs)
                    elif rsi_val > rsi_overbought:
                        ck = f"alert:rsi_overbought:{symbol}:{interval}"
                        if not await _cache.exists(ck):
                            alerts.append(AlertEvent(
                                kind="rsi_overbought",
                                symbol=symbol, exchange=exchange, interval=interval,
                                message=f"RSI {rsi_val:.1f} — overbought (>{rsi_overbought})",
                                severity="warning", value=rsi_val, cache_key=ck,
                            ))
                            await _cache.set_flag(ck, alert_cooldown_secs)

            # ── Bollinger Squeeze ─────────────────────────────────────────────
            if "bb_squeeze" in indicators:
                sq = indicators["bb_squeeze"]["value"]
                if sq and sq >= 1.0:
                    ck = f"alert:bb_squeeze:{symbol}:{interval}"
                    if not await _cache.exists(ck):
                        alerts.append(AlertEvent(
                            kind="bb_squeeze",
                            symbol=symbol, exchange=exchange, interval=interval,
                            message="Bollinger Squeeze active — low-volatility coil, breakout expected",
                            severity="info", value=sq, cache_key=ck,
                        ))
                        await _cache.set_flag(ck, alert_cooldown_secs)

            # ── VIX regime change ─────────────────────────────────────────────
            if "vix_regime" in indicators and asset_type == "equity":
                regime = (indicators["vix_regime"]["payload"] or {}).get("regime")
                vix_val = indicators["vix_regime"]["value"]
                if vix_val and vix_val > vix_alert_threshold:
                    ck = f"alert:vix_elevated:{interval}"
                    if not await _cache.exists(ck):
                        alerts.append(AlertEvent(
                            kind="vix_elevated",
                            symbol=symbol, exchange=exchange, interval=interval,
                            message=f"VIX {vix_val:.1f} — regime: {regime}",
                            severity="warning", value=vix_val, cache_key=ck,
                        ))
                        await _cache.set_flag(ck, alert_cooldown_secs)

            # ── FA composite tier flip (equity only) ──────────────────────────
            if asset_type == "equity":
                derived = await fundamental.latest_derived(self._pool, symbol)
                if "composite_score" in derived:
                    tier = (derived["composite_score"]["payload"] or {}).get("tier")
                    prev_tier_key = f"fa_tier:{symbol}"
                    prev_tier = await _cache.get_json(prev_tier_key)
                    if prev_tier and prev_tier != tier and tier in ("strong", "weak"):
                        ck = f"alert:fa_tier_flip:{symbol}"
                        if not await _cache.exists(ck):
                            alerts.append(AlertEvent(
                                kind="fa_tier_flip",
                                symbol=symbol, exchange=exchange, interval=interval,
                                message=f"FA composite tier changed: {prev_tier} → {tier}",
                                severity="warning" if tier == "weak" else "info",
                                cache_key=ck,
                                payload={"prev_tier": prev_tier, "new_tier": tier},
                            ))
                            await _cache.set_flag(ck, alert_cooldown_secs)
                    if tier:
                        await _cache.set(prev_tier_key, tier, 86400)

            # ── Liquidity sweep ───────────────────────────────────────────────
            liq_key = next((k for k in indicators if k.startswith("liquidity_sweep")), None)
            if liq_key:
                count = indicators[liq_key]["value"]
                if count and count > 0:
                    ck = f"alert:liq_sweep:{symbol}:{interval}"
                    if not await _cache.exists(ck):
                        alerts.append(AlertEvent(
                            kind="liquidity_sweep",
                            symbol=symbol, exchange=exchange, interval=interval,
                            message=f"Liquidity sweep detected ({int(count)} sweeps)",
                            severity="info", value=count, cache_key=ck,
                        ))
                        await _cache.set_flag(ck, alert_cooldown_secs)

        for sym in equity_symbols:
            try:
                await _check(sym, "equity")
            except Exception as exc:
                log.error("alert scan error symbol=%s: %s", sym, exc)

        for sym in crypto_symbols:
            try:
                await _check(sym, "crypto")
            except Exception as exc:
                log.error("alert scan error symbol=%s: %s", sym, exc)

        return alerts

    # ── Private builders ──────────────────────────────────────────────────────

    async def _build_price(self, symbol: str, asset_type: str) -> Optional[PriceSnapshot]:
        try:
            if asset_type == "equity":
                row = await ohlcv.latest_bar(self._pool, symbol, "equity_ohlcv", self._equity_interval)
            else:
                row = await ohlcv.latest_crypto_bar(self._pool, symbol, self._crypto_interval)
            if not row:
                return None
            prev_row = None
            try:
                bars = await ohlcv.recent_bars(
                    self._pool, symbol,
                    "equity_ohlcv" if asset_type == "equity" else "crypto_ohlcv",
                    self._equity_interval if asset_type == "equity" else self._crypto_interval,
                    limit=2,
                )
                if len(bars) >= 2:
                    prev_row = bars[-2]
            except Exception:
                pass
            change_pct = None
            if prev_row and prev_row["close"]:
                change_pct = (row["close"] - prev_row["close"]) / prev_row["close"] * 100
            return PriceSnapshot(
                symbol=symbol,
                asset_type=asset_type,
                interval=row.get("interval", self._equity_interval if asset_type == "equity" else self._crypto_interval),
                ts=row["ts"],
                open=float(row["open"]),
                high=float(row["high"]),
                low=float(row["low"]),
                close=float(row["close"]),
                volume=float(row["volume"]),
                source=row.get("source", ""),
                change_pct=change_pct,
            )
        except Exception as exc:
            log.warning("price build failed symbol=%s: %s", symbol, exc)
            return None

    async def _build_technical(self, symbol: str, asset_type: str) -> Optional[TechnicalSnapshot]:
        try:
            exchange = "equity" if asset_type == "equity" else "binance"
            interval = self._equity_interval if asset_type == "equity" else self._crypto_interval
            ind = await technical.latest_indicators(self._pool, symbol, exchange, interval)
            if not ind:
                return None

            def _val(key: str) -> Optional[float]:
                return ind.get(key, {}).get("value")

            def _payload(key: str) -> dict:
                return ind.get(key, {}).get("payload") or {}

            snap = TechnicalSnapshot(symbol=symbol, exchange=exchange, interval=interval)
            snap.rsi = _val("rsi_14")
            macd_p = _payload("macd_12_26_9")
            snap.macd_hist = _val("macd_12_26_9")
            snap.macd_bullish_cross = macd_p.get("bullish_cross_line_signal")
            snap.macd_bearish_cross = macd_p.get("bearish_cross_line_signal")
            trend_p = _payload("trend")
            snap.trend_direction = trend_p.get("direction")
            snap.trend_slope_pct = trend_p.get("slope_pct")
            bb_sq_p = _payload("bb_squeeze")
            snap.bb_squeeze = bb_sq_p.get("squeeze")
            vix_p = _payload("vix_regime")
            snap.vix_regime = vix_p.get("regime")
            snap.vix_value = _val("vix_regime")
            snap.adx = _val("adx_14")
            snap.atr = _val("atr_14")
            piv_p = _payload("pivots_prior_bar")
            classic = piv_p.get("classic") or {}
            snap.pivot_pp = classic.get("PP")
            snap.pivot_r1 = classic.get("R1")
            snap.pivot_s1 = classic.get("S1")
            fvg_key = next((k for k in ind if k.startswith("fvg_")), None)
            if fvg_key:
                snap.fvg_active_count = int(_val(fvg_key) or 0)
            ob_key = next((k for k in ind if k.startswith("order_blocks_")), None)
            if ob_key:
                snap.ob_active_count = int(_val(ob_key) or 0)
            lq_key = next((k for k in ind if k.startswith("liquidity_sweep_")), None)
            if lq_key:
                snap.liq_sweep_count = int(_val(lq_key) or 0)
            hs_key = next((k for k in ind if k.startswith("hs_pattern_")), None)
            if hs_key:
                hs_p = _payload(hs_key)
                snap.hs_found = hs_p.get("hs_found")
                snap.hs_neckline_break = hs_p.get("hs_neckline_break")
                snap.inv_hs_found = hs_p.get("inv_hs_found")
                snap.inv_hs_neckline_break = hs_p.get("inv_hs_neckline_break")
            tri_key = next((k for k in ind if k.startswith("triangle_")), None)
            if tri_key:
                tri_p = _payload(tri_key)
                snap.triangle_kind = tri_p.get("kind") if tri_p.get("kind") != "none" else None
                snap.triangle_breakout = tri_p.get("breakout") if tri_p.get("breakout") != "none" else None
            flag_key = next((k for k in ind if k.startswith("flag_")), None)
            if flag_key:
                flag_p = _payload(flag_key)
                snap.bull_flag = flag_p.get("bull_flag")
                snap.bear_flag = flag_p.get("bear_flag")
            ribbon_p = _payload("ma_ribbon")
            snap.golden_cross = ribbon_p.get("golden_cross")
            snap.death_cross = ribbon_p.get("death_cross")
            ms_key = next((k for k in ind if k.startswith("market_structure_")), None)
            if ms_key:
                snap.market_structure_score = _val(ms_key)
            candle_p = _payload("candle_patterns")
            snap.candle_patterns = candle_p.get("patterns") or []
            return snap
        except Exception as exc:
            log.warning("technical build failed symbol=%s: %s", symbol, exc)
            return None

    async def _build_fundamental(self, symbol: str) -> Optional[FundamentalSnapshot]:
        try:
            derived = await fundamental.latest_derived(self._pool, symbol)
            ttm = await fundamental.latest_ttm(self._pool, symbol)
            if not derived and not ttm:
                return None

            def _d_tier(key: str) -> Optional[str]:
                return (derived.get(key, {}).get("payload") or {}).get("tier")

            def _d_val(key: str) -> Optional[float]:
                return derived.get(key, {}).get("value")

            snap = FundamentalSnapshot(symbol=symbol)
            snap.composite_score = _d_val("composite_score")
            snap.composite_tier = _d_tier("composite_score")
            snap.eps_strength = _d_tier("eps_strength")
            snap.revenue_strength = _d_tier("revenue_strength")
            snap.pe_tier = _d_tier("pe_vs_5y_mean")
            snap.pe_pct_vs_5y = _d_val("pe_vs_5y_mean")
            snap.fcf_yield_pct = (derived.get("fcf_yield", {}).get("payload") or {}).get("fcf_yield_pct")
            snap.fcf_yield_tier = _d_tier("fcf_yield_tier")
            snap.gross_margin_tier = _d_tier("gross_margin_tier")
            snap.gross_margin_pct = (derived.get("gross_margin_tier", {}).get("payload") or {}).get("gross_margin_pct")
            snap.net_margin_tier = _d_tier("net_margin_tier")
            snap.net_margin_pct = (derived.get("net_margin_tier", {}).get("payload") or {}).get("net_margin_pct")
            snap.peg_tier = _d_tier("peg_tier")
            snap.earnings_surprise_avg = _d_val("earnings_surprise_avg")
            snap.earnings_surprise_tier = _d_tier("earnings_surprise_avg")
            snap.gross_margin_trend = (derived.get("gross_margin_trend_8q", {}).get("payload") or {}).get("direction")
            snap.net_margin_trend = (derived.get("net_margin_trend_8q", {}).get("payload") or {}).get("direction")
            snap.eps_ttm = ttm.get("eps_ttm")
            snap.pe_ratio_ttm = ttm.get("pe_ratio_ttm")
            # Finnhub stores market_cap in millions USD; convert to raw dollars for the formatter.
            mc = ttm.get("market_cap")
            snap.market_cap = mc * 1_000_000 if mc is not None else None

            # ── Tier 3 context signals ────────────────────────────────────────
            def _t3_payload(key: str) -> dict:
                return derived.get(key, {}).get("payload") or {}

            # Share Count Trend
            t3_share_p = _t3_payload("t3_share_trend")
            snap.share_trend_pct = t3_share_p.get("annual_change_pct")
            snap.share_trend_tier = t3_share_p.get("tier")

            # DCF
            t3_dcf_p = _t3_payload("t3_dcf")
            snap.dcf_market_vs_intrinsic_pct = t3_dcf_p.get("market_cap_vs_dcf_pct")
            snap.dcf_tier = t3_dcf_p.get("tier")
            snap.dcf_growth_rate_pct = t3_dcf_p.get("growth_rate_pct")
            snap.dcf_value_millions = t3_dcf_p.get("dcf_value_millions")

            # Interest Coverage
            t3_ic_p = _t3_payload("t3_interest_coverage")
            snap.interest_coverage = t3_ic_p.get("coverage_ratio")
            snap.interest_coverage_tier = t3_ic_p.get("tier")

            # Asset & Inventory Turnover
            t3_at_p = _t3_payload("t3_asset_turnover")
            snap.asset_turnover = t3_at_p.get("asset_turnover")
            t3_inv_p = _t3_payload("t3_inventory_turnover")
            snap.inventory_turnover = t3_inv_p.get("inventory_turnover")

            # Analyst Target Price
            t3_analyst_p = _t3_payload("t3_analyst_target")
            snap.analyst_upside_pct = t3_analyst_p.get("upside_pct")
            snap.analyst_target_price = t3_analyst_p.get("target_price")
            snap.analyst_target_tier = t3_analyst_p.get("tier")

            # Goodwill & Intangibles
            t3_goodwill_p = _t3_payload("t3_goodwill_risk")
            snap.goodwill_pct = t3_goodwill_p.get("goodwill_intangibles_pct")
            snap.goodwill_tier = t3_goodwill_p.get("tier")

            # Price-to-Sales
            t3_ps_p = _t3_payload("t3_ps_ratio")
            snap.ps_ratio = t3_ps_p.get("ps_ratio")
            snap.ps_tier = t3_ps_p.get("tier")

            # FCF Conversion Rate (T3.9)
            t3_fcf_conv_p = _t3_payload("t3_fcf_conversion")
            snap.fcf_conversion_ratio = t3_fcf_conv_p.get("fcf_conversion_ratio")
            snap.fcf_conversion_tier = t3_fcf_conv_p.get("tier")

            # Analyst Recommendation Trend (T3.10)
            t3_rec_p = _t3_payload("t3_analyst_rec_trend")
            snap.analyst_rec_trend_delta = t3_rec_p.get("trend_delta")
            snap.analyst_rec_trend_tier = t3_rec_p.get("tier")
            snap.analyst_rec_net_score = t3_rec_p.get("net_score_current")

            # ── Tier 2 derived signals ────────────────────────────────────────
            def _t2_payload(key: str) -> dict:
                return derived.get(key, {}).get("payload") or {}

            # ROE / ROA
            t2_roe_p = _t2_payload("t2_roe")
            snap.roe_pct = t2_roe_p.get("roe_pct")
            snap.roe_tier = t2_roe_p.get("tier")
            t2_roa_p = _t2_payload("t2_roa")
            snap.roa_pct = t2_roa_p.get("roa_pct")

            # ROIC
            t2_roic_p = _t2_payload("t2_roic")
            snap.roic_pct = t2_roic_p.get("roic_pct")
            snap.roic_tier = t2_roic_p.get("tier")

            # Leverage — D/E ratio
            t2_lev_p = _t2_payload("t2_leverage")
            snap.leverage_de = t2_lev_p.get("debt_to_equity")
            snap.leverage_tier = t2_lev_p.get("tier")

            # Net Debt / EBITDA proxy
            t2_nd_p = _t2_payload("t2_net_debt_ebitda")
            snap.net_debt_ebitda = t2_nd_p.get("ratio")
            snap.net_debt_ebitda_tier = t2_nd_p.get("tier")

            # EV/EBITDA
            t2_ev_p = _t2_payload("t2_ev_ebitda")
            snap.ev_ebitda = t2_ev_p.get("ev_to_ebitda")
            snap.ev_ebitda_tier = t2_ev_p.get("tier")

            # Current Ratio / Quick Ratio
            t2_cr_p = _t2_payload("t2_current_ratio")
            snap.current_ratio = t2_cr_p.get("current_ratio")
            snap.current_ratio_tier = t2_cr_p.get("tier")
            t2_qr_p = _t2_payload("t2_quick_ratio")
            snap.quick_ratio = t2_qr_p.get("quick_ratio")

            # P/B ratio
            t2_pb_p = _t2_payload("t2_pb")
            snap.pb_ratio = t2_pb_p.get("price_to_book")
            snap.pb_tier = t2_pb_p.get("tier")

            # Dividend yield + payout sustainability
            t2_div_p = _t2_payload("t2_dividend")
            snap.dividend_yield_pct = t2_div_p.get("dividend_yield_pct")
            snap.dividend_sustainability = t2_div_p.get("sustainability")

            # CapEx intensity
            t2_capex_p = _t2_payload("t2_capex_intensity")
            snap.capex_intensity_pct = t2_capex_p.get("capex_intensity_pct")
            snap.capex_tier = t2_capex_p.get("tier")

            # Tier 2 composite health
            t2_health_p = _t2_payload("t2_health_score")
            snap.t2_health_score = _d_val("t2_health_score")
            snap.t2_health_tier = t2_health_p.get("tier")

            # ── Qualitative signals ───────────────────────────────────────────
            def _qual_payload(key: str) -> dict:
                return derived.get(key, {}).get("payload") or {}

            # Moat proxy
            qual_moat_p = _qual_payload("qual_moat_proxy")
            snap.qual_moat_proxy_tier = qual_moat_p.get("tier")
            snap.qual_moat_margin_mean = qual_moat_p.get("gross_margin_mean")
            snap.qual_moat_margin_std = qual_moat_p.get("gross_margin_std")

            # Insider signal
            qual_ins_p = _qual_payload("qual_insider_signal")
            snap.qual_insider_signal = qual_ins_p.get("tier")
            buyer_count = qual_ins_p.get("buyer_count")
            seller_count = qual_ins_p.get("seller_count")
            snap.qual_insider_buyer_count = int(buyer_count) if buyer_count is not None else None
            snap.qual_insider_seller_count = int(seller_count) if seller_count is not None else None

            # News sentiment
            qual_s7_p = _qual_payload("qual_news_sentiment_7d")
            snap.qual_news_sentiment_7d = qual_s7_p.get("avg_sentiment")
            snap.qual_news_sentiment_7d_tier = qual_s7_p.get("tier")

            qual_s30_p = _qual_payload("qual_news_sentiment_30d")
            snap.qual_news_sentiment_30d = qual_s30_p.get("avg_sentiment")
            snap.qual_news_sentiment_30d_tier = qual_s30_p.get("tier")

            # R&D intensity
            qual_rd_p = _qual_payload("qual_rd_intensity")
            snap.qual_rd_intensity_pct = qual_rd_p.get("rd_pct")
            snap.qual_rd_tier = qual_rd_p.get("tier")

            # ── Correlation signals ───────────────────────────────────────────
            def _corr_payload(key: str) -> dict:
                return derived.get(key, {}).get("payload") or {}

            # Cluster tiers
            snap.corr_earnings_quality_tier = (_corr_payload("corr_earnings_quality").get("tier"))
            snap.corr_valuation_quality_tier = (_corr_payload("corr_valuation_quality").get("tier"))
            snap.corr_leverage_liquidity_tier = (_corr_payload("corr_leverage_liquidity").get("tier"))
            snap.corr_operational_tier = (_corr_payload("corr_operational").get("tier"))

            # Summary
            corr_sum_p = _corr_payload("corr_summary")
            snap.corr_summary_score = _d_val("corr_summary")
            snap.corr_summary_tier = corr_sum_p.get("tier")

            # Master signals
            corr_master_p = _corr_payload("corr_master_signals")
            snap.corr_master_net_signal = corr_master_p.get("net_signal")

            bc = corr_master_p.get("bullish_convergence") or {}
            snap.corr_bullish_convergence_fired = bc.get("fired")
            snap.corr_bullish_convergence_score = bc.get("score")

            hv = corr_master_p.get("hidden_value") or {}
            snap.corr_hidden_value_fired = hv.get("fired")

            dw = corr_master_p.get("deterioration_warning") or {}
            snap.corr_deterioration_warning_fired = dw.get("fired")

            vt = corr_master_p.get("value_trap") or {}
            snap.corr_value_trap_fired = vt.get("fired")

            lc = corr_master_p.get("leverage_cycle_warning") or {}
            snap.corr_leverage_cycle_fired = lc.get("fired")

            # Accumulate all warnings and positives across clusters for display
            for ckey in ("corr_earnings_quality", "corr_valuation_quality",
                         "corr_leverage_liquidity", "corr_operational"):
                cp = _corr_payload(ckey)
                snap.corr_warnings.extend(cp.get("warnings") or [])
                snap.corr_positives.extend(cp.get("positives") or [])

            return snap
        except Exception as exc:
            log.warning("fundamental build failed symbol=%s: %s", symbol, exc)
            return None

    async def _build_sentiment(self, symbol: str) -> Optional[SentimentSnapshot]:
        try:
            row = await sentiment.latest_sentiment(self._pool, symbol)
            if not row:
                return None
            return SentimentSnapshot(
                symbol=symbol,
                source=row["source"],
                score=float(row["score"]) if row["score"] is not None else None,
                ts=row["ts"],
                raw_payload=row.get("payload") or {},
            )
        except Exception as exc:
            log.warning("sentiment build failed symbol=%s: %s", symbol, exc)
            return None

    async def _build_news(self, symbol: str, asset_type: str = "equity") -> list[NewsHeadline]:
        try:
            rows = await news.recent_headlines(
                self._pool, symbol, self._news_limit, asset_type=asset_type
            )
            return [
                NewsHeadline(
                    headline=r["headline"],
                    source=r["source"],
                    url=r.get("url"),
                    sentiment=float(r["sentiment"]) if r.get("sentiment") is not None else None,
                    ts=r["ts"],
                    symbol=r.get("symbol"),
                )
                for r in rows
            ]
        except Exception as exc:
            log.warning("news build failed symbol=%s: %s", symbol, exc)
            return []

    def _fill_market_ops_global(self, mo: MarketOpsSlice, raw: dict) -> None:
        if isinstance(raw.get("as_of"), str):
            mo.global_as_of = raw["as_of"]
        g = raw.get("global")
        if isinstance(g, dict):
            if g.get("vix") is not None:
                try:
                    mo.global_vix = float(g["vix"])
                    mo.vix_from_macro_fred = True
                except (TypeError, ValueError):
                    pass
            if isinstance(g.get("vix_regime"), str):
                mo.global_vix_regime = g["vix_regime"]
            if isinstance(g.get("vix_label"), str):
                mo.global_vix_label = g["vix_label"]
        mods = raw.get("reference_modules")
        if isinstance(mods, dict):
            for k in sorted(mods.keys()):
                ent = mods[k]
                if isinstance(ent, dict):
                    mo.reference_coverage_lines.append(
                        f"{k}: {ent.get('status', '—')} — {ent.get('hint', '')}"
                    )

    def _classify_vix_market_ops(self, vix: float) -> tuple[str, str]:
        """Match market-operations worker bands (MARKET_OPS_VIX_*)."""
        lo, nm, el = (
            self._market_ops_vix_low_max,
            self._market_ops_vix_normal_max,
            self._market_ops_vix_elevated_max,
        )
        if vix < lo:
            return (
                "low",
                f"VIX {vix:.1f} — complacency risk; vol spikes can surprise trend strategies.",
            )
        if vix < nm:
            return "normal", f"VIX {vix:.1f} — typical range; baseline sizing rules."
        if vix < el:
            return (
                "elevated",
                f"VIX {vix:.1f} — elevated fear; wider noise band, size with care.",
            )
        return (
            "stress",
            f"VIX {vix:.1f} — stress regime; prioritize liquidity and gap risk.",
        )

    async def _hydrate_market_ops_vix(self, mo: MarketOpsSlice) -> None:
        """Prefer live macro_fred VIXCLS; if missing, use benchmark TA vix_regime (same source as TA embed)."""
        fv: Optional[float] = None
        from_fred = False
        try:
            v = await ohlcv.latest_macro(self._pool, "VIXCLS")
            if v is not None:
                fv = float(v)
                from_fred = True
        except (TypeError, ValueError, Exception) as exc:
            log.debug("market ops VIX from macro_fred: %s", exc)

        if fv is None:
            try:
                ind = await technical.latest_indicators(
                    self._pool,
                    self._benchmark_symbol,
                    "equity",
                    self._equity_interval,
                )
                vr = ind.get("vix_regime") or {}
                if vr.get("value") is not None:
                    fv = float(vr["value"])
            except (TypeError, ValueError, Exception) as exc:
                log.debug("market ops VIX TA fallback: %s", exc)

        if fv is None:
            return
        mo.global_vix = fv
        mo.vix_from_macro_fred = from_fred
        reg, lbl = self._classify_vix_market_ops(fv)
        mo.global_vix_regime = reg
        if from_fred:
            mo.global_vix_label = lbl
        else:
            mo.global_vix_label = (
                lbl
                + f" — _Same source as TA **vix_regime** (**{self._benchmark_symbol}**). "
                "**macro_fred** has no **VIXCLS** — run **data-equity** FRED._"
            )

    async def _build_market_ops_slice(
        self,
        symbol: str,
        asset_type: str,
        price: Optional[PriceSnapshot],
        tech: Optional[TechnicalSnapshot],
        *,
        include_symbol: bool = True,
    ) -> Optional[MarketOpsSlice]:
        if not self._market_ops_enable:
            return None
        from statistics import median

        mo = MarketOpsSlice()
        try:
            raw = await ohlcv.latest_macro_derived(
                self._pool,
                "mo_reference_snapshot",
                source="market_operations",
            )
            if raw:
                self._fill_market_ops_global(mo, raw)
        except Exception as exc:
            log.warning("market ops global load failed: %s", exc)

        await self._hydrate_market_ops_vix(mo)

        if not include_symbol or not symbol:
            return mo

        table = "equity_ohlcv" if asset_type == "equity" else "crypto_ohlcv"
        interval = self._equity_interval if asset_type == "equity" else self._crypto_interval
        try:
            bars = await ohlcv.recent_bars(
                self._pool,
                symbol,
                table,
                interval,
                self._market_ops_volume_lookback,
            )
        except Exception as exc:
            log.warning("market ops bars failed symbol=%s: %s", symbol, exc)
            bars = []

        if len(bars) >= 5:
            vols = [float(b["volume"]) for b in bars if b.get("volume") is not None]
            if vols:
                med_v = float(median(vols))
                last_v = vols[-1]
                mo.volume_lookback_bars = len(vols)
                if med_v > 0:
                    mo.volume_vs_median_ratio = round(last_v / med_v, 2)

        close = float(price.close) if price and price.close is not None else None
        if close is None and bars:
            try:
                close = float(bars[-1]["close"])
            except (TypeError, ValueError, KeyError):
                close = None

        atr_val: Optional[float] = None
        if tech and tech.atr is not None:
            atr_val = float(tech.atr)
        else:
            try:
                ex = "equity" if asset_type == "equity" else "binance"
                ind = await technical.latest_indicators(
                    self._pool, symbol, ex, interval
                )
                ik = f"atr_{self._market_ops_atr_period}"
                row = ind.get(ik) or {}
                if row.get("value") is not None:
                    atr_val = float(row["value"])
            except Exception as exc:
                log.debug("market ops atr fetch symbol=%s: %s", symbol, exc)

        if atr_val is not None:
            mo.atr = atr_val
        if atr_val is not None and close and close > 0:
            mo.atr_pct = round(100.0 * atr_val / close, 3)

        if mo.atr_pct is not None and mo.atr_pct >= self._market_ops_atr_pct_elevated:
            mo.flags.append("atr_pct_elevated")
        if (
            mo.volume_vs_median_ratio is not None
            and mo.volume_vs_median_ratio >= self._market_ops_volume_ratio_elevated
        ):
            mo.flags.append("volume_vs_median_elevated")

        if asset_type == "equity":
            mo.asset_execution_note = (
                "US equities: RTH liquidity; gaps on news/earnings — see market_operations_reference.html."
            )
        else:
            mo.asset_execution_note = (
                "Crypto: 24/7; funding/OI not wired — microstructure differs from US equity sessions."
            )

        return mo

    async def _build_macro(self) -> MacroSnapshot:
        snap = MacroSnapshot()
        try:
            async def _md(metric: str) -> dict:
                return await ohlcv.latest_macro_derived(self._pool, metric) or {}

            # mc_* / aa_* before FRED — latest_macro used to do float(NULL) and abort
            # the whole try before we reached macro_derived (daily grey cards vs /analyze).
            mc_p = await _md("mc_market_cycle")
            if mc_p:
                try:
                    inp = mc_p.get("inputs") if isinstance(mc_p.get("inputs"), dict) else {}

                    def _mc_instr(k: str) -> Optional[str]:
                        v = inp.get(k)
                        return str(v) if v is not None and v != "" else None

                    def _mc_f(key: str) -> Optional[float]:
                        if mc_p.get(key) is None:
                            return None
                        try:
                            return float(mc_p[key])
                        except (TypeError, ValueError):
                            return None

                    def _mc_i(key: str) -> Optional[int]:
                        if mc_p.get(key) is None:
                            return None
                        try:
                            return int(mc_p[key])
                        except (TypeError, ValueError):
                            return None

                    snap.market_cycle = MarketCycleSnapshot(
                        symbol=str(mc_p.get("symbol") or "SPY"),
                        close=_mc_f("close"),
                        drawdown_pct=_mc_f("drawdown_pct"),
                        pct_vs_sma200=_mc_f("pct_vs_sma200"),
                        sma200=_mc_f("sma200"),
                        price_phase=mc_p.get("price_phase")
                        if isinstance(mc_p.get("price_phase"), str)
                        else None,
                        crash_warning=bool(mc_p.get("crash_warning")),
                        days_off_peak=_mc_i("days_off_peak"),
                        composite_phase=mc_p.get("composite_phase")
                        if isinstance(mc_p.get("composite_phase"), str)
                        else None,
                        composite_label=mc_p.get("composite_label")
                        if isinstance(mc_p.get("composite_label"), str)
                        else None,
                        composite_score=_mc_f("value"),
                        gc_stance=_mc_instr("gc_stance"),
                        mp_stance=_mc_instr("mp_stance"),
                        inf_stance=_mc_instr("inf_stance"),
                        gg_stance=_mc_instr("gg_stance"),
                        bars_used=_mc_i("bars_used"),
                    )
                except Exception as mc_exc:
                    log.warning("macro market_cycle parse failed (daily embed fallback): %s", mc_exc)

            corr_p = await _md("mc_macro_correlation")
            if corr_p:
                try:
                    r = corr_p.get("regime")
                    if isinstance(r, str):
                        snap.macro_corr_regime = r
                    if corr_p.get("score") is not None:
                        snap.macro_corr_score = float(corr_p["score"])
                    lbl = corr_p.get("label")
                    if isinstance(lbl, str):
                        snap.macro_corr_label = lbl
                    fl = corr_p.get("flags")
                    if isinstance(fl, list):
                        snap.macro_corr_flags = [str(x) for x in fl if x is not None][:12]
                except (TypeError, ValueError) as c_exc:
                    log.warning("macro mc_macro_correlation parse failed: %s", c_exc)

            aa_p = await _md("aa_reference_snapshot")
            if aa_p:
                add = _parse_additional_snapshot(aa_p)
                if add:
                    snap.additional = add

            # ── Raw FRED values ────────────────────────────────────────────────
            snap.vix = await ohlcv.latest_macro(self._pool, "VIXCLS")
            snap.dgs10 = await ohlcv.latest_macro(self._pool, "DGS10")
            snap.dexuseu = await ohlcv.latest_macro(self._pool, "DEXUSEU")
            snap.fedfunds = await ohlcv.latest_macro(self._pool, "FEDFUNDS")
            snap.dgs2 = await ohlcv.latest_macro(self._pool, "DGS2")
            snap.dgs30 = await ohlcv.latest_macro(self._pool, "DGS30")
            snap.real_rate_10y = await ohlcv.latest_macro(self._pool, "DFII10")
            snap.breakeven_10y = await ohlcv.latest_macro(self._pool, "T10YIE")
            snap.breakeven_5y = await ohlcv.latest_macro(self._pool, "T5YIE")

            hy_raw = await ohlcv.latest_macro(self._pool, "BAMLH0A0HYM2")
            ig_raw = await ohlcv.latest_macro(self._pool, "BAMLC0A0CM")
            snap.hy_spread = hy_raw * 100 if hy_raw is not None else None
            snap.ig_spread = ig_raw * 100 if ig_raw is not None else None

            m2 = await ohlcv.latest_macro(self._pool, "M2SL")
            snap.m2_billions = m2

            # Daily header "Macro" strip: same VIX fallback as market ops when FRED has no VIXCLS.
            if snap.vix is None:
                try:
                    ind = await technical.latest_indicators(
                        self._pool,
                        self._benchmark_symbol,
                        "equity",
                        self._equity_interval,
                    )
                    vr = ind.get("vix_regime") or {}
                    if vr.get("value") is not None:
                        snap.vix = float(vr["value"])
                except (TypeError, ValueError, Exception) as exc:
                    log.debug("macro snapshot VIX TA fallback: %s", exc)

            rate_p = await _md("mp_rate")
            snap.mp_rate_regime = rate_p.get("regime")
            snap.mp_rate_change_yoy_bps = rate_p.get("change_yoy_bps")

            yc_p = await _md("mp_yield_curve")
            snap.yield_curve_2s10s = yc_p.get("spread_2s10s_pct")
            snap.yield_curve_3m10y = yc_p.get("spread_3m10y_pct")
            snap.yield_curve_regime = yc_p.get("regime")

            rr_p = await _md("mp_real_rate")
            snap.real_rate_regime = rr_p.get("regime")

            bs_p = await _md("mp_balance_sheet")
            snap.fed_balance_sheet_bn = bs_p.get("total_assets_billions")
            snap.fed_bs_4w_change_bn = bs_p.get("4w_change_billions")
            snap.fed_bs_regime = bs_p.get("regime")

            cs_p = await _md("mp_credit_spread")
            snap.credit_hy_bps = cs_p.get("hy_spread_bps")
            snap.credit_ig_bps = cs_p.get("ig_spread_bps")
            snap.credit_regime = cs_p.get("regime")

            be_p = await _md("mp_breakeven_inflation")
            snap.inflation_expectations_regime = be_p.get("regime")

            m2_p = await _md("mp_m2_supply")
            if m2_p.get("yoy_pct") is not None:
                try:
                    snap.m2_yoy_pct = float(m2_p["yoy_pct"])
                except (TypeError, ValueError):
                    snap.m2_yoy_pct = None
            snap.m2_regime = m2_p.get("regime")

            stance_p = await _md("mp_stance")
            snap.mp_stance = stance_p.get("stance")
            snap.mp_score = stance_p.get("value")  # stored as the scalar value field

            # ── Growth Cycle — Tier 1 (Leading) ──────────────────────────────
            pmi_p = await _md("gc_pmi")
            snap.gc_pmi = pmi_p.get("value")
            snap.gc_pmi_regime = pmi_p.get("regime")
            snap.gc_pmi_trend3m = pmi_p.get("trend3m")

            lei_p = await _md("gc_lei")
            snap.gc_lei = lei_p.get("value")
            snap.gc_lei_six_month_rate = lei_p.get("six_month_rate_pct")
            snap.gc_lei_regime = lei_p.get("regime")

            claims_p = await _md("gc_claims")
            snap.gc_claims_4w_ma = claims_p.get("icsa_4w_ma")
            snap.gc_claims_latest = claims_p.get("icsa_latest")
            snap.gc_claims_ccsa = claims_p.get("ccsa_latest")
            snap.gc_claims_regime = claims_p.get("regime")

            housing_p = await _md("gc_housing")
            snap.gc_housing_starts = housing_p.get("houst_k_ann")
            snap.gc_housing_permits = housing_p.get("permit_k_ann")
            snap.gc_housing_regime = housing_p.get("regime")

            # ── Growth Cycle — Tier 2 (Coincident) ───────────────────────────
            gdp_p = await _md("gc_gdp")
            snap.gc_gdp_ann_pct = gdp_p.get("ann_pct")
            snap.gc_gdp_regime = gdp_p.get("regime")

            empl_p = await _md("gc_employment")
            snap.gc_payrolls_k = empl_p.get("payems_k")
            snap.gc_unemployment = empl_p.get("unrate_pct")
            snap.gc_ahe_pct = empl_p.get("ahe_yoy_pct")  # YoY % change computed from level
            snap.gc_sahm_pp = empl_p.get("sahm_pp")
            snap.gc_empl_regime = empl_p.get("regime")

            consumer_p = await _md("gc_consumer")
            snap.gc_retail_yoy_pct = consumer_p.get("rrsfs_yoy_pct")
            snap.gc_retail_nominal_mn = consumer_p.get("rsafs_nominal_mn")
            snap.gc_consumer_regime = consumer_p.get("regime")

            # ── Growth Cycle — Tier 3 (Lagging / Sentiment) ──────────────────
            umich_p = await _md("gc_consumer_sentiment")
            snap.gc_umich = umich_p.get("value")
            snap.gc_umich_regime = umich_p.get("regime")

            capex_p = await _md("gc_capex")
            snap.gc_capex_3m_pct = capex_p.get("neworder_3m_pct")
            snap.gc_capex_latest = capex_p.get("neworder_latest")
            snap.gc_durable_goods = capex_p.get("dgorder_latest")
            snap.gc_capex_regime = capex_p.get("regime")

            # ── Growth Cycle Composite ─────────────────────────────────────────
            gc_stance_p = await _md("gc_stance")
            snap.gc_stance = gc_stance_p.get("stance")
            snap.gc_score = gc_stance_p.get("value")
            snap.gc_signals_used = gc_stance_p.get("signals_used")

            # ── Inflation & Prices ────────────────────────────────────────────
            cpi_p = await _md("inf_cpi")
            snap.inf_cpi_yoy = cpi_p.get("yoy_pct")
            snap.inf_cpi_regime = cpi_p.get("regime")

            core_cpi_p = await _md("inf_core_cpi")
            snap.inf_core_cpi_yoy = core_cpi_p.get("yoy_pct")
            snap.inf_core_cpi_regime = core_cpi_p.get("regime")

            shelter_p = await _md("inf_shelter")
            snap.inf_shelter_yoy = shelter_p.get("yoy_pct")
            snap.inf_shelter_regime = shelter_p.get("regime")

            core_pce_p = await _md("inf_core_pce")
            snap.inf_core_pce_yoy = core_pce_p.get("core_pce_yoy")
            snap.inf_core_pce_regime = core_pce_p.get("regime")
            snap.inf_headline_pce_yoy = core_pce_p.get("headline_pce_yoy")

            ppi_p = await _md("inf_ppi")
            snap.inf_ppi_yoy = ppi_p.get("ppifid_yoy")
            snap.inf_ppi_regime = ppi_p.get("regime")
            snap.inf_ppi_cpi_spread = ppi_p.get("ppi_cpi_spread")
            snap.inf_ppi_margin_signal = ppi_p.get("margin_signal")
            snap.inf_ppiaco_yoy = ppi_p.get("ppiaco_yoy")

            oil_p = await _md("inf_oil")
            snap.inf_wti = oil_p.get("wti_usd")
            snap.inf_brent = oil_p.get("brent_usd")
            snap.inf_brent_wti_spread = oil_p.get("brent_wti_spread")
            snap.inf_oil_regime = oil_p.get("regime")

            wage_p = await _md("inf_wages")
            snap.inf_ahe_yoy = wage_p.get("ahe_yoy_pct")
            snap.inf_eci_yoy = wage_p.get("eci_yoy_pct")
            snap.inf_wage_regime = wage_p.get("regime")

            copper_p = await _md("inf_copper")
            snap.inf_copper_yoy = copper_p.get("copper_yoy_pct")
            snap.inf_copper_usd = copper_p.get("copper_usd_per_ton")
            snap.inf_copper_regime = copper_p.get("regime")

            inf_stance_p = await _md("inf_stance")
            snap.inf_stance = inf_stance_p.get("stance")
            snap.inf_score = inf_stance_p.get("value")
            snap.inf_signals_used = inf_stance_p.get("signals_used")

            # ── Global & Geopolitical ───────────────────────────────────────
            gbd_p = await _md("gg_broad_dollar")
            snap.gg_broad_dollar_index = gbd_p.get("value")
            snap.gg_broad_dollar_regime = gbd_p.get("regime")
            if snap.gg_broad_dollar_index is None and gbd_p.get("index") is not None:
                snap.gg_broad_dollar_index = float(gbd_p["index"])

            uj_p = await _md("gg_usdjpy")
            snap.gg_usdjpy_spot = uj_p.get("value")
            if snap.gg_usdjpy_spot is None and uj_p.get("latest_spot") is not None:
                snap.gg_usdjpy_spot = float(uj_p["latest_spot"])
            snap.gg_usdjpy_chg_20d_pct = uj_p.get("pct_chg_20d")
            snap.gg_usdjpy_regime = uj_p.get("regime")

            chn_p = await _md("gg_china_gdp")
            snap.gg_china_gdp_yoy = chn_p.get("yoy_pct")
            if snap.gg_china_gdp_yoy is None and chn_p.get("value") is not None:
                snap.gg_china_gdp_yoy = float(chn_p["value"])
            snap.gg_china_gdp_regime = chn_p.get("regime")

            fis_p = await _md("gg_fiscal")
            snap.gg_fiscal_deficit_pct_gdp = fis_p.get("deficit_pct_gdp")
            snap.gg_fiscal_fyfsd_millions = fis_p.get("fyfsd_millions")
            if snap.gg_fiscal_fyfsd_millions is None and fis_p.get("value") is not None:
                # When GDP missing, value may hold raw FYFSD millions
                v = fis_p.get("value")
                if v is not None and fis_p.get("deficit_pct_gdp") is None:
                    snap.gg_fiscal_fyfsd_millions = float(v)
            snap.gg_fiscal_regime = fis_p.get("regime")

            gg_stance_p = await _md("gg_stance")
            snap.gg_stance = gg_stance_p.get("stance")
            snap.gg_score = gg_stance_p.get("value")
            snap.gg_signals_used = gg_stance_p.get("signals_used")

            # Heal stale mc_market_cycle inputs: the Go worker embeds gc/mp/inf/gg stance
            # strings at write time; if those stances were insufficient_data (worker ran
            # before FRED data arrived) but are now available as standalone metrics,
            # backfill so the market-cycle embed agrees with the live macro embeds above it.
            if snap.market_cycle is not None:
                def _stale(v: Optional[str]) -> bool:
                    return v is None or v == "insufficient_data"
                if _stale(snap.market_cycle.gc_stance) and not _stale(snap.gc_stance):
                    snap.market_cycle.gc_stance = snap.gc_stance
                if _stale(snap.market_cycle.mp_stance) and not _stale(snap.mp_stance):
                    snap.market_cycle.mp_stance = snap.mp_stance
                if _stale(snap.market_cycle.inf_stance) and not _stale(snap.inf_stance):
                    snap.market_cycle.inf_stance = snap.inf_stance
                if _stale(snap.market_cycle.gg_stance) and not _stale(snap.gg_stance):
                    snap.market_cycle.gg_stance = snap.gg_stance

        except Exception as exc:
            log.warning("macro build failed: %s", exc)
        return snap

    async def _build_analyze_context(
        self, symbol: str, asset_type: str
    ) -> Optional[AnalyzeContextSnapshot]:
        ctx = AnalyzeContextSnapshot(benchmark_symbol=self._benchmark_symbol)
        has_any = False
        try:
            mc = await ohlcv.latest_macro_derived(self._pool, "mc_market_cycle")
            if mc:
                pp = mc.get("price_phase")
                if isinstance(pp, str):
                    ctx.benchmark_price_phase = pp
                    has_any = True
                cp = mc.get("composite_phase")
                if isinstance(cp, str):
                    ctx.benchmark_composite_phase = cp
                    has_any = True
                if mc.get("drawdown_pct") is not None:
                    ctx.benchmark_drawdown_pct = float(mc["drawdown_pct"])
                    has_any = True

            corr = await ohlcv.latest_macro_derived(self._pool, "mc_macro_correlation")
            if corr:
                r = corr.get("regime")
                if isinstance(r, str):
                    ctx.macro_corr_regime = r
                    has_any = True
                if corr.get("score") is not None:
                    ctx.macro_corr_score = float(corr["score"])
                    has_any = True
                lbl = corr.get("label")
                if isinstance(lbl, str):
                    ctx.macro_corr_label = lbl
                    has_any = True
                fl = corr.get("flags")
                if isinstance(fl, list):
                    ctx.macro_corr_flags = [str(x) for x in fl if x is not None][:10]
                    if ctx.macro_corr_flags:
                        has_any = True

            aa = await ohlcv.latest_macro_derived(self._pool, "aa_reference_snapshot")
            if aa:
                summary = _format_additional_summary_line(aa)
                if summary:
                    ctx.additional_summary_line = summary
                    has_any = True

            if asset_type == "equity":
                rs = await ohlcv.rel_return_vs_benchmark_excess_pct(
                    self._pool,
                    symbol,
                    self._benchmark_symbol,
                    self._equity_interval,
                    bars=20,
                )
                if rs is not None:
                    ctx.rs_20d_vs_benchmark_pct = rs
                    has_any = True
        except Exception as exc:
            log.warning("analyze context build failed: %s", exc)
            return None
        return ctx if has_any else None

    async def _build_macro_intel(self, equity_symbols: list[str]) -> MacroIntelSnapshot:
        snap = MacroIntelSnapshot()
        try:
            for r in await macro_intel.upcoming_economic_events(
                self._pool, hours=72, limit=10
            ):
                snap.economic_events.append(
                    EconomicCalendarBrief(
                        event_ts=r["event_ts"],
                        country=r.get("country") or "",
                        event_name=r.get("event_name") or "",
                        impact=r.get("impact"),
                    )
                )
            for r in await macro_intel.upcoming_earnings(
                self._pool, equity_symbols, days=14, limit=16
            ):
                snap.earnings_events.append(
                    EarningsCalendarBrief(
                        earnings_date=r["earnings_date"],
                        symbol=r.get("symbol") or "",
                        quarter=r.get("quarter"),
                        hour=r.get("hour"),
                    )
                )
            g = await macro_intel.latest_gpr(self._pool)
            if g:
                snap.gpr_month = g["month_ts"]
                if g.get("gpr_total") is not None:
                    snap.gpr_total = float(g["gpr_total"])
            gd = await macro_intel.latest_gdelt(self._pool, None)
            if gd:
                snap.gdelt_day = gd["day_ts"]
                snap.gdelt_query_label = gd.get("query_label")
                if gd.get("article_count") is not None:
                    snap.gdelt_article_count = int(gd["article_count"])
                if gd.get("avg_tone") is not None:
                    snap.gdelt_avg_tone = float(gd["avg_tone"])
            nar = await macro_intel.latest_narrative(self._pool, "fomc_statement")
            if nar:
                snap.narrative_kind = nar.get("doc_kind")
                snap.narrative_at = nar.get("created_at")
                if nar.get("llm_score") is not None:
                    snap.narrative_score = float(nar["llm_score"])
                snap.narrative_summary = nar.get("llm_summary")
            for r in await macro_intel.macro_tagged_headlines(self._pool, limit=8):
                snap.macro_headlines.append(
                    NewsHeadline(
                        headline=r["headline"],
                        source=r["source"],
                        url=r.get("url"),
                        ts=r.get("ts"),
                    )
                )
        except Exception as exc:
            log.warning("macro intel build failed: %s", exc)
        return snap

    # ── Cache serialisation helpers ───────────────────────────────────────────

    @staticmethod
    def _serialise_symbol_report(report: SymbolReport) -> dict:
        """Best-effort JSON serialisation for Redis cache."""
        import dataclasses
        def _convert(obj):
            if dataclasses.is_dataclass(obj) and not isinstance(obj, type):
                return {k: _convert(v) for k, v in dataclasses.asdict(obj).items()}
            if isinstance(obj, datetime):
                return obj.isoformat()
            if isinstance(obj, list):
                return [_convert(i) for i in obj]
            return obj
        return _convert(report)

    @staticmethod
    def _deserialise_symbol_report(data: dict) -> SymbolReport:
        """Reconstruct a SymbolReport from cached JSON dict (best-effort)."""
        from datetime import datetime
        def _ts(v):
            if isinstance(v, str):
                try:
                    return datetime.fromisoformat(v)
                except Exception:
                    return None
            return v

        p = data.get("price")
        price = PriceSnapshot(**{**p, "ts": _ts(p["ts"])}) if p else None
        t = data.get("technical")
        tech = TechnicalSnapshot(**t) if t else None
        f = data.get("fundamental")
        fund = FundamentalSnapshot(**f) if f else None
        s = data.get("sentiment")
        sent = SentimentSnapshot(**{**s, "ts": _ts(s.get("ts"))}) if s else None
        news_list = [NewsHeadline(**{**n, "ts": _ts(n.get("ts"))}) for n in data.get("news", [])]
        ac = data.get("analyze_context")
        analyze_ctx: Optional[AnalyzeContextSnapshot] = None
        if isinstance(ac, dict):
            mflags = ac.get("macro_corr_flags") or []
            if not isinstance(mflags, list):
                mflags = []
            analyze_ctx = AnalyzeContextSnapshot(
                benchmark_symbol=str(ac.get("benchmark_symbol") or "SPY"),
                benchmark_price_phase=ac.get("benchmark_price_phase")
                if isinstance(ac.get("benchmark_price_phase"), str)
                else None,
                benchmark_composite_phase=ac.get("benchmark_composite_phase")
                if isinstance(ac.get("benchmark_composite_phase"), str)
                else None,
                benchmark_drawdown_pct=float(ac["benchmark_drawdown_pct"])
                if ac.get("benchmark_drawdown_pct") is not None
                else None,
                rs_20d_vs_benchmark_pct=float(ac["rs_20d_vs_benchmark_pct"])
                if ac.get("rs_20d_vs_benchmark_pct") is not None
                else None,
                macro_corr_regime=ac.get("macro_corr_regime")
                if isinstance(ac.get("macro_corr_regime"), str)
                else None,
                macro_corr_score=float(ac["macro_corr_score"])
                if ac.get("macro_corr_score") is not None
                else None,
                macro_corr_label=ac.get("macro_corr_label")
                if isinstance(ac.get("macro_corr_label"), str)
                else None,
                macro_corr_flags=[str(x) for x in mflags],
                additional_summary_line=ac.get("additional_summary_line")
                if isinstance(ac.get("additional_summary_line"), str)
                else None,
            )
        mo_data = data.get("market_ops")
        market_ops: Optional[MarketOpsSlice] = None
        if isinstance(mo_data, dict):
            mflags = mo_data.get("flags") or []
            if not isinstance(mflags, list):
                mflags = []
            mlines = mo_data.get("reference_coverage_lines") or []
            if not isinstance(mlines, list):
                mlines = []
            market_ops = MarketOpsSlice(
                global_as_of=mo_data.get("global_as_of")
                if isinstance(mo_data.get("global_as_of"), str)
                else None,
                global_vix=float(mo_data["global_vix"])
                if mo_data.get("global_vix") is not None
                else None,
                global_vix_regime=mo_data.get("global_vix_regime")
                if isinstance(mo_data.get("global_vix_regime"), str)
                else None,
                global_vix_label=mo_data.get("global_vix_label")
                if isinstance(mo_data.get("global_vix_label"), str)
                else None,
                reference_coverage_lines=[str(x) for x in mlines],
                atr=float(mo_data["atr"]) if mo_data.get("atr") is not None else None,
                atr_pct=float(mo_data["atr_pct"])
                if mo_data.get("atr_pct") is not None
                else None,
                volume_vs_median_ratio=float(mo_data["volume_vs_median_ratio"])
                if mo_data.get("volume_vs_median_ratio") is not None
                else None,
                volume_lookback_bars=int(mo_data["volume_lookback_bars"])
                if mo_data.get("volume_lookback_bars") is not None
                else None,
                flags=[str(x) for x in mflags],
                asset_execution_note=mo_data.get("asset_execution_note")
                if isinstance(mo_data.get("asset_execution_note"), str)
                else None,
                vix_from_macro_fred=bool(mo_data.get("vix_from_macro_fred")),
            )
        return SymbolReport(
            symbol=data["symbol"],
            asset_type=data["asset_type"],
            price=price,
            technical=tech,
            fundamental=fund,
            sentiment=sent,
            news=news_list,
            analyze_context=analyze_ctx,
            market_ops=market_ops,
        )

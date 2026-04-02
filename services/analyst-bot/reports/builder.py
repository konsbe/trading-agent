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
from db.queries import fundamental, news, ohlcv, sentiment, technical
from reports.models import (
    AlertEvent,
    DailyReport,
    FundamentalSnapshot,
    MacroSnapshot,
    NewsHeadline,
    PriceSnapshot,
    SentimentSnapshot,
    SymbolReport,
    TechnicalSnapshot,
)

log = logging.getLogger(__name__)


class ReportBuilder:
    def __init__(
        self,
        pool: asyncpg.Pool,
        equity_interval: str = "1Day",
        crypto_interval: str = "1d",
        news_limit: int = 5,
        price_cache_ttl: int = 300,
        analyze_cache_ttl: int = 600,
    ) -> None:
        self._pool = pool
        self._equity_interval = equity_interval
        self._crypto_interval = crypto_interval
        self._news_limit = news_limit
        self._price_cache_ttl = price_cache_ttl
        self._analyze_cache_ttl = analyze_cache_ttl

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
        report.news = await self._build_news(symbol)

        if use_cache:
            await _cache.set(cache_key, self._serialise_symbol_report(report), self._analyze_cache_ttl)

        return report

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

    async def _build_news(self, symbol: str) -> list[NewsHeadline]:
        try:
            rows = await news.recent_headlines(self._pool, symbol, self._news_limit)
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

    async def _build_macro(self) -> MacroSnapshot:
        snap = MacroSnapshot()
        try:
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

            # HY/IG spreads: FRED series is in % — convert to bps for display
            hy_raw = await ohlcv.latest_macro(self._pool, "BAMLH0A0HYM2")
            ig_raw = await ohlcv.latest_macro(self._pool, "BAMLC0A0CM")
            snap.hy_spread = hy_raw * 100 if hy_raw is not None else None
            snap.ig_spread = ig_raw * 100 if ig_raw is not None else None

            m2 = await ohlcv.latest_macro(self._pool, "M2SL")
            snap.m2_billions = m2

            # ── Computed signals from macro_derived ───────────────────────────
            async def _md(metric: str) -> dict:
                return await ohlcv.latest_macro_derived(self._pool, metric) or {}

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
            snap.m2_yoy_pct = float(m2_p["yoy_pct"]) if m2_p.get("yoy_pct") is not None else None
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
            snap.gc_ahe_pct = empl_p.get("ahe_pct")
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

        except Exception as exc:
            log.warning("macro build failed: %s", exc)
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
        return SymbolReport(
            symbol=data["symbol"],
            asset_type=data["asset_type"],
            price=price,
            technical=tech,
            fundamental=fund,
            sentiment=sent,
            news=news_list,
        )

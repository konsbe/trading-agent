"""§8.3's momentum slash commands.

/scanner [bucket] [min_score]  — top candidates from today's scan
/score TICKER                  — full breakdown for any universe symbol
/tracked                       — active momentum_tracked rows with unrealised move

These are "the 'fetch dynamically' requirement" §8.3 names: the pre-existing
commands only operate on the statically configured BOT_EQUITY_SYMBOLS /
BOT_CRYPTO_SYMBOLS lists, whereas the whole point of the scanner is to surface
symbols nobody configured. Every query here reads the scanner's result set.

All three carry the same evidence caveat as the scheduled alerts. Of §4 v2's
seven components only `rvol` is statistically validated, and a score out of a
two-digit ceiling reads as authoritative whether or not it has earned it — so the
confidence level travels with the number on every surface, not just the automated
ones. Someone typing /score on a single ticker is if anything more likely to act
on the answer than someone skimming a feed.
"""

from __future__ import annotations

import logging

import discord
from discord.ext import commands

from db.queries import momentum as mq
from notifier.discord import momentum as fmt

log = logging.getLogger(__name__)

_BUCKETS = ["market", "penny"]


class MomentumCog(commands.Cog):
    def __init__(self, bot) -> None:
        self.bot = bot

    # ── /scanner ─────────────────────────────────────────────────────────────

    @discord.slash_command(
        name="scanner",
        description="Top momentum candidates from today's scan (not your watchlist)",
    )
    async def scanner_cmd(
        self,
        ctx: discord.ApplicationContext,
        bucket: discord.Option(str, "Price bucket", choices=_BUCKETS, required=False) = None,  # type: ignore[valid-type]
        min_score: discord.Option(int, "Minimum score", required=False) = None,  # type: ignore[valid-type]
    ) -> None:
        await ctx.defer()
        try:
            rows = await mq.top_candidates(
                self.bot.pool, bucket=bucket, min_score=min_score, limit=10
            )
            fresh = await mq.scan_freshness(self.bot.pool)
        except Exception:
            log.exception("/scanner query failed")
            await ctx.respond("Could not read the scanner's results — see logs.")
            return

        last = (fresh or {}).get("last_scan_date")
        scored = (fresh or {}).get("scored_today")

        if not rows:
            # An empty result is ambiguous without freshness: "nothing qualified"
            # and "the scanner has not run" look identical, and only one is a
            # market condition.
            await ctx.respond(
                f"No candidates match.\n"
                f"Last scan: **{last or 'never'}** ({scored or 0} symbols scored).\n"
                f"{fmt.EVIDENCE_CAVEAT}"
            )
            return

        embed = discord.Embed(
            title=f"🔎 Momentum candidates · {last or 'unknown date'}",
            description=(
                f"{len(rows)} shown"
                + (f" · bucket={bucket}" if bucket else "")
                + (f" · score ≥ {min_score}" if min_score is not None else "")
            ),
            color=0x3498DB,
        )
        for r in rows:
            denom = fmt.score_denominator(r.get("catalyst_tier"))
            embed.add_field(
                name=f"{r['symbol']} · {r['bucket']} · {r['momentum_score_100']}/{denom}",
                value=(
                    f"chg {_d(r.get('change_pct'), '%')} · "
                    f"RVOL {_d(r.get('rvol_20'), 'x')} · "
                    f"accel {_d(r.get('vol_accel'), 'x')}"
                ),
                inline=False,
            )
        embed.set_footer(text=f"{fmt.SESSION_CAVEAT}\n{fmt.EVIDENCE_CAVEAT}")
        await ctx.respond(embed=embed)

    # ── /score ───────────────────────────────────────────────────────────────

    @discord.slash_command(
        name="score",
        description="Full momentum score breakdown for any symbol in the universe",
    )
    async def score_cmd(
        self,
        ctx: discord.ApplicationContext,
        ticker: discord.Option(str, "Ticker symbol"),  # type: ignore[valid-type]
    ) -> None:
        await ctx.defer()
        sym = ticker.strip().upper()
        try:
            row = await mq.score_for_symbol(self.bot.pool, sym)
        except Exception:
            log.exception("/score query failed for %s", sym)
            await ctx.respond(f"Could not read a score for {sym} — see logs.")
            return

        if row is not None:
            await ctx.respond(fmt.build_score_breakdown(row, sym))
            return

        # No score. §3.2's gate failures are retained precisely so this is
        # answerable — "no score" alone leaves a user unable to tell a
        # thin-liquidity rejection from a missing-data one.
        try:
            gate = await mq.gate_failure_for_symbol(self.bot.pool, sym)
        except Exception:
            gate = None

        if gate is None:
            await ctx.respond(fmt.build_score_breakdown(None, sym))
            return

        reasons = gate.get("gate_failures") or "(none recorded)"
        await ctx.respond(
            f"**{sym}** — no score: it did not pass §3.2's hard gates.\n"
            f"Bucket: `{gate.get('bucket') or '—'}` · as of `{gate.get('ts')}`\n"
            f"Gate failures: `{reasons}`\n"
            f"close {_d(gate.get('close'))} · chg {_d(gate.get('change_pct'), '%')} · "
            f"RVOL {_d(gate.get('rvol_20'), 'x')} · $vol {_d(gate.get('dollar_volume'), '', 0)}\n\n"
            f"A gate failure means **excluded, not low-scored** (§3.2) — scoring only "
            f"ranks within the candidate set.\n"
            f"{fmt.EVIDENCE_CAVEAT}"
        )

    # ── /tracked ─────────────────────────────────────────────────────────────

    @discord.slash_command(
        name="tracked",
        description="Active momentum positions with unrealised move since the alert",
    )
    async def tracked_cmd(self, ctx: discord.ApplicationContext) -> None:
        await ctx.defer()
        try:
            rows = await mq.tracked_positions(self.bot.pool, status="active")
        except Exception:
            log.exception("/tracked query failed")
            await ctx.respond("Could not read tracked positions — see logs.")
            return

        if not rows:
            await ctx.respond("No active tracked positions.")
            return

        embed = discord.Embed(
            title="📍 Tracked momentum positions",
            description=f"{len(rows)} active",
            color=0x9B59B6,
        )
        for r in rows:
            move = r.get("unrealized_pct")
            arrow = "▲" if (move or 0) >= 0 else "▼"
            embed.add_field(
                name=f"{r['symbol']} · {r.get('bucket') or '—'}",
                value=(
                    f"{arrow} {_d(move, '%')} since alert\n"
                    f"ref {_d(r.get('reference_price'))} → {_d(r.get('current_close'))}\n"
                    f"alerted {r.get('alert_ts')}"
                ),
                inline=True,
            )
        embed.set_footer(
            text=(
                "Unrealised move against the alert-time close · "
                "tracking only, not a position or a recommendation"
            )
        )
        await ctx.respond(embed=embed)


def _d(value, suffix: str = "", digits: int = 2) -> str:
    """Render a value, or an em dash for nulls.

    Repeats the formatter's convention rather than importing a private helper:
    nulls are '—', never 0, because a zero meaning "missing" is indistinguishable
    from a measured zero once it reaches a reader who cannot inspect the row.
    """
    if value is None:
        return "—"
    if isinstance(value, (int, float)):
        return f"{value:.{digits}f}{suffix}"
    return str(value)


def setup(bot) -> None:
    bot.add_cog(MomentumCog(bot))

"""§8.3's momentum scan job — posts candidates to the four bucket channels.

Routing is (bucket x side): penny-buy, penny-sell, market-buy, market-sell.

Three behaviours here are requirements rather than choices:

1. **An unset channel ID logs a warning and produces nothing. It never crashes
   the bot.** That is the existing convention in this service and it matters more
   for the momentum channels than the others, because there are four of them and
   a partial rollout — market channels configured, penny channels not — is the
   normal intermediate state rather than a misconfiguration.

2. **Thresholds are per bucket.** A global threshold does not mean the same thing
   in two buckets whose score distributions differ by nine points and whose base
   rates differ threefold; the pilot's global 60 selected penny candidates that
   hit LESS often than the penny base rate. See §4.4.

3. **Cooldown is per symbol per bucket**, reusing the existing Redis
   alert-cooldown machinery. §8.3: a symbol that stays qualified for a week must
   not alert daily. Keying on bucket as well as symbol means a name that migrates
   across the $2 boundary is not silently suppressed by the other bucket's key.
"""

from __future__ import annotations

import logging
from typing import TYPE_CHECKING, Any, Mapping, Sequence

from db import cache
from db.queries import momentum as mq
from notifier.discord import momentum as fmt

if TYPE_CHECKING:
    from config import BotConfig

log = logging.getLogger(__name__)

#: Cooldown key namespace. Includes the bucket so a symbol crossing the $2
#: boundary is not muted by the key it held in its previous bucket.
_COOLDOWN_KEY = "momentum:alert:{bucket}:{symbol}"


class MomentumScanJob:
    """Posts today's alertable candidates, deduplicated by Redis cooldown."""

    def __init__(
        self,
        cfg: "BotConfig",
        pool: Any,
        notifiers: "list[Any]",
    ) -> None:
        self._cfg = cfg
        self._pool = pool
        # Resolved from the notifier list rather than passed separately, matching
        # how the other jobs receive `notifiers`. Momentum routes to specific
        # channels via send_action, so it needs the Discord notifier itself
        # rather than the generic send_alert fan-out.
        self._discord = next(
            (n for n in notifiers if hasattr(n, "send_action")), None
        )

    def _redis(self) -> Any | None:
        """The shared Redis handle, or None when the cache is not initialised.

        Read through db.cache rather than held as an attribute so this job uses
        the same connection as the existing alert cooldown, instead of opening a
        second one with its own lifecycle.
        """
        try:
            return cache.get()
        except RuntimeError:
            return None

    # ── channel routing ──────────────────────────────────────────────────────

    def _channel_for(self, bucket: str, side: str) -> int | None:
        """Resolve one of the four channels, or None when unconfigured."""
        mapping = {
            ("penny", "buy"): getattr(self._cfg, "discord_penny_buy_channel_id", None),
            ("penny", "sell"): getattr(self._cfg, "discord_penny_sell_channel_id", None),
            ("market", "buy"): getattr(self._cfg, "discord_market_buy_channel_id", None),
            ("market", "sell"): getattr(self._cfg, "discord_market_sell_channel_id", None),
        }
        return mapping.get((bucket, side))

    # ── cooldown ─────────────────────────────────────────────────────────────

    async def _on_cooldown(self, symbol: str, bucket: str) -> bool:
        """True when this symbol+bucket alerted inside the cooldown window.

        With no Redis the job alerts every run rather than silently suppressing:
        a missing cache must not look like "nothing qualified". Noisy and correct
        beats quiet and wrong.
        """
        redis = self._redis()
        if redis is None:
            return False
        key = _COOLDOWN_KEY.format(bucket=bucket, symbol=symbol)
        try:
            return bool(await redis.exists(key))
        except Exception as exc:  # pragma: no cover - cache outage
            log.warning("momentum cooldown check failed for %s (%s); alerting anyway", symbol, exc)
            return False

    async def _mark_alerted(self, symbol: str, bucket: str) -> None:
        redis = self._redis()
        if redis is None:
            return
        key = _COOLDOWN_KEY.format(bucket=bucket, symbol=symbol)
        ttl = int(getattr(self._cfg, "bot_momentum_cooldown_secs", 5 * 24 * 3600))
        try:
            await redis.setex(key, ttl, "1")
        except Exception as exc:  # pragma: no cover - cache outage
            log.warning("momentum cooldown set failed for %s (%s)", symbol, exc)

    # ── run ──────────────────────────────────────────────────────────────────

    async def run(self) -> None:
        log.debug("running momentum scan")
        try:
            rows: Sequence[Mapping[str, Any]] = await mq.alertable_candidates(
                self._pool,
                min_score_market=int(getattr(self._cfg, "bot_momentum_min_score_market", 65)),
                min_score_penny=int(getattr(self._cfg, "bot_momentum_min_score_penny", 72)),
            )
        except Exception:
            log.exception("momentum candidate query failed")
            return

        if not rows:
            # Deliberately INFO with the freshness check attached: "nothing
            # qualified today" and "the scanner has not run since Tuesday" look
            # identical from an empty result, and only one is a market condition.
            try:
                fresh = await mq.scan_freshness(self._pool)
                log.info(
                    "momentum scan: no candidates cleared their bucket threshold "
                    "(last scan %s, %s symbols scored)",
                    (fresh or {}).get("last_scan_date"),
                    (fresh or {}).get("scored_today"),
                )
            except Exception:
                log.info("momentum scan: no candidates, and freshness check failed")
            return

        if self._discord is None:
            log.warning("no Discord notifier available; %d momentum candidates not posted", len(rows))
            return

        posted = skipped_cooldown = skipped_channel = 0
        for row in rows:
            symbol = row["symbol"]
            bucket = row["bucket"]

            if await self._on_cooldown(symbol, bucket):
                skipped_cooldown += 1
                continue

            channel_id = self._channel_for(bucket, "buy")
            if not channel_id:
                # Warning, not an error, and the loop continues: a partial
                # rollout is a normal state, not a failure.
                log.warning(
                    "no Discord channel configured for bucket=%s side=buy; %s not posted",
                    bucket,
                    symbol,
                )
                skipped_channel += 1
                continue

            try:
                embed = fmt.build_momentum_embed(row, embed_cls=self._embed_cls())
                await self._discord.send_action(channel_id, embed)
            except Exception:
                log.exception("failed posting momentum alert for %s", symbol)
                continue

            await self._mark_alerted(symbol, bucket)
            posted += 1

        log.info(
            "momentum scan: %d candidates, %d posted, %d on cooldown, %d unroutable",
            len(rows),
            posted,
            skipped_cooldown,
            skipped_channel,
        )

    @staticmethod
    def _embed_cls() -> Any:
        """Import discord.Embed lazily.

        Keeps the formatter and this job importable in a test environment without
        discord.py installed, which is what lets the output layer be unit-tested
        at all.
        """
        import discord

        return discord.Embed

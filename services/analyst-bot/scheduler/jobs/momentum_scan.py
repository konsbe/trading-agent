"""§8.3's momentum scan job — posts candidates to the four bucket channels.

Runs in SCREENER mode by default (BOT_MOMENTUM_ALERT_MODE): alerts fire on a
gate pass per bucket, not on a score threshold, and carry no score.

Routing is (bucket x side): penny-buy, penny-sell, market-buy, market-sell.
Only the BUY channels are ever posted to — see _channel_for for why the sell
channels exist and stay silent.

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
from datetime import date, datetime, timezone
from typing import TYPE_CHECKING, Any, Callable, Mapping, Sequence
from zoneinfo import ZoneInfo

from db import cache
from db.queries import momentum as mq
from notifier.discord import momentum as fmt

if TYPE_CHECKING:
    from config import BotConfig

log = logging.getLogger(__name__)

#: Cooldown key namespace. Includes the bucket so a symbol crossing the $2
#: boundary is not muted by the key it held in its previous bucket.
_COOLDOWN_KEY = "momentum:alert:{bucket}:{symbol}"

#: Marks a session as alerted, so the evening retries post it exactly once.
_SESSION_KEY = "momentum:session_alerted:{session}"
_SESSION_TTL_SECS = 7 * 24 * 3600

_NEW_YORK = ZoneInfo("America/New_York")


def session_just_closed(now: datetime) -> date | None:
    """The New York trading date an evening run should alert on, or None on a weekend.

    Holidays are NOT modelled here — the holiday calendar lives once, in
    data-analyzer. On a holiday there is simply no fresh scan for the date, so
    the freshness gate skips it; the log says "holiday, or the chain has not
    finished" rather than pretending to know which.
    """
    local = now.astimezone(_NEW_YORK)
    if local.weekday() >= 5:
        return None
    return local.date()


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
        #: Injectable clock, for tests.
        self._now: Callable[[], datetime] = lambda: datetime.now(timezone.utc)
        #: Fallback when Redis is unavailable, so retries in this process still
        #: post a session once.
        self._alerted_sessions: set[str] = set()

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
        """Resolve one of the four channels, or None when unconfigured.

        THE SELL CHANNELS ARE MAPPED BUT NEVER POSTED TO, deliberately.

        This job only ever calls with side="buy". The sell mapping stays so the
        configuration keeps its shape and so the decision is visible at the
        point someone would go to add sell posting — rather than them finding
        an absent key and assuming it was an oversight.

        §5's exit rules are not validated, and one of them is demonstrably
        broken: `breakout_failed` fires on session 1 at a median peak of 0.00%
        on 651 of 1,545 replayed positions (42%). It closes positions that
        never moved. Posting that into a channel is a trading instruction with
        no evidence behind it, which is a different and worse thing than a
        screener alert.

        Position tracking continues in `momentum_tracked` for research. The
        exits are Phase 2 §8's input, not output.
        """
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

    # ── burst cap ────────────────────────────────────────────────────────────

    def _cap(self) -> int:
        """Max candidates posted per channel per day, 0 to disable."""
        return int(getattr(self._cfg, "bot_momentum_max_alerts_per_channel", 10))

    @staticmethod
    def _apply_cap(rows: list, cap: int) -> tuple[list, int]:
        """Return (rows to post, count suppressed).

        Screener mode alerts on every gate pass, and gate passes are bursty:
        across ten years the mean is 4.5/day and the median 3, but the p90 is 9
        and the worst single day -- 2024-11-06, the session after the US
        election -- produced 157. A 157-message burst is unreadable,
        and worse, it arrives exactly on the days a reader most wants to skim
        the feed.

        The cap is applied AFTER the existing ordering, which is by RVOL
        descending, so the kept rows are the highest-RVOL ones. That ordering
        is descriptive — it is not a claim that the kept candidates are better,
        only that a cut has to fall somewhere and this one is reproducible and
        stated. The suppressed count is always posted, so a truncated day is
        never mistaken for a quiet one.
        """
        if cap <= 0 or len(rows) <= cap:
            return rows, 0
        return rows[:cap], len(rows) - cap

    # ── run ──────────────────────────────────────────────────────────────────

    async def _session_alerted(self, session: date) -> bool:
        key = _SESSION_KEY.format(session=session.isoformat())
        if key in self._alerted_sessions:
            return True
        r = self._redis()
        if r is None:
            return False
        try:
            return bool(await r.exists(key))
        except Exception:
            log.warning("momentum scan: session marker lookup failed; proceeding")
            return False

    async def _mark_session_alerted(self, session: date) -> None:
        key = _SESSION_KEY.format(session=session.isoformat())
        self._alerted_sessions.add(key)
        r = self._redis()
        if r is None:
            return
        try:
            await r.set(key, "1", ex=_SESSION_TTL_SECS)
        except Exception:
            log.warning("momentum scan: could not persist session marker %s", key)

    async def _fresh_session(self) -> date | None:
        """The session to alert on, or None (logged) when there is nothing fresh.

        Alerts fire only for the session that just closed. The previous job
        alerted on the latest stored scan whatever its date, so a scanner that
        had not run for days re-posted an old scan as if it were today's.
        """
        session = session_just_closed(self._now())
        if session is None:
            log.info("momentum scan: weekend — no session to alert on")
            return None
        try:
            latest = await mq.latest_scan_date(self._pool)
        except Exception:
            log.exception("momentum scan: could not read the latest scan date")
            return None
        if latest != session:
            log.info(
                "momentum scan: latest scan is %s, not today's session %s "
                "(market holiday, or momentum-daily has not finished yet) — not alerting",
                latest, session,
            )
            return None
        if await self._session_alerted(session):
            log.debug("momentum scan: session %s already alerted", session)
            return None
        return session

    async def run(self) -> None:
        log.debug("running momentum scan")
        session = await self._fresh_session()
        if session is None:
            return
        try:
            mode = str(getattr(self._cfg, "bot_momentum_alert_mode", "screener")).lower()
            select = (
                mq.alertable_candidates_by_score
                if mode == "score"
                else mq.alertable_candidates
            )
            if mode not in ("screener", "score"):
                # Unknown value falls back to the SAFER mode and says so. A typo
                # must not silently re-enable score-threshold alerting, which
                # makes a ranking claim the evidence does not support.
                log.warning(
                    "BOT_MOMENTUM_ALERT_MODE=%r is not screener|score; using screener", mode
                )
            rows: Sequence[Mapping[str, Any]] = await select(
                self._pool,
                min_score_market=int(getattr(self._cfg, "bot_momentum_min_score_market", 65)),
                min_score_penny=int(getattr(self._cfg, "bot_momentum_min_score_penny", 72)),
                scan_date=session,
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
            await self._mark_session_alerted(session)
            return

        if self._discord is None:
            # Not marked as alerted: a later run with a notifier should still post.
            log.warning("no Discord notifier available; %d momentum candidates not posted", len(rows))
            return

        # Cooldown is applied BEFORE the cap, so a suppressed-by-cooldown symbol
        # does not consume one of the day's slots. Doing it the other way round
        # would let a handful of repeat symbols crowd out everything new.
        eligible: dict[str, list] = {}
        skipped_cooldown = 0
        for row in rows:
            if await self._on_cooldown(row["symbol"], row["bucket"]):
                skipped_cooldown += 1
                continue
            eligible.setdefault(row["bucket"], []).append(row)

        cap = self._cap()
        posted = skipped_channel = suppressed = 0

        for bucket, bucket_rows in eligible.items():
            # Per CHANNEL, and buckets route to different channels, so each gets
            # its own allowance. A global cap would let a busy market day starve
            # the penny channel entirely.
            to_post, over = self._apply_cap(bucket_rows, cap)
            suppressed += over

            channel_id = self._channel_for(bucket, "buy")
            if not channel_id:
                # Warning, not an error, and the loop continues: a partial
                # rollout is a normal state, not a failure.
                log.warning(
                    "no Discord channel configured for bucket=%s side=buy; %d candidates not posted",
                    bucket,
                    len(bucket_rows),
                )
                skipped_channel += len(bucket_rows)
                continue

            for row in to_post:
                symbol = row["symbol"]
                try:
                    embed = fmt.build_momentum_embed(row, embed_cls=self._embed_cls())
                    await self._discord.send_action(channel_id, embed)
                except Exception:
                    log.exception("failed posting momentum alert for %s", symbol)
                    continue
                await self._mark_alerted(symbol, bucket)
                posted += 1

            if over:
                # Always posted when anything was cut. A truncated day must not
                # be indistinguishable from a quiet one — that is the same
                # "empty result is ambiguous" failure the freshness check exists
                # for, one level down.
                try:
                    await self._discord.send_action(
                        channel_id,
                        f"…and {over} more {bucket} candidate(s) today — see /scanner",
                    )
                except Exception:
                    log.exception("failed posting overflow summary for bucket=%s", bucket)

        await self._mark_session_alerted(session)
        log.info(
            "momentum scan: session %s — %d candidates, %d posted, %d on cooldown, %d unroutable, %d over cap",
            session,
            len(rows),
            posted,
            skipped_cooldown,
            skipped_channel,
            suppressed,
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

"""
Optional scheduled job: fetch FOMC (or other) statement HTML, score hawkish/dovish via OpenAI,
and insert into narrative_scores for the Macro intel Discord embed.

Requires OPENAI_API_KEY, FOMC_STATEMENT_URL, and BOT_FOMC_NARRATIVE_ENABLE=true.
"""
from __future__ import annotations

import json
import logging
import re
from typing import TYPE_CHECKING, Any, Optional

import httpx

if TYPE_CHECKING:
    import asyncpg

    from config import BotConfig

log = logging.getLogger(__name__)


def _strip_html(html: str, max_chars: int = 38000) -> str:
    t = re.sub(r"(?is)<script[^>]*>.*?</script>", " ", html)
    t = re.sub(r"(?is)<style[^>]*>.*?</style>", " ", t)
    t = re.sub(r"<[^>]+>", " ", t)
    t = re.sub(r"\s+", " ", t).strip()
    return t[:max_chars]


def _parse_llm_json(raw: str) -> Optional[dict[str, Any]]:
    raw = raw.strip()
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        pass
    m = re.search(r"\{[^{}]*\}", raw, re.DOTALL)
    if m:
        try:
            return json.loads(m.group(0))
        except json.JSONDecodeError:
            return None
    return None


class FomcNarrativeJob:
    def __init__(self, cfg: "BotConfig", db_pool: "asyncpg.Pool") -> None:
        self._cfg = cfg
        self._pool = db_pool

    async def run(self) -> None:
        if not self._cfg.bot_fomc_narrative_enable:
            return
        key = self._cfg.openai_api_key.strip()
        url = self._cfg.fomc_statement_url.strip()
        if not key or not url:
            log.debug("FOMC narrative skipped — missing OPENAI_API_KEY or FOMC_STATEMENT_URL")
            return

        log.info("FOMC narrative job: fetching %s", url)
        try:
            async with httpx.AsyncClient(timeout=60.0, follow_redirects=True) as client:
                r = await client.get(url, headers={"User-Agent": "trading-agent-analyst-bot/1.0"})
                r.raise_for_status()
                text = _strip_html(r.text)
        except Exception as exc:
            log.error("FOMC narrative fetch failed: %s", exc)
            return

        if len(text) < 200:
            log.warning("FOMC narrative: extracted text too short (%d chars)", len(text))
            return

        from openai import AsyncOpenAI

        oai = AsyncOpenAI(api_key=key)
        model = self._cfg.openai_model
        prompt = (
            "You are a macro policy analyst. Read the following central-bank policy statement "
            "excerpt. Reply with a single JSON object only (no markdown), keys:\n"
            '  "hawkish_score": number from -1.0 (max dovish) to +1.0 (max hawkish),\n'
            '  "summary": string, at most 2 sentences, factual and neutral.\n\n'
            f"Statement excerpt:\n{text}"
        )
        try:
            comp = await oai.chat.completions.create(
                model=model,
                messages=[{"role": "user", "content": prompt}],
                temperature=0.2,
                max_tokens=400,
            )
            raw_out = (comp.choices[0].message.content or "").strip()
        except Exception as exc:
            log.error("OpenAI FOMC narrative failed: %s", exc)
            return

        data = _parse_llm_json(raw_out)
        if not data:
            log.warning("FOMC narrative: could not parse JSON from model output")
            return

        score = data.get("hawkish_score")
        summary = data.get("summary")
        if summary is None:
            log.warning("FOMC narrative: missing summary in JSON")
            return
        try:
            score_f = float(score) if score is not None else None
        except (TypeError, ValueError):
            score_f = None

        from db.queries import macro_intel

        try:
            await macro_intel.insert_narrative_score(
                self._pool,
                doc_kind="fomc_statement",
                source_url=url,
                title="FOMC statement (scheduled)",
                llm_score=score_f,
                llm_summary=str(summary)[:4000],
                model=model,
                payload={"chars": len(text)},
            )
        except Exception as exc:
            log.error("FOMC narrative DB insert failed: %s", exc)
            return

        log.info("FOMC narrative stored score=%s", score_f)

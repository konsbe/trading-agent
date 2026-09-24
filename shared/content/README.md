# shared/content

Text that more than one service must render identically.

| File | Read by |
|------|---------|
| `momentum_caveats.json` | analyst-bot (`notifier/discord/momentum.py`) and momentum-api (`services/data-analyzer/cmd/momentum-api`) |

`momentum_caveats.json` is the single source of `EVIDENCE_CAVEAT`,
`RESEARCH_SCORE_CAVEAT` and the per-rule `exit_reason_notes` served with tracked
positions (one note per §5 exit reason; each states the rule, and the three Phase
1 §10.1.9 flagged — `breakout_failed`, `stop_atr`, `timeout` — carry their
specific finding. `lost_vwap` and `momentum_stalled` are plain definitions,
because no issue was found with them). Neither service has a fallback copy: both refuse to
start if the file is missing, and tests on both sides assert against it. Edit the
text here and nowhere else.

**Both services need it at runtime.** In Docker it is mounted read-only at
`/shared/content` with `MOMENTUM_CAVEATS_PATH=/shared/content/momentum_caveats.json`
(see `infra/docker-compose.yml`). A deployment without that mount stops the bot at
boot. See "Cross-service runtime dependencies" in
`services/data-ingestion/data_ingestion.md`.

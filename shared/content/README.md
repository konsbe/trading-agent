# shared/content

Text that more than one service must render identically.

| File | Read by |
|------|---------|
| `momentum_caveats.json` | analyst-bot (`notifier/discord/momentum.py`) and momentum-api (`services/data-analyzer/cmd/momentum-api`) |
| `handbook.json` | momentum-api, `GET /api/v1/education/handbook` (+ glossary) — draft content, `docs/EDUCATION_SECTION_CONTENT_SPEC.md` §1 |
| `masterclass.json` | momentum-api, `GET /api/v1/education/masterclass` (+ glossary) — draft content, spec §2 |

**Education content format.** Each Handbook entry and MasterClass entry is a
list of `blocks`: `paragraph` (`text`), `heading` (`text`), `list` (`items`), and
— Handbook only — `caveat` (`key` into `momentum_caveats.json`, e.g.
`heuristic_ta_caveat`). Text may use `**bold**` and `*italic*`, nothing else.
Caveat text is never typed into these files: the loader resolves the key, so a
Handbook page can never drift from the caveat every other surface shows.
MasterClass entries also carry a `summary` (the at-most-three-line version shown
first). `terms` is reserved for the Glossary, which is extracted from both files
at load, never authored separately. MasterClass must not contain any of this
app's field names or output labels.

`momentum_caveats.json` is the single source of `EVIDENCE_CAVEAT`,
`RESEARCH_SCORE_CAVEAT`, `HEURISTIC_TA_CAVEAT` (key `heuristic_ta_caveat`,
served by momentum-api above every `heuristic_signals` section; the bot does not
read it yet) and the per-rule `exit_reason_notes` served with tracked
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

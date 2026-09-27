# shared/content

Text that more than one service must render identically.

| File | Read by |
|------|---------|
| `momentum_caveats.json` | analyst-bot (`notifier/discord/momentum.py`) and momentum-api (`services/data-analyzer/cmd/momentum-api`) |
| `handbook.json` | momentum-api, `GET /api/v1/education/handbook` (+ glossary) — draft content, `docs/EDUCATION_SECTION_CONTENT_SPEC.md` §1 |
| `masterclass.json` | momentum-api, `GET /api/v1/education/masterclass` (+ glossary) — draft content, spec §2 |
| `market_report_descriptions.json` | momentum-api (`GET /api/v1/market-report/today`, `MOMENTUM_MARKET_REPORT_TEXT_PATH`) and analyst-bot (`reports/market_text.py`, `MARKET_REPORT_TEXT_PATH`) — the Daily Market Report's one-line descriptions per stored code (market-cycle composite, macro regime, intermarket pairs, seasonality disclaimer). Both surfaces replace the stored macro-row text with these; an unknown code keeps its stored text. No fallback copy on either side; a test in momentum-api fails when a producer emits a code the file does not describe |
| `correlation_sentences.json` | momentum-api (`GET /api/v1/scanner/today/{symbol}/analysis`, `MOMENTUM_CORRELATION_TEXT_PATH`) and analyst-bot (`reports/correlation_text.py`, `CORRELATION_TEXT_PATH`) — each correlation aligned/divergent sentence fundamental-analysis stores (its format string; `%.1f` = a rendered number) and the text both surfaces show for it. A sentence the file does not know is shown as stored. No fallback copy; a momentum-api test parses `internal/fundamental/analyze.go` and fails when a stored sentence and the file drift apart |
| `correlation_labels.json` | momentum-api (served with the analysis as `name_label` / `tier_label` / `composite_label` / `net_label` / `fired_labels` / `labels`, `MOMENTUM_CORRELATION_LABELS_PATH`; mfe-scanner renders them as served and keeps no copy) and analyst-bot (`reports/correlation_labels.py`, `CORRELATION_LABELS_PATH`) — the correlation card's labels by stored code: cluster names, cluster tiers, the not-evaluated wording (a cluster that ran no check), pattern names, net-count wording and headings. A momentum-api test parses `analyze.go` and fails when a tier, pattern or net-signal code and the file differ |

**Editing these files.** Rewrite a JSON file here the way it is stored — 2-space indent, non-ASCII characters kept as-is (not `\u` escapes), and a trailing newline (Python: `json.dumps(data, ensure_ascii=False, indent=2) + "\n"`). That round-trips `handbook.json` byte-for-byte, so an edit changes only what it edits instead of reformatting the whole file. Most of these files are loaded at start-up by both momentum-api and analyst-bot, and both refuse to start on a missing or malformed file: validate the JSON (e.g. `python3 -m json.tool FILE >/dev/null`) and run the content tests on both sides before deploying.

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

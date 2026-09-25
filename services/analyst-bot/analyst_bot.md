# analyst-bot — known issues

Open defects in the bot, one row each. Fixed rows move to the commit that fixed them.

| Issue | Current state | Fix |
|---|---|---|
| **RSI action rule never counts RSI divergence or support/resistance** | `actions/rules/rsi.py` reads `indicators["rsi_divergence"]` (payload `divergence_type`) and `indicators["support_resistance"]` (payload `near_support` / `near_resistance`). technical-analysis writes `rsi_divergence_rsi14_sw5` (payload `bullish_regular`, `bearish_regular`, `kind`) and `sr_levels` (payload `support`, `resistance`, `support_touches`, `resistance_touches`, `current_price`). Neither lookup ever matches, so on live data only trend and the FA composite tier can add confluence: `rsi_oversold` reaches BUY_WATCH only with uptrend + strong FA, `rsi_overbought` reaches TRIM_WATCH only with downtrend + weak FA. Logged 2026-09-25 | Divergence: look up the `rsi_divergence_` prefix and read `bullish_regular` / `bearish_regular`. Support/resistance is not a rename: the worker stores levels, not "near" flags, so a proximity rule has to be defined (e.g. price within N% or N×ATR of the level) before the rule can use it. Independent of the heuristic-signals validation, which does not replay the RSI rule (it needs point-in-time FA tiers). Same class as the MACD lookup bug fixed in `fbaee0a` |

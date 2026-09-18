#!/usr/bin/env bash
# Verify a daily-bar source is usable for the momentum scanner.
#
# Spec: docs/MOMENTUM_SCANNER_PHASE1.md §2.2. Two questions, in order:
#
#   1. REACHABILITY — does this network get a real answer from the provider?
#   2. CONSOLIDATED VOLUME — is the volume all US venues, or one venue?
#
# The second question is the one that is easy to skip and fatal to get wrong.
# Every volume feature in §3 (RVOL, acceleration, dollar volume) is meaningless
# on single-venue volume, which is exactly why §2.2 rejects Alpaca's free tier.
# A fallback that quietly supplies IEX-only volume would leave the whole scanner
# producing confident, wrong numbers.
#
# The check: AAPL trades roughly 40–60 million shares a day consolidated. IEX
# alone is a low single-digit percentage of that, so a single-venue feed reports
# ~1–3 million. That is a 20-40x gap — unmistakable, and it needs no second
# provider to compare against.
#
# Usage:
#   ./verify-bar-source.sh yahoo
#   ./verify-bar-source.sh tiingo    # reads TIINGO_API_KEY
#   ./verify-bar-source.sh polygon   # reads POLYGON_API_KEY
#   ./verify-bar-source.sh twelve    # reads TWELVE_DATA_API_KEY
#   ./verify-bar-source.sh eodhd     # reads EODHD_API_KEY
#   ./verify-bar-source.sh all
#
# Keys are read from the environment, so source .env first:
#   set -a; . ./.env; set +a; ./services/data-ingestion/scripts/verify-bar-source.sh all
#
# Run it from the network the backfill will actually use. Yahoo's result in
# particular is egress-dependent: it returns a blanket 429 from some corporate
# networks regardless of request rate, which is an IP-reputation block and not
# something a rate setting can fix.
set -uo pipefail

SYMBOL="${SYMBOL:-AAPL}"
UA='Mozilla/5.0 (X11; Linux x86_64; rv:125.0) Gecko/20100101 Firefox/125.0'

# Expected consolidated daily volume for the probe symbol, order of magnitude.
CONSOLIDATED_MIN="${CONSOLIDATED_MIN:-10000000}"  # 10M — comfortably above any single venue
SINGLE_VENUE_MAX="${SINGLE_VENUE_MAX:-5000000}"   # 5M  — comfortably above IEX's typical share

note()  { printf '  %s\n' "$*"; }
head2()  { printf '\n%s\n' "$*"; }

# verdict <provider> <volume>
# Turns a reported volume into the §2.2 answer.
verdict() {
  local provider="$1" vol="$2"
  if [ -z "$vol" ] || [ "$vol" = "null" ] || [ "$vol" = "0" ]; then
    note "VOLUME:      unavailable — cannot assess consolidation"
    return 1
  fi
  printf '  VOLUME:      %s shares (%s)\n' "$vol" "$SYMBOL"
  if [ "$(printf '%.0f' "$vol")" -ge "$CONSOLIDATED_MIN" ]; then
    note "CONSOLIDATED: YES — magnitude is consistent with all-venue volume"
    return 0
  elif [ "$(printf '%.0f' "$vol")" -le "$SINGLE_VENUE_MAX" ]; then
    note "CONSOLIDATED: NO  — looks single-venue (IEX-scale). UNUSABLE per §2.2:"
    note "              every §3 volume feature would be wrong while looking fine."
    return 1
  else
    note "CONSOLIDATED: AMBIGUOUS — between thresholds; compare against a known-good source"
    return 1
  fi
}

probe_yahoo() {
  head2 "── yahoo (no key; §2.2's designated primary) ──"
  local from to body code
  to=$(date +%s); from=$((to - 30*86400))
  body=$(curl -s -w '\n%{http_code}' --max-time 20 \
    -H "User-Agent: $UA" -H 'Accept: application/json, */*' \
    "https://query1.finance.yahoo.com/v8/finance/chart/${SYMBOL}?interval=1d&period1=${from}&period2=${to}&includePrePost=false" 2>/dev/null)
  code=$(printf '%s' "$body" | tail -1)
  note "HTTP:        ${code}"
  if [ "$code" = "429" ]; then
    note "BLOCKED:     blanket 429. This is IP reputation, not rate limiting —"
    note "             lowering the request rate will not help. Retry from a"
    note "             different egress (hotspot / home) to confirm."
    return 1
  fi
  [ "$code" = "200" ] || { note "unexpected status; body: $(printf '%s' "$body" | head -c 160)"; return 1; }
  local vol
  vol=$(printf '%s' "$body" | sed '$d' | python3 -c '
import json,sys
try:
    d=json.load(sys.stdin); r=d["chart"]["result"][0]
    v=[x for x in r["indicators"]["quote"][0]["volume"] if x]
    print(int(v[-1]) if v else "")
except Exception: print("")
' 2>/dev/null)
  verdict yahoo "$vol"
}

probe_tiingo() {
  head2 "── tiingo (API-key auth, so not IP-reputation dependent) ──"
  local tok="${1:-}" body code
  [ -n "$tok" ] || { note "no token supplied; skipping"; return 2; }
  body=$(curl -s -w '\n%{http_code}' --max-time 25 \
    "https://api.tiingo.com/tiingo/daily/${SYMBOL}/prices?token=${tok}" 2>/dev/null)
  code=$(printf '%s' "$body" | tail -1); note "HTTP:        ${code}"
  [ "$code" = "200" ] || { note "body: $(printf '%s' "$body" | sed '$d' | head -c 160)"; return 1; }
  local vol
  vol=$(printf '%s' "$body" | sed '$d' | python3 -c '
import json,sys
try:
    d=json.load(sys.stdin); print(int(d[-1].get("volume") or 0))
except Exception: print("")
' 2>/dev/null)
  verdict tiingo "$vol"
}

probe_polygon() {
  head2 "── polygon.io (API-key auth) ──"
  local key="${1:-}" body code to from
  [ -n "$key" ] || { note "no key supplied; skipping"; return 2; }
  to=$(date -u +%F); from=$(date -u -d '30 days ago' +%F 2>/dev/null || date -u -v-30d +%F)
  body=$(curl -s -w '\n%{http_code}' --max-time 25 \
    "https://api.polygon.io/v2/aggs/ticker/${SYMBOL}/range/1/day/${from}/${to}?adjusted=true&sort=asc&apiKey=${key}" 2>/dev/null)
  code=$(printf '%s' "$body" | tail -1); note "HTTP:        ${code}"
  [ "$code" = "200" ] || { note "body: $(printf '%s' "$body" | sed '$d' | head -c 160)"; return 1; }
  local vol
  vol=$(printf '%s' "$body" | sed '$d' | python3 -c '
import json,sys
try:
    d=json.load(sys.stdin); rs=d.get("results") or []
    print(int(rs[-1].get("v") or 0) if rs else "")
except Exception: print("")
' 2>/dev/null)
  verdict polygon "$vol"
}

probe_twelve() {
  head2 "── twelve data (API-key auth) ──"
  local key="${1:-}" body code
  [ -n "$key" ] || { note "no key supplied; skipping"; return 2; }
  body=$(curl -s -w '\n%{http_code}' --max-time 25 \
    "https://api.twelvedata.com/time_series?symbol=${SYMBOL}&interval=1day&outputsize=30&apikey=${key}" 2>/dev/null)
  code=$(printf '%s' "$body" | tail -1); note "HTTP:        ${code}"
  [ "$code" = "200" ] || { note "body: $(printf '%s' "$body" | sed '$d' | head -c 160)"; return 1; }
  local vol
  vol=$(printf '%s' "$body" | sed '$d' | python3 -c '
import json,sys
try:
    d=json.load(sys.stdin); vs=d.get("values") or []
    print(int(float(vs[0].get("volume") or 0)) if vs else "")
except Exception: print("")
' 2>/dev/null)
  verdict twelve "$vol"
}

probe_eodhd() {
  head2 "── eodhd (API-key auth) ──"
  local key="${1:-}" body code
  [ -n "$key" ] || { note "no key supplied; skipping"; return 2; }
  body=$(curl -s -w '\n%{http_code}' --max-time 25 \
    "https://eodhd.com/api/eod/${SYMBOL}.US?api_token=${key}&fmt=json&period=d" 2>/dev/null)
  code=$(printf '%s' "$body" | tail -1); note "HTTP:        ${code}"
  [ "$code" = "200" ] || { note "body: $(printf '%s' "$body" | sed '$d' | head -c 160)"; return 1; }
  local vol
  vol=$(printf '%s' "$body" | sed '$d' | python3 -c '
import json,sys
try:
    d=json.load(sys.stdin); print(int(d[-1].get("volume") or 0)) if d else print("")
except Exception: print("")
' 2>/dev/null)
  verdict eodhd "$vol"
}

printf 'Bar-source verification — probe symbol %s\n' "$SYMBOL"
printf 'Consolidated if >= %s shares/day; single-venue if <= %s\n' "$CONSOLIDATED_MIN" "$SINGLE_VENUE_MAX"

case "${1:-all}" in
  yahoo)   probe_yahoo ;;
  tiingo)  probe_tiingo "${2:-${TIINGO_API_KEY:-}}" ;;
  polygon) probe_polygon "${2:-${POLYGON_API_KEY:-}}" ;;
  twelve)  probe_twelve "${2:-${TWELVE_DATA_API_KEY:-}}" ;;
  eodhd)   probe_eodhd "${2:-${EODHD_API_KEY:-}}" ;;
  all)
    probe_yahoo   || true
    probe_tiingo  "${TIINGO_API_KEY:-}"      || true
    probe_polygon "${POLYGON_API_KEY:-}"     || true
    probe_twelve  "${TWELVE_DATA_API_KEY:-}" || true
    probe_eodhd   "${EODHD_API_KEY:-}"       || true
    ;;
  *) printf 'unknown provider: %s\n' "$1"; exit 2 ;;
esac

head2 "Record the outcome in services/data-ingestion/data_ingestion.md — including"
note "a clean 'yahoo works from here', since that is an operational fact the next"
note "person needs before spending an hour reproducing a blocked result."

# Discord Bot — Setup & Configuration Guide

This document explains how to create a Discord bot application, obtain every required credential, configure your channels, and understand how the analyst-bot interacts with Discord.

---

## Step 1 — Create a Discord Application & Bot

### 1.1 Open the Developer Portal

Go to [https://discord.com/developers/applications](https://discord.com/developers/applications) and log in with your Discord account.

### 1.2 Create a New Application

1. Click **"New Application"** (top-right)
2. Name it (e.g. `trading-analyst-bot`)
3. Accept the terms → click **"Create"**

### 1.3 Create the Bot User

1. In the left sidebar click **"Bot"**
2. Click **"Add Bot"** → **"Yes, do it!"**
3. Under **"Token"** click **"Reset Token"** → copy the token and **save it somewhere safe** — you will only see it once

```env
DISCORD_BOT_TOKEN=your_token_here
```

> **Security**: Never commit your token to git. The `.env` file is already in `.gitignore`.
> If you accidentally expose it, immediately click **"Reset Token"** to invalidate the old one.

### 1.4 Bot Settings (recommended)

Still on the **Bot** page:

| Setting | Value | Why |
|---|---|---|
| **Public Bot** | OFF | Prevents others from adding your bot to their servers |
| **Requires OAuth2 Code Grant** | OFF | Not needed |
| **Message Content Intent** | OFF | Bot uses slash commands, not message reading |
| **Server Members Intent** | OFF | Not required |
| **Presence Intent** | OFF | Not required |

---

## Step 2 — Invite the Bot to Your Server

### 2.1 Generate an Invite URL

1. In the left sidebar click **"OAuth2"** → **"URL Generator"**
2. Under **Scopes** check:
   - `bot`
   - `applications.commands` ← required for slash commands
3. Under **Bot Permissions** check:
   - `Send Messages`
   - `Embed Links`
   - `Read Message History`
   - `View Channels`
4. Copy the generated URL at the bottom

### 2.2 Invite

Open the URL in your browser → select your server → click **"Authorise"**.

The bot will appear in your server's member list as **offline** (it becomes online when `main.py` runs).

---

## Step 3 — Obtain the Guild (Server) ID

The Guild ID is your Discord server's unique identifier.

### How to get it

1. In Discord, open **User Settings** → **Advanced** → enable **"Developer Mode"**
2. Right-click your **server name** (in the left sidebar)
3. Click **"Copy Server ID"**

```env
DISCORD_GUILD_ID=123456789012345678
```

> **Optional**: If set, slash commands register instantly on that specific server (useful during development). If left empty, commands register globally (takes up to 1 hour to propagate).

---

## Step 4 — Create Channels & Obtain Channel IDs

The bot posts to three dedicated channels. Create them in your Discord server first.

### Recommended channel structure

```
📊 Trading
 ├── 📈 #daily-report       ← full market report every morning
 ├── 🚨 #alerts             ← threshold breach alerts (RSI, squeeze, FA flip)
 └── 💬 #commands           ← where users run /price /analyze /signals
```

### How to get a Channel ID

1. Make sure **Developer Mode** is enabled (Step 3.1)
2. Right-click the **channel name** in the sidebar
3. Click **"Copy Channel ID"**

```env
DISCORD_DAILY_REPORT_CHANNEL_ID=111122223333444455
DISCORD_ALERTS_CHANNEL_ID=555566667777888899
DISCORD_COMMANDS_CHANNEL_ID=000011112222333344
```

> `DISCORD_COMMANDS_CHANNEL_ID` is informational — slash commands work in any channel the bot has access to. You can restrict slash commands to specific channels via Discord's server settings (Integration Permissions).

---

## Step 5 — Full `.env` Configuration

Add these to your `.env` file at the repository root:

```env
# ── Discord ───────────────────────────────────────────────────────────────────

# Bot token from Step 1.3
DISCORD_BOT_TOKEN=your_bot_token_here

# Server (Guild) ID from Step 3
DISCORD_GUILD_ID=123456789012345678

# Channel IDs from Step 4
DISCORD_DAILY_REPORT_CHANNEL_ID=111122223333444455
DISCORD_ALERTS_CHANNEL_ID=555566667777888899
DISCORD_COMMANDS_CHANNEL_ID=000011112222333344

# ── Symbols the bot monitors ──────────────────────────────────────────────────
BOT_EQUITY_SYMBOLS=AAPL,MSFT,SPY
BOT_CRYPTO_SYMBOLS=BTCUSDT,ETHUSDT
BOT_EQUITY_INTERVAL=1Day
BOT_CRYPTO_INTERVAL=1d

# ── Scheduled reports ─────────────────────────────────────────────────────────
# Cron format (UTC): minute hour day-of-month month day-of-week
BOT_DAILY_REPORT_CRON=0 7 * * *        # 07:00 UTC daily
BOT_WEEKLY_DIGEST_CRON=0 8 * * 1       # Monday 08:00 UTC
BOT_ALERT_SCAN_INTERVAL=300            # every 5 minutes

# ── Alert thresholds ─────────────────────────────────────────────────────────
BOT_RSI_OVERSOLD=30
BOT_RSI_OVERBOUGHT=70
BOT_VIX_ALERT_THRESHOLD=25
BOT_BB_SQUEEZE_ALERT=true
BOT_ALERT_COOLDOWN_SECS=14400          # 4 hours — same alert won't fire again within this window

# ── Redis cache TTLs (seconds) ────────────────────────────────────────────────
BOT_CACHE_PRICE_TTL=300                # /price cache: 5 min
BOT_CACHE_ANALYZE_TTL=600             # /analyze cache: 10 min
BOT_CACHE_DAILY_REPORT_TTL=3600       # daily report cache: 1 hour

# ── Report limits ────────────────────────────────────────────────────────────
BOT_NEWS_HEADLINES_LIMIT=5
BOT_REPORT_MAX_SYMBOLS=10
```

---

## Step 6 — Start the Bot

```bash
# Start everything (DB + ingestion + analyzer + bot)
make up && make up-bot

# Or start only the bot (DB must already be running with data)
make up-bot

# Check logs
make log-bot
```

Expected startup log:

```
INFO  main           initialising DB pool
INFO  main           initialising Redis cache
INFO  scheduler      daily report scheduled: 0 7 * * *
INFO  scheduler      alert scan scheduled every 300s
INFO  bot.client     Bot ready — logged in as trading-analyst-bot#1234 (id=987654321...)
```

---

## How the Bot Works

### Slash Commands (user-triggered)

All commands work in any channel the bot has access to. Use `/` in Discord to see them.

| Command | Usage | What it returns |
|---|---|---|
| `/price` | `/price AAPL` or `/price BTCUSDT equity` | Latest OHLCV price bar — open, high, low, close, volume, daily change % |
| `/signals` | `/signals AAPL` | Fast one-embed summary: RSI, trend, MACD cross, VIX regime, FA composite score |
| `/analyze` | `/analyze AAPL` | Full multi-panel analysis: price → technical → fundamentals → sentiment + news |
| `/report` | `/report` | Triggers the daily market report immediately (same as the 07:00 job) |
| `/status` | `/status` | Bot health: DB ✅/❌, Redis ✅/❌, scheduler jobs, configured symbols |
| `/ping` | `/ping` | Bot latency in milliseconds |

**Cache behaviour**: `/price` and `/analyze` results are cached in Redis so repeated calls within the TTL window don't query TimescaleDB. Cache is bypassed for `/report`.

### Scheduled Posts (automatic)

#### Daily Report — `#daily-report` channel
- Fires at `BOT_DAILY_REPORT_CRON` (default 07:00 UTC)
- Posts one header embed (macro: VIX, 10Y yield, EUR/USD) followed by one compact embed per symbol
- Each symbol embed shows: price + change %, trend direction, RSI, MACD cross, Bollinger Squeeze, FA composite score, sentiment, top headline

#### Alert Scanner — `#alerts` channel
- Runs every `BOT_ALERT_SCAN_INTERVAL` seconds (default 5 min)
- Checks every symbol × interval combination
- Posts an alert embed if any condition is breached
- **Dedup via Redis**: the same alert for the same symbol won't fire again until `BOT_ALERT_COOLDOWN_SECS` expires (default 4 hours)

| Alert type | Trigger condition | Severity |
|---|---|---|
| `rsi_oversold` | RSI < `BOT_RSI_OVERSOLD` (default 30) | warning |
| `rsi_overbought` | RSI > `BOT_RSI_OVERBOUGHT` (default 70) | warning |
| `bb_squeeze` | Bollinger Bands inside Keltner Channels | info |
| `vix_elevated` | VIX > `BOT_VIX_ALERT_THRESHOLD` (default 25) | warning |
| `fa_tier_flip` | FA composite score changed tier (e.g. neutral → weak) | warning/info |
| `liquidity_sweep` | Liquidity sweep detected in technical indicators | info |

---

## Channel Permission Checklist

For each channel the bot posts to, verify it has these permissions:

| Permission | Required for |
|---|---|
| **View Channel** | All channels |
| **Send Messages** | All channels |
| **Embed Links** | All channels (bot uses embeds exclusively) |
| **Read Message History** | Optional (for button interactions in future Phase 2) |

To set permissions:
1. Right-click the channel → **Edit Channel** → **Permissions**
2. Click the **+** next to your bot's role (or the `@everyone` role if the bot uses that)
3. Enable the permissions above

---

## Restricting Slash Commands to Specific Channels

By default slash commands work in every channel. To restrict `/analyze` to only `#commands`:

1. Go to **Server Settings** → **Integrations**
2. Find `trading-analyst-bot` → click **Manage**
3. Under each command, click **"Add channels"** and select `#commands`

This does not affect scheduled posts — those are sent by the bot directly, not via slash commands.

---

## Troubleshooting

| Problem | Likely cause | Fix |
|---|---|---|
| Bot appears offline | `main.py` not running / `DISCORD_BOT_TOKEN` wrong | Check `make log-bot` |
| Slash commands not appearing | Commands not registered yet | Wait up to 1 hour (global) or set `DISCORD_GUILD_ID` for instant registration |
| `DISCORD_DAILY_REPORT_CHANNEL_ID not set` in logs | Channel ID missing from `.env` | Copy the channel ID (Step 4) and add it |
| No data in embeds | DB empty or data-ingestion not running | Run `make up-ingestion` and wait for backfill |
| Alert fires every scan | Redis not running | Check `make log-bot` for Redis connection error |
| `403 Forbidden` when posting | Bot missing **Send Messages** or **Embed Links** permission | Fix channel permissions (see above) |
| Token invalid error | Token was reset after copying | Go to Developer Portal → Bot → Reset Token → update `.env` |

---

## Adding a Second Platform (e.g. Telegram)

The bot is designed to send the same reports to multiple platforms simultaneously. When you're ready:

1. Fill in `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHAT_ID` in `.env`
2. Open `services/analyst-bot/notifier/telegram/notifier.py` — it already has the stub
3. Implement `send_daily_report`, `send_alert`, `send_symbol_report`
4. Create `notifier/telegram/formatter.py` to convert report models → Telegram MarkdownV2
5. In `main.py`, uncomment the `TelegramNotifier` import and append it to the `notifiers` list

No changes to the scheduler, report builder, or Discord code are needed.

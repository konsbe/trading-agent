# svc-mcp

Read-only remote MCP server behind the claude.ai custom connector. Claude (from Anthropic's cloud) calls `https://mcp.<your-domain>/mcp` over MCP streamable HTTP; a Cloudflare named tunnel carries that to this service on the Compose network, and the service answers from `momentum-api` with GET requests only. momentum-api itself stays private (`127.0.0.1:8090`, no auth, never routed by the tunnel).

```
claude.ai ──HTTPS──► Cloudflare edge (WAF: Anthropic IPs on /mcp,/token)
                         │ outbound-only tunnel
                     cloudflared ──(mcp-edge network)──► svc-mcp :8095 ──GET──► momentum-api :8090
```

> **Auth deviation:** this service does not use Keycloak. There is no `auth/` deployment in this repo, and GitHub sign-in was chosen explicitly for this connector. svc-mcp is its own single-client, single-user OAuth 2.1 authorization server; GitHub only answers "who signed in".

## Tools

All tools are annotated read-only. Inputs are validated before any URL is built (symbol format, fixed choices, `limit` ≤ 100); results over 100 KB are refused with a request to narrow the query.

| Tool | momentum-api |
|---|---|
| `get_market_report` | `GET /api/v1/market-report/today` |
| `list_alerts` (`mode` raw/grouped, `symbol`, `alert_type`, `severity`, `since`, `until`, `before`, `limit`) | `GET /api/v1/alerts` |
| `get_candidates`, `get_candidate(symbol)` | `GET /api/v1/scanner/today`, `/scanner/today/{symbol}` |
| `get_symbol_analysis(symbol)` (technicals, fundamentals, heuristic signals incl. the BUY/TRIM action signal, cash flow; polls up to 20 s while computing) | `GET /api/v1/scanner/today/{symbol}/analysis` |
| `get_watchlist` | `GET /api/v1/watchlist` |
| `get_tracked_positions(status)` | `GET /api/v1/scanner/tracked` |
| `get_data_source_status` | `GET /api/v1/data-sources/status` |
| `list_handbook_sections`, `get_handbook_section(id)` | `GET /api/v1/education/handbook` |

The initialize instructions and the tool descriptions carry the three shared caveats verbatim from `shared/content/momentum_caveats.json` (the file the app and analyst-bot read): candidates and tracked positions the evidence and research-score caveats, alerts and analysis the heuristic-signals caveat, every tool the framing "describes stored data; does not predict prices and is not trading advice".

## Security model

- **No writes.** The momentum-api client has no method parameter: it only sends GET, only to `/api/v1/...` paths built from validated input. No database access.
- **Sign-in.** Claude is the only OAuth client: pre-registered client ID and secret (confidential client, `client_secret_basic` or `_post`), exact redirect URIs (`https://claude.ai/api/mcp/auth_callback`, `https://claude.com/api/mcp/auth_callback`), authorization code with PKCE S256 only. `/authorize` sends the browser to GitHub with **no scope** (public profile only); the callback accepts exactly `GITHUB_ALLOWED_USER_ID` (numeric, not the login) and revokes the GitHub token straight after reading the id. A browser-binding cookie stops a callback from finishing in another browser.
- **Tokens.** Access (1 h) and refresh (30 d) tokens are HMAC-SHA256-signed, bound to the resource URL, the client ID and the allowed user id, and not stored. Rotating `MCP_TOKEN_SIGNING_KEY` revokes all of them; changing `GITHUB_ALLOWED_USER_ID` revokes the old user's.
- **IP allowlist.** `/mcp` and `/token` accept only `MCP_IP_ALLOWLIST` (default `160.79.104.0/21`, Anthropic's published outbound range), both at the Cloudflare edge and in the service. `/authorize` and the GitHub callback stay open: your browser visits them from your own IP. The client IP is `CF-Connecting-IP` only when the TCP peer is in `MCP_TRUSTED_PROXIES` (the `mcp-edge` network, where only cloudflared sits); from anywhere else the header is ignored. Every blocked request is logged (`"blocked: outside IP allowlist"`, time, IP, path). The range is shared by every Claude user, so it filters scanners, not people: OAuth is the control. `MCP_IP_ALLOWLIST_ENABLE=false` switches the in-service check off if Claude ever calls from outside the range.
- **Rate limits.** `/mcp`: 60/min per token and 300/min overall; sign-in endpoints 10/min per IP; 5 failed sign-ins or token requests from one IP lock it out of the sign-in endpoints for 15 minutes.
- **No secrets out.** Every momentum-api reply is JSON-parsed and redacted before it leaves: keys that name a secret (token, api_key, password, dsn, …) are dropped, `token=…`/`apikey=…` query values, `Bearer …` values and `user:pass@` URL credentials are masked. Errors return a short code, never an internal URL. On today's live data the redaction changes nothing.
- **Logs** (JSON, stdout): time, client IP, method, path (never the query string), a 12-hex hash of the bearer token, the JSON-RPC method, the tool name and argument *names*, status, bytes, duration. Never response bodies, argument values, tokens, codes or secrets.

## Environment

Set in the repo-root `.env`; `infra/docker-compose.yml` passes only these to the container (never the whole `.env`). Values marked *secret* are generated or pasted straight into `.env`: never print, paste into chat, or commit them.

| Variable | Default | |
|---|---|---|
| `MCP_PUBLIC_URL` | — (required) | `https://mcp.<your-domain>`, no path; the OAuth issuer. The resource is this + `/mcp`. |
| `MCP_OAUTH_CLIENT_ID` | — (required) | Claude's client ID (generated). |
| `MCP_OAUTH_CLIENT_SECRET` | — (required, *secret*) | Claude's client secret, ≥ 32 chars (generated). |
| `MCP_TOKEN_SIGNING_KEY` | — (required, *secret*) | base64 of ≥ 32 random bytes (generated). |
| `GITHUB_OAUTH_CLIENT_ID` | — (required) | From the GitHub OAuth app. |
| `GITHUB_OAUTH_CLIENT_SECRET` | — (required, *secret*) | From the GitHub OAuth app. |
| `GITHUB_ALLOWED_USER_ID` | — (required) | Your numeric GitHub user id. |
| `CLOUDFLARE_TUNNEL_TOKEN` | — (*secret*) | For the cloudflared container. |
| `MCP_IP_ALLOWLIST_ENABLE` | `true` | `false` disables the in-service check (logged as a warning at startup). |
| `MCP_IP_ALLOWLIST` | `160.79.104.0/21` | Comma-separated CIDRs for `/mcp` and `/token`. |
| `MCP_TRUSTED_PROXIES` | Compose: `172.31.250.0/29` | Peers whose `CF-Connecting-IP` is believed. |
| `MCP_ADDR` | `0.0.0.0:8095` | |
| `MCP_MOMENTUM_API_URL` | `http://momentum-api:8090` | |
| `MCP_CAVEATS_PATH` | `/shared/content/momentum_caveats.json` | Missing file fails startup. |
| `MCP_OAUTH_REDIRECT_URIS` | claude.ai and claude.com callbacks | |
| `MCP_ACCESS_TOKEN_TTL` / `MCP_REFRESH_TOKEN_TTL` | `1h` / `720h` | |
| `MCP_RATE_PER_TOKEN_PER_MIN` / `MCP_RATE_GLOBAL_PER_MIN` / `MCP_RATE_AUTH_PER_IP_PER_MIN` | `60` / `300` / `10` | |
| `GITHUB_WEB_URL` / `GITHUB_API_URL` | `https://github.com` / `https://api.github.com` | Tests only. |

## Commands

```sh
make build      # bin/svc-mcp
make test       # unit + end-to-end (local HTTPS, fake GitHub, fake momentum-api)
make run        # needs the environment above
make docker     # trading-agent-svc-mcp image
```

End-to-end against the real momentum-api (read-only): `MCP_E2E_MOMENTUM_API=http://localhost:8090 go test ./internal/app/ -run TestEndToEnd -v`.

From the repo root (refused 23:00–02:30 Greek, the daily-chain window; each starts only the named containers with `--no-deps`, so nothing else is recreated):

```sh
make mcp-up         # svc-mcp + cloudflared
make mcp-up-local   # svc-mcp only, no tunnel (nothing public)
make log-mcp
make mcp-stop
```

## Setup: domain, tunnel, GitHub, edge rules, Claude

Do these in order. Nothing is public until step 1.4 starts cloudflared.

### 1. Cloudflare domain and named tunnel

1. **Domain.** In the Cloudflare dashboard, either *Domain Registration → Register Domains* (buy one at cost), or *Add a domain* for one you own and switch its nameservers at your registrar to the two Cloudflare shows. Wait until the domain shows **Active**.
2. **Tunnel.** *Zero Trust → Networks → Tunnels → Create a tunnel → Cloudflared*. Name it `trading-agent-mcp`. On the install screen pick **Docker**; the command shown ends in `--token <long value>`. Copy only that value into `.env` as `CLOUDFLARE_TUNNEL_TOKEN=` (edit the file directly; do not paste it in chat). Skip running their command.
3. **Public hostname.** Next screen: *Subdomain* `mcp`, *Domain* yours, *Path* empty, *Service* `HTTP` : `svc-mcp:8095`. Save. Add no other hostname: never momentum-api or anything else.
4. Set `MCP_PUBLIC_URL=https://mcp.<your-domain>` in `.env`. (Start it after step 2, when the GitHub values exist: svc-mcp refuses to start without them.)

### 2. GitHub OAuth app

1. Confirm two-factor authentication is on: *GitHub → Settings → Password and authentication*.
2. *Settings → Developer settings → OAuth Apps → New OAuth App* (an OAuth App, not a GitHub App):
   - Application name: `trading-agent MCP`
   - Homepage URL: `https://mcp.<your-domain>`
   - Authorization callback URL: `https://mcp.<your-domain>/oauth/github/callback`
   - Enable Device Flow: **off**
3. *Register application*. Put the **Client ID** in `.env` as `GITHUB_OAUTH_CLIENT_ID=`. *Generate a new client secret* and put it straight into `.env` as `GITHUB_OAUTH_CLIENT_SECRET=`.
4. Your numeric id: `curl -s https://api.github.com/users/<your-login> | grep '"id"'` (the first `"id"`, a number). Put it in `.env` as `GITHUB_ALLOWED_USER_ID=`.
5. Start, in daytime: `make mcp-up`, then `make log-mcp`: expect `svc-mcp listening` and cloudflared `Registered tunnel connection` (×4).
6. Check from outside: `curl -s https://mcp.<your-domain>/.well-known/oauth-protected-resource/mcp` returns JSON whose `resource` is `https://mcp.<your-domain>/mcp`; `curl -s -o /dev/null -w '%{http_code}\n' -X POST https://mcp.<your-domain>/mcp` returns **403** (your IP is outside Anthropic's range: the allowlist works).

### 3. Cloudflare edge rules

In the dashboard, your domain → *Security → WAF*:

1. **Custom rule** `mcp: Anthropic only`, action **Block**, expression (*Edit expression*):
   ```
   (http.host eq "mcp.<your-domain>" and http.request.uri.path in {"/mcp" "/token"} and not ip.src in {160.79.104.0/21})
   ```
2. **Custom rule** `mcp: known paths only`, action **Block**:
   ```
   (http.host eq "mcp.<your-domain>" and not http.request.uri.path in {"/mcp" "/token" "/authorize" "/oauth/github/callback" "/.well-known/oauth-protected-resource" "/.well-known/oauth-protected-resource/mcp" "/.well-known/oauth-authorization-server" "/.well-known/openid-configuration"})
   ```
3. **Rate limiting rule** `mcp: rate`: when `http.host eq "mcp.<your-domain>"`, counting by IP, **50 requests per 10 seconds**, action **Block** for 10 seconds (the free plan's period).
4. Leave **Bot Fight Mode** and **Under Attack mode** off for this domain: they challenge server-to-server requests, and Claude cannot answer a challenge.
5. Security events (*Security → Events*) show what the rules blocked; svc-mcp's own log shows the same for `/mcp` and `/token`.

### 4. Add the connector in Claude

1. Read the two values in your own terminal (do not paste them into any chat): `grep -E '^MCP_OAUTH_CLIENT_(ID|SECRET)=' .env`.
2. claude.ai → *Customize → Connectors* → *Add custom connector*:
   - Name: `trading-agent`
   - Remote MCP server URL: `https://mcp.<your-domain>/mcp`
   - *Advanced settings*: OAuth Client ID = `MCP_OAUTH_CLIENT_ID`, OAuth Client Secret = `MCP_OAUTH_CLIENT_SECRET`.
   These cannot be edited later; to change them, remove the connector and add it again.
3. *Add*, then *Connect*: the browser goes to GitHub (consent screen asks for no permissions), then back to Claude as connected.
4. In a chat, enable the connector and ask "What does today's market report say?". `make log-mcp` should show `request` lines from `160.79.104.x` with `"tool":"get_market_report"`.
5. If Claude reports a connection failure and the log shows `blocked: outside IP allowlist` from another address, Anthropic called from outside the published range: set `MCP_IP_ALLOWLIST_ENABLE=false`, relax WAF rule 1, restart (`make mcp-up`), and record the address.

### Revoking

- Every token: `make mcp-rotate-key` (rewrites `MCP_TOKEN_SIGNING_KEY` in `.env` without printing it), then `make mcp-up`. Claude must reconnect.
- Stop public access at once: `make mcp-stop` (cloudflared first), or remove the connector in Claude.

## Known limits

- momentum-api's write routes (PUT/DELETE watchlist, followed and computed symbols) have no auth and are reachable from any container on the Compose default network. svc-mcp cannot call them, and the tunnel never routes to momentum-api, but anything else on that network could.
- Authorization codes and pending sign-ins live in memory: a restart during a sign-in means starting it again from Claude. Issued tokens survive restarts.
- `get_symbol_analysis` triggers the same on-demand analysis the Stock Detail page does when data is stale.

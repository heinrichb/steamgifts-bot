# TODO / Future features

A living list of things that would make the bot better. Not promises — just the backlog. PRs welcome on any of these. ([CONTRIBUTING.md](CONTRIBUTING.md))

## Completed

- [x] **Smart entry scorer** — scan→rank→enter architecture with sniper boost (+10), wishlist boost (+5), level-locked boost (+3), and cost efficiency
- [x] **Pagination** — `max_pages` config knob to fetch multiple listing pages per filter
- [x] **Steam account sync** — auto-syncs once per 24h, refunds points for owned games
- [x] **Discord webhook notifications** — rich embed on wins via `/giveaways/won` polling
- [x] **Cloudflare challenge detection** — clear error instead of confusing parse failures
- [x] **HTTP retry with exponential backoff** — 502/503/504/429 retry up to 3× (5s/10s/20s)
- [x] **Structured JSON logging** — `--log-format json` for Docker/aggregators
- [x] **Config hot-reload on SIGHUP** — reload config without restarting the process
- [x] **Per-account proxy support** — HTTP/SOCKS5 via `proxy_url` config
- [x] **Level-locked giveaway parsing** — pre-filters entries by contributor level
- [x] **Configurable scorer weights** — all weights tunable via config.yml `scorer:` section
- [x] **Clearer cookie expiry messages** — detects login page, tells user exactly what to do
- [x] **macOS LaunchAgent service install** — completes Linux/Windows/macOS platform trifecta
- [x] **Per-account scorer weight overrides** — different strategies per account via config
- [x] **Telegram bot notifications** — alongside Discord, with shared postJSON helper
- [x] **Expected-value scoring** — replaced cost-efficiency (rewarded shovelware) with win probability per point
- [x] **Multicopy in default filters** — multi-copy giveaways scanned by default

## UX

- [ ] **Auto cookie capture via local proxy** — a simple paste form is no better than the terminal; this needs to proxy the steamgifts login flow and intercept the Set-Cookie header automatically. Significantly more complex (HTTPS, redirect chain, Steam OpenID) but would be the real UX breakthrough.
- [ ] **GUI subsystem Windows build** — eliminate the .bat launcher by building with `-H windowsgui`
- [ ] **Code-signed Windows builds** (Authenticode) so SmartScreen stops warning
- [ ] **Auto-update check** on startup with opt-in download

## Scorer enhancements

- [ ] **Popularity / quality boost** — score AAA/highly-rated games higher via Steam Web API + SQLite cache
- [ ] **Optional per-Steam-app entry cap** _(default: off)_ — limit entries per game for users who want to spread points wider. Default is uncapped (steamgifts auto-refunds on win)

> **Note on duplicate games**: by default the bot enters every unique giveaway code, including multiple for the same game. Steamgifts auto-refunds losing entries on win, so this is economically correct.

## Filter system v2

- [ ] **Parameterized filters** — `copy_min: 5`, `type: any`, etc.
- [ ] **Combined filters** — single request with multiple constraints
- [x] **Raw URL escape hatch** — filters starting with `/` are passed through as-is

## Reliability

- [ ] **Captcha detection + alerting** — pause + notify instead of burning entries
- [ ] **Cookie rotation / re-auth via Steam OpenID**

## Features

- [ ] **SQLite state persistence** — entry history, dedupe across restarts, won-game tracking
- [ ] **Wishlist sync from Steam profile URL** — auto-update which games to prioritize
- [ ] **Backup / restore** of state directory

## Observability

- [x] **Prometheus `/metrics` endpoint** — `--metrics-addr :9090` exposes entries, points, cycles, sync, wins per account

## Operations

- [ ] **Helm chart** for k8s users
- [ ] **Web UI dashboard** — status, points, recent entries, manual trigger

## Internals

- [ ] **Parser fixture refresh tool** — `make refresh-fixtures` fetches current pages into `testdata/`

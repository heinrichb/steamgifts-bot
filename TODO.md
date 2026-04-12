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

## UX

- [ ] **Browser-helper cookie capture** — tiny `localhost:PORT` page that replaces the DevTools step
- [ ] **GUI subsystem Windows build** — eliminate the .bat launcher by building with `-H windowsgui`
- [ ] **Code-signed Windows builds** (Authenticode) so SmartScreen stops warning
- [ ] **Auto-update check** on startup with opt-in download
- [ ] **Clearer error messages** when cookies expire mid-run

## Scorer enhancements

- [ ] **Popularity / quality boost** — score AAA/highly-rated games higher via Steam Web API + SQLite cache
- [ ] **Per-account weight overrides** — different scoring strategies per account
- [ ] **Optional per-Steam-app entry cap** _(default: off)_ — limit entries per game for users who want to spread points wider. Default is uncapped (steamgifts auto-refunds on win)
- [ ] **Configurable scorer weights** — expose the weight constants in config.yml

> **Note on duplicate games**: by default the bot enters every unique giveaway code, including multiple for the same game. Steamgifts auto-refunds losing entries on win, so this is economically correct.

## Filter system v2

- [ ] **Parameterized filters** — `copy_min: 5`, `type: any`, etc.
- [ ] **Combined filters** — single request with multiple constraints
- [ ] **Raw URL escape hatch** — `raw: "/giveaways/search?type=group&copy_min=3"`

## Reliability

- [ ] **Captcha detection + alerting** — pause + notify instead of burning entries
- [ ] **Cookie rotation / re-auth via Steam OpenID**
- [ ] **Telegram / generic webhook notifications** — extend the existing notify package

## Features

- [ ] **SQLite state persistence** — entry history, dedupe across restarts, won-game tracking
- [ ] **Wishlist sync from Steam profile URL** — auto-update which games to prioritize
- [ ] **Backup / restore** of state directory

## Observability

- [ ] **Prometheus `/metrics` endpoint** — entries attempted, points, request latency, errors

## Operations

- [ ] **Helm chart** for k8s users
- [ ] **Web UI dashboard** — status, points, recent entries, manual trigger
- [ ] **macOS LaunchAgent service install**

## Internals

- [ ] **Parser fixture refresh tool** — `make refresh-fixtures` fetches current pages into `testdata/`
- [ ] **macOS service install** — `~/Library/LaunchAgents/` plist

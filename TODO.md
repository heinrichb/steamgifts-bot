# TODO / Future features

A living list of things that would make the bot better. Not promises — just the backlog. PRs welcome on any of these. ([CONTRIBUTING.md](CONTRIBUTING.md))

## Completed

- [x] **Smart entry scorer** — scan→rank→enter architecture with sniper boost (+10), wishlist boost (+5), level-locked boost (+3), and cost efficiency
- [x] **Pagination** — scans all listing pages per filter until empty (safety cap: 100)
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
- [x] **Windows double-click support** — console subsystem build with cobra mousetrap disabled
- [ ] **Code-signed Windows builds** (Authenticode) so SmartScreen stops warning
- [x] **Auto-update check** on startup via GitHub Releases API (log-only, never downloads)

## Scorer enhancements

- [ ] **Popularity / quality boost** — score AAA/highly-rated games higher via Steam Web API + SQLite cache
- [x] **Per-app entry cap** _(default: off)_ — `max_entries_per_app` limits entries per game name

> **Note on duplicate games**: by default the bot enters every unique giveaway code, including multiple for the same game. Steamgifts auto-refunds losing entries on win, so this is economically correct.

## Filter system v2

- [x] **Parameterized + combined filters** — covered by raw URL escape hatch
- [x] **Raw URL escape hatch** — filters starting with `/` are passed through as-is

## Reliability

- [x] **Captcha detection** — detects reCAPTCHA/hCaptcha, tells user to solve in browser
- [ ] **Cookie rotation / re-auth via Steam OpenID**

## Features

- [ ] **SQLite state persistence** — entry history, dedupe across restarts, won-game tracking
- [ ] **Wishlist sync from Steam profile URL** — auto-update which games to prioritize
- [x] **Backup / restore** — `steamgifts-bot backup create/restore` zips config+state+logs

## Auto-redeem won keys

Delicate feature — a mistake could get accounts banned from steamgifts (not marking received, accidental duplicate comments) or Steam (too many bad-key attempts). Every subtask below is gated by config toggles, and the defaults leave everything off.

### Config (new fields, all opt-in, default off)

- [ ] **`auto_redeem.enabled: false`** — master switch for the whole feature
- [ ] **`auto_redeem.require_appid_match: true`** — after redeem, validate the key activated the correct Steam appid; if not, abort and alert
- [ ] **`auto_redeem.mark_received: true`** — POST the "mark received" action on the won-giveaway page (site etiquette; not marking can cause a ban)
- [ ] **`auto_redeem.comment.enabled: false`** — post a thank-you comment on the giveaway page (separate toggle from redeem)
- [ ] **`auto_redeem.comment.template: "Thanks!"`** — customizable message
- [ ] **`auto_redeem.max_per_cycle: 1`** — hard cap per scan cycle per account
- [ ] **`auto_redeem.dry_run: true`** — default to dry-run until user explicitly flips it; logs the full plan but performs no writes
- [ ] Per-account overrides, same pattern as existing account settings
- [ ] **`accounts[].steam_cookie`** — new field holding the Steam web session (`sessionid` + `steamLoginSecure`) required to call Steam's `ajaxRegisterKey` endpoint. Documented security note: as sensitive as the steamgifts cookie, redacted in logs

### Core subtasks

- [ ] **Persist won-game state to `state.json`** — new `redeemed` map keyed by giveaway code with `{attempted_at, status, appid, steam_result, commented_at}`. Critical invariant: `commented_at != nil` means we have already commented — **never comment twice**, regardless of any other state. This is the single source of truth for the 1-comment guardrail. Must survive restart (today's `seenWins` is in-memory only)
- [ ] **Key extraction from giveaway page** — fetch `/giveaway/<code>/<slug>` for each won game, parse the key-reveal flow (may require a POST to reveal before the key appears in the DOM). Handle the case where the key is blank / already revealed / tied to a different store
- [ ] **Key redemption against Steam** — POST to `store.steampowered.com/account/ajaxregisterkey` using `steam_cookie`; parse the JSON response. Map Steam's error codes (14 = already owned, 15 = invalid, 53 = rate-limited, 36 = region-locked, etc.) into typed Go errors
- [ ] **Rate-limit Steam redemption aggressively** — Steam locks the account for 1h after 5 bad-key attempts and 24h after 10 in a day. Global backoff (not just per-account) when a rate-limit response comes back. Stop all redemption attempts on the first rate-limit hit in a cycle
- [ ] **App-ID validation** — after a successful redeem, confirm the activated appid matches the appid advertised by the steamgifts giveaway (parse the Steam store link on the giveaway page). Any mismatch halts the flow, persists the anomaly, and alerts via the existing notifier
- [ ] **Mark-received POST** — POST to the steamgifts "mark received" endpoint (likely `/ajax.php?do=feedback_set_received` or similar — confirm via network tab before coding) with the xsrf token
- [ ] **Comment POST** — POST to the steamgifts comment endpoint (likely `/ajax.php?do=comment_insert` — confirm) with xsrf token, giveaway code, and the user's template. Gate on `commented_at == nil` check against persisted state **as the last action before the POST**, and set `commented_at` **before** the POST so a crash mid-POST never results in a second comment
- [ ] **Dry-run + staged rollout** — feature ships with `dry_run: true` by default. Dry-run logs the entire planned flow (extracted key redacted, redeem call skipped, mark-received skipped, comment skipped) so the user can audit a real won giveaway before enabling writes. Bot 1's current unredeemed win is the first real test subject

### Safety guardrails (non-negotiable)

- [ ] **Never more than 1 comment per giveaway** — enforced by persisted `commented_at`, checked under a per-account mutex
- [ ] **Abort-on-uncertainty** — any parser miss, HTTP anomaly, unexpected Steam response, or appid mismatch halts the entire redeem flow for that giveaway, records the failure with full context, and sends a notification. No retry without explicit human intervention
- [ ] **Key redaction in logs** — add `steam_key`, `cd_key`, `product_key`, `activation_key` to [internal/log/log.go](internal/log/log.go) `sensitiveKeys`
- [ ] **Kill-switch via SIGHUP** — flipping `auto_redeem.enabled: false` and sending SIGHUP stops any further redemption immediately (existing reload path)
- [ ] **No concurrent redeem across accounts** — serialize redemption with a global mutex; Steam treats rapid cross-account redeems as suspicious
- [ ] **Backoff on consecutive failures** — 3 consecutive failures of any kind pauses redemption for 24h and alerts the user

### Tests

- [ ] `httptest.Server` fixtures for: Steam success, Steam wrong-appid, Steam rate-limit, Steam invalid-key, steamgifts mark-received success, steamgifts comment success, steamgifts comment-already-posted
- [ ] Table-driven test enforcing the "never comment twice" invariant against concurrent calls (sync/atomic or explicit mutex test)
- [ ] Dry-run test: verifies no POSTs are issued and state is not mutated

## Observability

- [x] **Prometheus `/metrics` endpoint** — `--metrics-addr :9090` exposes entries, points, cycles, sync, wins per account

## Operations

- [x] **Helm chart** — `deploy/helm/steamgifts-bot/` with config Secret, state PVC, metrics+dashboard
- [x] **Web UI dashboard** — `--dashboard-addr :8080` with auto-refresh status page

## Internals

- [x] **Parser fixture refresh tool** — `make refresh-fixtures` fetches and redacts real pages

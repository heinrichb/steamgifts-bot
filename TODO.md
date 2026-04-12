# TODO / Future features

A living list of things that would make the bot better. Not promises — just the backlog. PRs welcome on any of these. ([CONTRIBUTING.md](CONTRIBUTING.md))

## UX

- [ ] **Browser-helper cookie capture** — replace the DevTools step with a tiny `localhost:PORT` page that runs in the wizard. The user clicks a link, lands on a "paste cookie" form, done. Removes the most painful step entirely.
- [ ] **Animated GIF / video** of the wizard flow, linked from the README so users can preview the experience before downloading.
- [ ] **Code-signed Windows builds** (Authenticode) so SmartScreen stops warning users.
- [ ] **macOS notarized `.app` bundle** with a real GUI launcher (double-click to run).
- [ ] **Auto-update check** on startup with opt-in download (similar to gh, atuin, etc.).
- [ ] **Clearer error messages** when cookies expire mid-run — pop a desktop notification rather than just logging.

## Reliability

- [ ] **Captcha detection + alerting** — if steamgifts starts asking for one, pause the account and notify rather than burning the cookie.
- [ ] **Adaptive rate limiting** based on response headers and 429s.
- [ ] **Cookie rotation / re-auth via Steam OpenID** so users don't have to manually refresh `PHPSESSID` periodically.
- [ ] **Soft-fail on transient HTTP errors** (network blips, 502s) with backoff instead of logging an error every cycle.

## Filter system v2

The current `filters:` list accepts only fixed names (`wishlist`, `dlc`, `multicopy`, etc.) that map 1:1 to a hard-coded URL. Power users will want more.

- [ ] **Parameterized filters** — let `copy_min` take any N (`copy_min: 5`), let `type` take any value, etc.
- [ ] **Combined filters** — single request with multiple constraints (e.g. wishlist AND multicopy: `/giveaways/search?type=wishlist&copy_min=2`).
- [ ] **Raw URL escape hatch** — `raw: "/giveaways/search?type=group&copy_min=3&dlc=true"` for users who want full control.
- [ ] **Pagination** — listing pages have `&page=N`. Right now we only fetch page 1; for filters with lots of results that's fine because we sort by new and old ones are usually already entered, but the scoring engine will want a wider net.

The likely shape is moving `filters` from `[]string` to `[]Filter` where `Filter` is a struct with optional `type`, `copy_min`, `dlc`, `pages`, and `raw` fields. Backwards compatible via a custom unmarshal that accepts a bare string.

## Smart entry / scoring engine

Today the bot enters every joinable giveaway in DOM order, filter by filter. That's leaving wins on the table — a smarter strategy is to **score** each candidate and enter the highest-value ones first, until points run out.

Sketch of how this would fit:

- New `internal/scorer/` package. Pure function: `Score(g Giveaway, ctx ScoreContext) float64`.
- `ScoreContext` carries the things scoring needs that aren't on the giveaway itself: the user's wishlist, current points, current account level, optional Steam app metadata cache.
- Runner change: instead of `for _, g := range giveaways`, collect all joinable giveaways from every filter, sort by score descending, then enter in that order until points hit min.
- Scoring is **additive with weighted components**, all of which can be enabled/disabled and reweighted from `config.yaml`. That way users can build their own strategy without code changes.

Components to implement, in priority order:

- [ ] **Wishlist boost** — large positive weight if `g.Name` is in the user's Steam wishlist. Wishlist filter already prefers these, but a boost lets a wishlist game on the `all` filter still beat random titles. Requires the wishlist-sync feature below.
- [ ] **Sniper boost** — closing-soon + low-entry-count = high win probability. Score function: `(timeLeft < threshold) * (cost / entries)`. The closer to the deadline and the fewer entries, the higher the boost. Probably the single biggest EV win.
- [ ] **Level-locked boost** — opt-in flag. When the user is at or above a giveaway's required level, prioritize higher-level-locked entries since they have a smaller eligible audience. Needs the parser to extract `data-level-min` (or whatever the current attribute is) and the runner to know the account's level (already on the front page).
- [ ] **Popularity / quality boost** — score AAA / highly-rated games higher. Needs an external metadata source. Options:
  - Steam Web API (free, no key for public app data) for review score and player count
  - SteamSpy (free, rate-limited) for owner counts
  - Local cache (SQLite) keyed by Steam appid so we don't refetch every cycle
- [ ] **Cost efficiency** — small tiebreaker preferring cheaper games when other scores are equal, so the bot doesn't blow all its points on one expensive entry.
- [ ] **Per-account weight overrides** — one user might run a wishlist-only sniper alt and a wide-net main on the same machine.

Prerequisite features (each useful on their own):

- [ ] **Wishlist sync from a Steam profile URL** — fetch the user's wishlist on a slow cadence, cache to disk, expose to the scorer.
- [ ] **SQLite metadata cache** — already on the future-features list; the scorer is the main consumer.

## Features

- [ ] **Proxy support per account** (HTTP / SOCKS5) — useful when running many accounts.
- [ ] **OpenVPN integration** (the original project's stretch goal) — route per-account traffic through different VPNs.
- [ ] **SQLite state persistence** — entry history, dedupe across restarts, won-game tracking.
- [ ] **Discord / Telegram / generic webhook notifications** when you win something.
- [ ] **Wishlist sync from a Steam profile URL** — auto-update which games to prioritize.
- [ ] **Per-giveaway scoring** (ROI: points × win-probability based on entries/copies).
- [ ] **`config.yaml` hot-reload** on `SIGHUP`.
- [ ] **Backup / restore** of state directory.

## Observability

- [ ] **Prometheus `/metrics` endpoint** — entries attempted, points, request latency, errors.
- [ ] **Structured JSON logging mode** for log aggregators (slog already supports this; just needs a flag).

## Operations

- [ ] **Helm chart** for k8s users.
- [ ] **Web UI dashboard** — status, points, recent entries, manual trigger button. Probably a separate `internal/web/` package and an opt-in flag.
- [ ] **Multi-arch base image experiments** — ARM v6/v7 for Raspberry Pi Zero users.

## Internals

- [ ] **Parser fixture refresh tool** — `make refresh-fixtures` that fetches current pages into `testdata/` (with cookies redacted) so we can keep tests honest as steamgifts evolves.
- [ ] **Integration test harness** that spins up an httptest server with realistic HTML and exercises the full runner loop.
- [ ] **macOS service install** — `~/Library/LaunchAgents/` plist.

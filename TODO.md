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

# steamgifts-bot

A small, fast, multi-account giveaway bot for [steamgifts.com](https://www.steamgifts.com), written in Go.

- **Single static binary** — no runtime, no interpreter, no headless browser
- **Multi-account from day one** — each account runs independently
- **Friendly first-run wizard** — paste your cookie, the bot does the rest
- **Three install paths**: Docker, prebuilt binary, build from source
- **Optional background service** — Windows Scheduled Task or systemd user unit
- **YAML config** with full CLI flag and environment-variable overrides
- **Tiny Docker image** (~10 MB, distroless, runs as nonroot)

---

## Choose your install

### 🪟 Non-technical / Windows users

1. Go to the [Releases page](https://github.com/heinrichb/steamgifts-bot/releases) and download `steamgifts-bot_<version>_windows_x86_64.zip`.
2. Extract the zip somewhere convenient (e.g. `C:\steamgifts-bot`).
3. Double-click `steamgifts-bot.exe`.
4. Follow the wizard — it walks you through capturing your cookie with screenshots and validates it before saving anything.
5. When the wizard offers to install a background service, say yes — the bot will start automatically every time you log in.

> **SmartScreen warning?** The binary isn't yet code-signed (see [TODO.md](TODO.md)). Click **More info** → **Run anyway**.

### 🐧 Linux / 🍎 macOS desktop

```bash
# Install
curl -L https://github.com/heinrichb/steamgifts-bot/releases/latest/download/steamgifts-bot_linux_x86_64.tar.gz | tar -xz
sudo mv steamgifts-bot /usr/local/bin/

# First run
steamgifts-bot setup

# Optional: install as a user systemd service
steamgifts-bot service install
```

### 🐳 Docker (24/7 server / homelab)

```bash
# Grab the example config and edit it
curl -O https://raw.githubusercontent.com/heinrichb/steamgifts-bot/main/config.example.yml
cp config.example.yml config.yml
# …add your cookie…

# Grab compose file and start
curl -O https://raw.githubusercontent.com/heinrichb/steamgifts-bot/main/docker-compose.yml
docker compose up -d

# Tail logs
docker compose logs -f bot
```

Or build from source:

```bash
git clone https://github.com/heinrichb/steamgifts-bot.git
cd steamgifts-bot
docker compose up -d --build
```

### ⏰ Cron / systemd (DIY)

If you'd rather schedule the binary yourself:

```cron
# Run every 15 minutes
*/15 * * * * /usr/local/bin/steamgifts-bot run --once >> /var/log/steamgifts-bot.log 2>&1
```

Or write a one-shot systemd timer pointed at `steamgifts-bot run --once`. The `service install` command does the equivalent of this for you, but as a long-running service.

---

## Getting your cookie

The bot uses your `PHPSESSID` cookie to authenticate. The first-run wizard handles all of this for you, but if you want to do it by hand:

1. Open [steamgifts.com](https://www.steamgifts.com) and sign in with Steam
2. Press <kbd>F12</kbd> to open DevTools
3. **Chrome/Edge**: Application tab → Cookies → `https://www.steamgifts.com`
   **Firefox**: Storage tab → Cookies → `https://www.steamgifts.com`
4. Click the `PHPSESSID` row
5. Copy the **Value** column

Paste it into `config.yml` (`accounts[0].cookie`) or the wizard.

> **Treat this cookie like a password.** It grants full access to your steamgifts account. Never commit `config.yml`, never paste it in chat. The bot stores it with `0600` permissions.

---

## Configuration reference

Every key in `config.example.yml` can also be set via environment variable. Precedence (highest first):

1. Environment variable — `STEAMGIFTS_DEFAULTS_MIN_POINTS=100`
2. `config.yml`
3. Built-in defaults

| Key                            | Default                                    | Description                                         |
| ------------------------------ | ------------------------------------------ | --------------------------------------------------- |
| `defaults.min_points`          | `50`                                       | Don't enter if it would drop you below this.        |
| `defaults.pause_minutes`       | `15`                                       | Sleep between scan cycles.                          |
| `defaults.enter_pinned`        | `false`                                    | Include pinned/featured giveaways.                  |
| `defaults.max_entries_per_run` | `25`                                       | Safety cap.                                         |
| `defaults.user_agent`          | Current Chrome UA                          | HTTP User-Agent (mimics a real browser by default). |
| `filters`                      | `[wishlist, group, recommended, new, all]` | Order to scan. Stops when points hit `min_points`.  |
| `accounts[].name`              | —                                          | Friendly label.                                     |
| `accounts[].cookie`            | —                                          | PHPSESSID cookie value.                             |
| `accounts[].*`                 | —                                          | Any `defaults.*` key can be overridden per-account. |

### CLI commands

```
steamgifts-bot                  # smart default: run if config exists, else launch wizard
steamgifts-bot setup            # interactive first-run wizard (re-runnable)
steamgifts-bot run              # run the bot (use --once for a single pass)
steamgifts-bot run --tui        # live status dashboard
steamgifts-bot run --dry-run    # log candidate entries without submitting
steamgifts-bot check            # validate config + cookies, show points, exit
steamgifts-bot accounts add     # add an account with the cookie wizard
steamgifts-bot accounts list    # list configured accounts (cookies redacted)
steamgifts-bot accounts remove <name>
steamgifts-bot service install  # register Scheduled Task / systemd user unit
steamgifts-bot service uninstall
steamgifts-bot service status
steamgifts-bot version
```

---

## Troubleshooting

**`cookie didn't work`** — your `PHPSESSID` has expired. Sign back in to steamgifts.com and grab a fresh cookie. `steamgifts-bot setup` will walk you through it again.

**`http 403` errors** — usually means the cookie is bad or expired. Same fix.

**`go vet` / `golangci-lint` warnings on contribution** — run `make fmt` and `make lint` before pushing.

**The bot isn't entering anything** — try `steamgifts-bot run --once --dry-run --log-level debug` to see what it's looking at without committing.

---

## Development

```bash
make test       # go test -race
make lint       # golangci-lint
make fmt        # gofmt + goimports
make build      # build the binary
make run        # build + dry-run --once at debug level
make docker     # build the Docker image locally
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full developer guide.

---

## License

[MIT](LICENSE) © 2026 Brennen Heinrich

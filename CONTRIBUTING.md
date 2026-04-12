# Contributing

Thanks for your interest! This project is small and friendly. PRs that fix bugs, improve UX, or expand test coverage are always welcome.

## Quick start

```bash
git clone https://github.com/heinrichb/steamgifts-bot.git
cd steamgifts-bot
make test
```

## Requirements

- Go 1.26+ (the version in `go.mod` is the floor)
- `golangci-lint` (install with `brew install golangci-lint` or [the official script](https://golangci-lint.run/welcome/install/))
- `make` for the convenience targets (optional — every target is one `go` command underneath)
- Docker (optional, only needed if you're touching the Dockerfile)

## Make targets

| Target                  | What it does                                                    |
| ----------------------- | --------------------------------------------------------------- |
| `make build`            | Build the binary into `./steamgifts-bot`                        |
| `make test`             | `go test -race -count=1 ./...`                                  |
| `make lint`             | `golangci-lint run`                                             |
| `make fmt`              | `gofmt -s -w .`, `goimports -w .`, and Prettier for YAML/MD     |
| `make vet`              | `go vet ./...`                                                  |
| `make tidy`             | `go mod tidy`                                                   |
| `make run`              | Build and run a `--once --dry-run --log-level debug` cycle      |
| `make check`            | Build and run `steamgifts-bot check`                            |
| `make docker`           | Build the Docker image as `steamgifts-bot:dev`                  |
| `make refresh-fixtures` | Fetch real steamgifts pages into `testdata/` (needs config.yml) |

## Project layout

```
cmd/steamgifts-bot/    # CLI entrypoint (main.go + Windows console helpers)
cmd/refresh-fixtures/  # dev tool: fetch real HTML fixtures
internal/
  account/             # per-account runner + orchestrator
  cli/                 # cobra command tree + menu + backup
  client/              # HTTP client with retry + proxy
  config/              # YAML schema + validation + scorer weights
  log/                 # slog wrapper with dual output + redaction
  metrics/             # Prometheus /metrics endpoint
  notify/              # Discord + Telegram win notifications
  ratelimit/           # jittered sleeper
  scorer/              # smart entry scoring (sniper, wishlist, level, EV)
  service/             # systemd / Startup folder / LaunchAgent install
  state/               # persistent JSON state (last sync times)
  steamgifts/          # HTML parser + entry submission + sync + wins
  update/              # GitHub Releases version check
  web/                 # embedded dashboard (HTML templates)
  wizard/              # first-run TUI flow
deploy/helm/           # Kubernetes Helm chart
```

The `steamgifts/` package is where 90% of bug reports will land. It owns the HTML parser and the entry POST.

## Testing conventions

- **Unit tests live alongside the code** as `*_test.go`.
- **Parser tests** load HTML from `internal/steamgifts/testdata/*.html`. When steamgifts.com changes its markup, fix it here first by saving a current page and adding a focused test for whatever broke.
- **HTTP code** uses `httptest.NewServer` rather than mocking. The `client.WithBaseURL` option exists specifically for this.
- **Don't mock cookies** in entry tests — we want the real net/http cookie jar to round-trip them, that's a class of bug we've burned on before.

## Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(parser): handle pinned-row class change
fix(wizard): retry on transient cookie validation errors
docs: clarify Docker compose first-run flow
test(client): cover non-2xx body propagation
```

GoReleaser uses these prefixes to group the changelog automatically.

## Pull request checklist

- [ ] `make test` is green
- [ ] `make lint` is clean
- [ ] New behavior has a test (or there's a comment explaining why one isn't possible)
- [ ] If you touched user-facing flags, config keys, or wizard copy, the README is updated
- [ ] If it's a behavior change, add a line to `CHANGELOG.md` under `[Unreleased]`

## Adding new parser fixtures

When the steamgifts HTML changes:

1. Open the affected page logged in
2. Right-click → Save As → "HTML, complete" is fine
3. Trim the saved file down to just the relevant `<div class="giveaway__row-...">` blocks (keep the page small — fixtures are read every test run)
4. Drop it in `internal/steamgifts/testdata/`
5. Add a test that asserts the new shape

## Releasing

Maintainers only:

```bash
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

GoReleaser builds binaries for linux/macOS/windows × amd64/arm64 and publishes the GitHub Release. The Docker workflow simultaneously builds and pushes the multi-arch image to GHCR.

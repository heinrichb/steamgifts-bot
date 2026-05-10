# Contributing

Thanks for your interest in steamgifts-bot!

## Bug reports

Found something broken? Open an issue with:

- What happened and what you expected
- Steps to reproduce the problem
- Your OS and Go version
- Relevant lines from `steamgifts-bot.log` if available

The more detail you include, the faster it gets fixed.

## Feature requests

Have an idea for something new? Open an issue describing:

- The problem you're trying to solve
- Why the current behavior doesn't cover it
- A concrete example of how the feature would work

Please search existing issues before opening a new one.

## Development setup

```bash
git clone https://github.com/heinrichb/steamgifts-bot.git
cd steamgifts-bot
make test
```

### Requirements

- Go 1.26+ (the version in `go.mod` is the floor)
- `golangci-lint` (install with `brew install golangci-lint` or [the official script](https://golangci-lint.run/welcome/install/))
- `make` for the convenience targets (optional — every target is one `go` command underneath)
- Docker (optional, only needed if you're touching the Dockerfile)

### Make targets

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

### Project layout

```
cmd/steamgifts-bot/    # CLI entrypoint (main.go + Windows console helpers)
cmd/refresh-fixtures/  # dev tool: fetch real HTML fixtures
internal/
  account/             # per-account runner + orchestrator
  cli/                 # cobra command tree + TUI app + menu + backup
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
  update/              # GitHub Releases version check + self-update
  web/                 # embedded dashboard (HTML templates)
  wizard/              # first-run TUI flow
deploy/helm/           # Kubernetes Helm chart
```

### Testing conventions

- **Unit tests live alongside the code** as `*_test.go`.
- **Parser tests** load HTML from `internal/steamgifts/testdata/*.html`. When steamgifts.com changes its markup, fix it here first by saving a current page and adding a focused test for whatever broke.
- **HTTP code** uses `httptest.NewServer` rather than mocking. The `client.WithBaseURL` option exists specifically for this.
- **Don't mock cookies** in entry tests — we want the real net/http cookie jar to round-trip them.

### Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(parser): handle pinned-row class change
fix(wizard): retry on transient cookie validation errors
docs: clarify Docker compose first-run flow
test(client): cover non-2xx body propagation
```

GoReleaser uses these prefixes to group the changelog automatically.

## Releasing

Maintainers only:

```bash
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

GoReleaser builds binaries for linux/macOS/windows × amd64/arm64 and publishes the GitHub Release. The Docker workflow simultaneously builds and pushes the multi-arch image to GHCR.

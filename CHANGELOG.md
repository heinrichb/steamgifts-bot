# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial Go rewrite of the project, replacing the never-built TypeScript scaffolding.
- Multi-account support from day one — each account runs independently in its own goroutine with its own cookie jar and rate limiter.
- Interactive first-run wizard (`steamgifts-bot setup`) that opens the browser, walks the user through DevTools cookie capture, and validates the cookie live before saving.
- `run --tui` live status dashboard built on bubbletea showing per-account points, entries, and next-run countdown.
- `check` subcommand that validates config + every cookie and prints a summary table.
- `accounts add/list/remove` subcommands for managing accounts without hand-editing YAML.
- `service install/uninstall/status` for per-user systemd units (Linux) and Scheduled Tasks (Windows).
- `--dry-run` mode that scans and logs candidate entries without submitting.
- YAML config with full CLI flag and `STEAMGIFTS_*` environment-variable override layering.
- Multi-stage Dockerfile producing a ~10 MB distroless image; `docker-compose.yml` for end users.
- GitHub Actions: lint+test+build matrix CI, multi-arch GHCR Docker publish, GoReleaser release on tag.
- Parser fixture tests covering wishlist, pinned, already-entered, expired, and missing-cookie cases.

### Notes

This is a pre-1.0 baseline. Expect the config schema and CLI surface to remain stable, but parser internals may change as steamgifts.com evolves.

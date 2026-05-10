# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Version displayed in interactive menu title, TUI dashboard header, and headless startup banner.
- Account count and background service status summary shown in menu header.
- "View service logs" menu option when the background service is running — replaces manual run options to prevent concurrent sessions.
- Styled startup banner with version for headless `run` mode.
- `service.IsActive()` on all platforms for live service status detection.
- Shared lipgloss style palette (`styles.go`) for consistent CLI styling.
- `config.MaxPoints` constant shared between validation and runner logic.

### Fixed

- Malformed proxy URLs are now reported as errors instead of being silently ignored.
- `backup restore` now sets `0600` permissions on `config.yml` to match `saveConfig` security posture.
- `accounts remove` with a missing config now shows a helpful error instead of a raw viper error.

### Changed

- Menu options are context-aware: when the background service is running, manual run options are replaced with log viewing to prevent conflicts.
- Lipgloss style definitions consolidated from 4 files into a shared palette, eliminating 15 duplicate style declarations.

## [1.2.0] - 2025-05-09

### Added

- Auto-enter 0-point giveaways — these cost nothing, so they are always entered regardless of the `min_points` setting.

## [1.1.0] - 2025-04-12

### Added

- Auto-update: background check for new releases with optional automatic update on menu launch.
- Wishlist filter optimizations for faster scanning.

### Fixed

- Log rotation via lumberjack to prevent unbounded log file growth.
- Expanded permanent rejection detection to cover "Exists in Account" and "Previously Won" errors, preventing unnecessary re-entry attempts.

## [1.0.0] - 2025-04-06

### Added

- Initial Go implementation replacing the never-built TypeScript scaffolding.
- Multi-account support — each account runs independently in its own goroutine with its own cookie jar and rate limiter.
- Interactive first-run wizard (`steamgifts-bot setup`) with browser-assisted cookie capture and live validation.
- `run --tui` live status dashboard built on bubbletea.
- `run --dry-run` mode that scans and logs candidates without submitting entries.
- `check` subcommand for validating config and cookies.
- `accounts add/list/remove` subcommands for managing accounts without hand-editing YAML.
- `service install/uninstall/status` for per-user systemd units (Linux), Startup folder (Windows), and LaunchAgent (macOS).
- `backup create/restore` for config, state, and log archiving.
- YAML config with CLI flag and `STEAMGIFTS_*` environment variable override layering.
- Scorer-based giveaway ranking with configurable weights (wishlist, sniper, level, cost efficiency).
- Persistent state file for Steam sync cooldown tracking.
- Jittered rate limiting with exponential backoff and retry on transient HTTP errors.
- Discord webhook and Telegram bot win notifications.
- Prometheus `/metrics` endpoint and web dashboard.
- Multi-stage Dockerfile producing a ~10 MB distroless image with `docker-compose.yml`.
- GitHub Actions CI/CD: lint, test, build matrix, multi-arch Docker publish, GoReleaser releases.
- Real-browser User-Agent default to avoid detection.
- SIGHUP config reload for headless mode.

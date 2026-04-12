// Package log wraps log/slog with a couple of project conveniences:
// level parsing, automatic TTY-aware colorization (via lmittmann/tint),
// and a per-account child logger helper.
//
// Output mode is auto-detected:
//
//   - Interactive terminals (a real TTY, NO_COLOR not set, TERM != "dumb")
//     get the tint handler — colorized levels, dim timestamps, highlighted
//     keys. Pleasant for `setup`, `check`, and a developer running `run`.
//   - Everything else (Docker, systemd, redirected stdout, NO_COLOR=1)
//     gets slog's plain TextHandler so log aggregators stay happy.
package log

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
)

// New constructs a slog.Logger writing to w at the given level.
// If w is nil, os.Stderr is used. Pass an empty levelStr for "info".
func New(w io.Writer, levelStr string) (*slog.Logger, error) {
	if w == nil {
		w = os.Stderr
	}
	level, err := ParseLevel(levelStr)
	if err != nil {
		return nil, err
	}

	if useColor(w) {
		h := tint.NewHandler(w, &tint.Options{
			Level:      level,
			TimeFormat: time.Kitchen,
			NoColor:    false,
		})
		return slog.New(h), nil
	}
	h := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	return slog.New(h), nil
}

// useColor reports whether w is an interactive terminal that should
// receive colorized output. Honors the NO_COLOR convention
// (https://no-color.org) and TERM=dumb.
func useColor(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if t := os.Getenv("TERM"); t == "dumb" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// ParseLevel converts "debug" / "info" / "warn" / "error" (case-insensitive)
// into a slog.Level. Empty string returns Info.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error", "err":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level %q (valid: debug, info, warn, error)", s)
	}
}

// Account returns a child logger tagged with the given account name.
func Account(parent *slog.Logger, name string) *slog.Logger {
	return parent.With("account", name)
}

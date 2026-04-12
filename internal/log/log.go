// Package log wraps log/slog with a couple of project conveniences:
// level parsing, a colorless text handler suitable for Docker logs, and
// a per-account child logger helper.
package log

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// New constructs a slog.Logger writing structured text to w at the given
// level. Pass an empty levelStr to default to "info".
func New(w io.Writer, levelStr string) (*slog.Logger, error) {
	if w == nil {
		w = os.Stderr
	}
	level, err := ParseLevel(levelStr)
	if err != nil {
		return nil, err
	}
	h := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	return slog.New(h), nil
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

// Package log wraps log/slog with project conveniences: level parsing,
// format selection (text/json/auto), TTY-aware colorization, and a
// per-account child logger helper.
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

// New constructs a slog.Logger writing to w at the given level and format.
// Format: "auto" (default) = color on TTY, plain text otherwise.
// "json" = structured JSON for log aggregators. "text" = plain text always.
func New(w io.Writer, levelStr, format string) (*slog.Logger, error) {
	if w == nil {
		w = os.Stderr
	}
	level, err := ParseLevel(levelStr)
	if err != nil {
		return nil, err
	}
	opts := &slog.HandlerOptions{Level: level}

	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return slog.New(slog.NewJSONHandler(w, opts)), nil
	case "text":
		return slog.New(slog.NewTextHandler(w, opts)), nil
	case "", "auto":
		if useColor(w) {
			h := tint.NewHandler(w, &tint.Options{
				Level:      level,
				TimeFormat: time.Kitchen,
			})
			return slog.New(h), nil
		}
		return slog.New(slog.NewTextHandler(w, opts)), nil
	default:
		return nil, fmt.Errorf("unknown log format %q (valid: auto, text, json)", format)
	}
}

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

// Package log wraps log/slog with project conveniences: level parsing,
// format selection, TTY-aware colorization, dual stderr+file output,
// and automatic sensitive-data redaction in log files.
package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
)

// New constructs a slog.Logger writing to w at the given level and format.
func New(w io.Writer, levelStr, format string) (*slog.Logger, error) {
	if w == nil {
		w = os.Stderr
	}
	level, err := ParseLevel(levelStr)
	if err != nil {
		return nil, err
	}
	h, err := newHandler(w, level, format)
	if err != nil {
		return nil, err
	}
	return slog.New(h), nil
}

// NewWithFile creates a dual-output logger: the console handler writes to
// stderr (colorized or plain), and a JSON file handler writes debug-level
// structured logs to logPath with sensitive values redacted. The file is
// created/appended automatically. Returns the logger and a cleanup func
// that closes the file.
func NewWithFile(levelStr, format, logPath string) (*slog.Logger, func(), error) {
	level, err := ParseLevel(levelStr)
	if err != nil {
		return nil, nil, err
	}
	consoleH, err := newHandler(os.Stderr, level, format)
	if err != nil {
		return nil, nil, err
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, nil, fmt.Errorf("log: mkdir: %w", err)
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("log: open %s: %w", logPath, err)
	}

	// File handler: always JSON, always debug level, always redacted.
	fileH := &redactingHandler{
		inner: slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug}),
	}

	multi := &multiHandler{handlers: []slog.Handler{consoleH, fileH}}
	return slog.New(multi), func() { f.Close() }, nil
}

func newHandler(w io.Writer, level slog.Level, format string) (slog.Handler, error) {
	opts := &slog.HandlerOptions{Level: level}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return slog.NewJSONHandler(w, opts), nil
	case "text":
		return slog.NewTextHandler(w, opts), nil
	case "", "auto":
		if useColor(w) {
			return tint.NewHandler(w, &tint.Options{
				Level:      level,
				TimeFormat: time.Kitchen,
			}), nil
		}
		return slog.NewTextHandler(w, opts), nil
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

// ParseLevel converts "debug" / "info" / "warn" / "error" into slog.Level.
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

// sensitiveKeys are log attribute keys whose values get redacted in file output.
var sensitiveKeys = map[string]bool{
	"cookie": true, "phpsessid": true, "token": true,
	"xsrf_token": true, "xsrf": true, "password": true,
	"secret": true, "webhook": true,
}

// redactingHandler wraps a slog.Handler and replaces sensitive attribute values.
type redactingHandler struct {
	inner slog.Handler
}

func (h *redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *redactingHandler) Handle(ctx context.Context, r slog.Record) error {
	redacted := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		redacted.AddAttrs(redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, redacted)
}

func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		out[i] = redactAttr(a)
	}
	return &redactingHandler{inner: h.inner.WithAttrs(out)}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{inner: h.inner.WithGroup(name)}
}

func redactAttr(a slog.Attr) slog.Attr {
	if sensitiveKeys[strings.ToLower(a.Key)] {
		return slog.String(a.Key, "***REDACTED***")
	}
	return a
}

// multiHandler fans out to multiple slog.Handlers.
type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

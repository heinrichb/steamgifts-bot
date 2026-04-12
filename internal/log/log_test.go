package log

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"":        slog.LevelInfo,
		"info":    slog.LevelInfo,
		"DEBUG":   slog.LevelDebug,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"err":     slog.LevelError,
	}
	for in, want := range cases {
		got, err := ParseLevel(in)
		if err != nil {
			t.Errorf("%s: %v", in, err)
		}
		if got != want {
			t.Errorf("%s: got %v, want %v", in, got, want)
		}
	}
	if _, err := ParseLevel("bogus"); err == nil {
		t.Error("expected error for unknown level")
	}
}

func TestNewAutoWritesPlainToBuffer(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New(&buf, "info", "auto")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	logger.Info("hello", "key", "value")
	out := buf.String()
	if !strings.Contains(out, "hello") || !strings.Contains(out, "key=value") {
		t.Errorf("expected text-handler output, got %q", out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("expected no ANSI escapes in non-TTY output, got %q", out)
	}
}

func TestNewJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New(&buf, "info", "json")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	logger.Info("hello", "key", "value")
	out := buf.String()
	if !strings.Contains(out, `"msg":"hello"`) || !strings.Contains(out, `"key":"value"`) {
		t.Errorf("expected JSON output, got %q", out)
	}
}

func TestNewTextFormat(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New(&buf, "info", "text")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	logger.Info("hello")
	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("expected text output, got %q", buf.String())
	}
}

func TestNewInvalidFormat(t *testing.T) {
	if _, err := New(nil, "info", "xml"); err == nil {
		t.Error("expected error for unknown format")
	}
}

func TestAccountChildLogger(t *testing.T) {
	var buf bytes.Buffer
	logger, _ := New(&buf, "info", "auto")
	child := Account(logger, "alt")
	child.Info("hi")
	if !strings.Contains(buf.String(), "account=alt") {
		t.Errorf("expected account=alt in output, got %q", buf.String())
	}
}

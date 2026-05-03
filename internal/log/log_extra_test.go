package log

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewNilWriter(t *testing.T) {
	// nil writer should default to os.Stderr without error.
	logger, err := New(nil, "info", "text")
	if err != nil {
		t.Fatalf("New with nil writer: %v", err)
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNewInvalidLevel(t *testing.T) {
	_, err := New(nil, "bogus", "text")
	if err == nil {
		t.Fatal("expected error for invalid level")
	}
}

func TestNewWithFileCreatesFileAndLogs(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logger, cleanup, err := NewWithFile("debug", "text", logPath)
	if err != nil {
		t.Fatalf("NewWithFile: %v", err)
	}
	defer cleanup()

	logger.Info("test message", "key", "value")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "test message") {
		t.Errorf("expected 'test message' in log, got: %s", content)
	}
	// File handler should use JSON format.
	if !strings.Contains(content, `"msg"`) {
		t.Errorf("expected JSON format in file, got: %s", content)
	}
}

func TestNewWithFileRedactsSensitiveKeys(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logger, cleanup, err := NewWithFile("debug", "text", logPath)
	if err != nil {
		t.Fatalf("NewWithFile: %v", err)
	}
	defer cleanup()

	logger.Info("auth", "cookie", "super-secret-value", "password", "s3cret")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, "super-secret-value") {
		t.Error("cookie value should be redacted in file output")
	}
	if strings.Contains(content, "s3cret") {
		t.Error("password value should be redacted in file output")
	}
	if !strings.Contains(content, "REDACTED") {
		t.Error("expected REDACTED placeholder")
	}
}

func TestNewWithFileInvalidLevel(t *testing.T) {
	dir := t.TempDir()
	_, _, err := NewWithFile("bogus", "text", filepath.Join(dir, "test.log"))
	if err == nil {
		t.Fatal("expected error for invalid level")
	}
}

func TestNewWithFileInvalidFormat(t *testing.T) {
	dir := t.TempDir()
	_, _, err := NewWithFile("info", "xml", filepath.Join(dir, "test.log"))
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestNewWithFileHonorsInfoLevel(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logger, cleanup, err := NewWithFile("info", "text", logPath)
	if err != nil {
		t.Fatalf("NewWithFile: %v", err)
	}
	defer cleanup()

	logger.Debug("debug noise", "k", "v")
	logger.Info("info kept", "k", "v")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "debug noise") {
		t.Errorf("debug record should not appear at info level, got: %s", content)
	}
	if !strings.Contains(content, "info kept") {
		t.Errorf("info record should appear, got: %s", content)
	}
}

func TestNewWithFileCreatesFileWithRestrictivePerms(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	_, cleanup, err := NewWithFile("info", "json", logPath)
	if err != nil {
		t.Fatalf("NewWithFile: %v", err)
	}
	defer cleanup()

	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat log: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("log file perm = %v, want 0o600", perm)
	}
}

func TestUseColorNonFile(t *testing.T) {
	var buf bytes.Buffer
	if useColor(&buf) {
		t.Error("expected false for non-file writer")
	}
}

func TestUseColorWithNoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	f, _ := os.CreateTemp(t.TempDir(), "test")
	defer f.Close()
	if useColor(f) {
		t.Error("expected false when NO_COLOR is set")
	}
}

func TestUseColorDumbTerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	f, _ := os.CreateTemp(t.TempDir(), "test")
	defer f.Close()
	if useColor(f) {
		t.Error("expected false for dumb terminal")
	}
}

func TestRedactingHandlerWithAttrsRedactsPresetKeys(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := &redactingHandler{inner: inner}

	// Pre-set a sensitive attr via WithAttrs, then emit a record.
	h2 := h.WithAttrs([]slog.Attr{slog.String("cookie", "super-secret")})
	logger := slog.New(h2)
	logger.Info("test")

	out := buf.String()
	if strings.Contains(out, "super-secret") {
		t.Errorf("cookie value should be redacted in WithAttrs output, got: %s", out)
	}
	if !strings.Contains(out, "REDACTED") {
		t.Errorf("expected REDACTED placeholder in output, got: %s", out)
	}
}

func TestRedactingHandlerWithGroupPreservesGroup(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := &redactingHandler{inner: inner}

	logger := slog.New(h.WithGroup("auth"))
	logger.Info("login", "user", "alice")

	out := buf.String()
	if !strings.Contains(out, "auth") {
		t.Errorf("expected group name 'auth' in output, got: %s", out)
	}
	if !strings.Contains(out, "alice") {
		t.Errorf("expected non-sensitive value in output, got: %s", out)
	}
}

func TestMultiHandlerEnabled(t *testing.T) {
	debugH := slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelDebug})
	errorH := slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError})
	m := &multiHandler{handlers: []slog.Handler{debugH, errorH}}

	// Debug enabled because first handler accepts it.
	if !m.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected debug enabled")
	}
}

func TestMultiHandlerHandle(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	h1 := slog.NewTextHandler(&buf1, &slog.HandlerOptions{Level: slog.LevelInfo})
	h2 := slog.NewTextHandler(&buf2, &slog.HandlerOptions{Level: slog.LevelInfo})
	m := &multiHandler{handlers: []slog.Handler{h1, h2}}

	logger := slog.New(m)
	logger.Info("hello")

	if !strings.Contains(buf1.String(), "hello") {
		t.Error("expected output in first handler")
	}
	if !strings.Contains(buf2.String(), "hello") {
		t.Error("expected output in second handler")
	}
}

func TestMultiHandlerWithAttrsFlowsToAll(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	h1 := slog.NewTextHandler(&buf1, &slog.HandlerOptions{Level: slog.LevelInfo})
	h2 := slog.NewTextHandler(&buf2, &slog.HandlerOptions{Level: slog.LevelInfo})
	m := &multiHandler{handlers: []slog.Handler{h1, h2}}

	logger := slog.New(m.WithAttrs([]slog.Attr{slog.String("env", "test")}))
	logger.Info("ping")

	if !strings.Contains(buf1.String(), "env=test") {
		t.Errorf("expected attr in handler 1, got: %s", buf1.String())
	}
	if !strings.Contains(buf2.String(), "env=test") {
		t.Errorf("expected attr in handler 2, got: %s", buf2.String())
	}
}

func TestMultiHandlerWithGroupFlowsToAll(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	h1 := slog.NewTextHandler(&buf1, &slog.HandlerOptions{Level: slog.LevelInfo})
	h2 := slog.NewTextHandler(&buf2, &slog.HandlerOptions{Level: slog.LevelInfo})
	m := &multiHandler{handlers: []slog.Handler{h1, h2}}

	logger := slog.New(m.WithGroup("req"))
	logger.Info("hit", "path", "/foo")

	if !strings.Contains(buf1.String(), "req.path=/foo") {
		t.Errorf("expected grouped attr in handler 1, got: %s", buf1.String())
	}
	if !strings.Contains(buf2.String(), "req.path=/foo") {
		t.Errorf("expected grouped attr in handler 2, got: %s", buf2.String())
	}
}

func TestRedactAttr(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"cookie", "***REDACTED***"},
		{"password", "***REDACTED***"},
		{"token", "***REDACTED***"},
		{"xsrf_token", "***REDACTED***"},
		{"webhook", "***REDACTED***"},
		{"proxy", "***REDACTED***"},
		{"secret", "***REDACTED***"},
		{"safe_key", "value"},
	}
	for _, tc := range tests {
		a := redactAttr(slog.String(tc.key, "value"))
		if a.Value.String() != tc.expected {
			t.Errorf("redactAttr(%q) = %q; want %q", tc.key, a.Value.String(), tc.expected)
		}
	}
}

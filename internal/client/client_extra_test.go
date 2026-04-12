package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/heinrichb/steamgifts-bot/internal/ratelimit"
)

func TestWithLogger(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "ok")
	}))
	defer srv.Close()

	c, err := New("test-cookie", "",
		WithBaseURL(srv.URL),
		WithLimiter(ratelimit.New(0, 0)),
		WithLogger(nil), // nil logger; should fall back to slog.Default()
	)
	if err != nil {
		t.Fatal(err)
	}
	// Should use DefaultUserAgent when empty string passed.
	if c.userAgent != DefaultUserAgent {
		t.Errorf("expected default UA, got %q", c.userAgent)
	}
}

func TestWithProxySetsTransport(t *testing.T) {
	c, err := New("test-cookie", "ua",
		WithProxy("http://proxy.example.com:8080"),
		WithLimiter(ratelimit.New(0, 0)),
	)
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := c.httpc.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport when proxy is set")
	}
	if transport.Proxy == nil {
		t.Error("expected proxy function to be set on transport")
	}
}

func TestWithProxyEmptyIsNoOp(t *testing.T) {
	c, err := New("test-cookie", "ua",
		WithProxy(""),
		WithLimiter(ratelimit.New(0, 0)),
	)
	if err != nil {
		t.Fatal(err)
	}
	// Empty proxy should not set a custom transport.
	if c.httpc.Transport != nil {
		t.Error("expected nil transport (default) when proxy is empty")
	}
}

func TestBaseURL(t *testing.T) {
	c, err := New("test-cookie", "ua",
		WithBaseURL("http://test.local"),
		WithLimiter(ratelimit.New(0, 0)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if c.BaseURL() != "http://test.local" {
		t.Errorf("BaseURL: %s", c.BaseURL())
	}
}

func TestGetAbsoluteURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "ok")
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	body, err := c.Get(context.Background(), srv.URL+"/absolute")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(body) != "ok" {
		t.Errorf("body: %q", body)
	}
}

func TestSnippetShort(t *testing.T) {
	got := Snippet([]byte("hello"))
	if got != "hello" {
		t.Errorf("Snippet: %q", got)
	}
}

func TestSnippetLong(t *testing.T) {
	long := strings.Repeat("x", 300)
	got := Snippet([]byte(long))
	// 200 chars + "…" (which is 3 bytes in UTF-8)
	if !strings.HasSuffix(got, "…") {
		t.Error("expected ellipsis suffix")
	}
	if !strings.HasPrefix(got, strings.Repeat("x", 200)) {
		t.Error("expected 200 chars before ellipsis")
	}
}

func TestHTTPErrorString(t *testing.T) {
	e := &HTTPError{Status: 403, URL: "http://example.com/", Body: "forbidden"}
	got := e.Error()
	if !strings.Contains(got, "403") || !strings.Contains(got, "forbidden") {
		t.Errorf("Error: %s", got)
	}
}

func TestRetryOn503(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		io.WriteString(w, "recovered")
	}))
	defer srv.Close()

	c, _ := New("test-cookie", "ua",
		WithBaseURL(srv.URL),
		WithLimiter(ratelimit.New(0, 0)),
		WithTimeout(30*time.Second),
	)
	body, err := c.Get(context.Background(), "/")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(body) != "recovered" {
		t.Errorf("body: %q", body)
	}
	if attempts < 3 {
		t.Errorf("expected at least 3 attempts, got %d", attempts)
	}
}

func TestContextCancelDuringBackoff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	c, _ := New("test-cookie", "ua",
		WithBaseURL(srv.URL),
		WithLimiter(ratelimit.New(0, 0)),
		WithTimeout(2*time.Second),
	)
	_, err := c.Get(ctx, "/")
	if err == nil {
		t.Fatal("expected error")
	}
}

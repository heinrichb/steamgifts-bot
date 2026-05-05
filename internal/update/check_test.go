package update

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// withTestServer overrides the package-level releaseURL and httpClient to
// point at an httptest server for the duration of a test.
func withTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	origURL := releaseURL
	origClient := httpClient
	releaseURL = srv.URL
	httpClient = srv.Client()
	t.Cleanup(func() {
		releaseURL = origURL
		httpClient = origClient
	})
	return srv
}

func TestCheckSkipsEmptyVersion(t *testing.T) {
	var buf bytes.Buffer
	Check(context.Background(), testLogger(&buf), "")
	if buf.Len() > 0 {
		t.Errorf("expected no log output for empty version, got: %s", buf.String())
	}
}

func TestCheckSkipsDevVersion(t *testing.T) {
	var buf bytes.Buffer
	Check(context.Background(), testLogger(&buf), "dev")
	if buf.Len() > 0 {
		t.Errorf("expected no log output for dev version, got: %s", buf.String())
	}
}

func TestCheckLogsWhenNewerAvailable(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(Release{
			TagName: "v2.0.0",
			HTMLURL: "https://github.com/heinrichb/steamgifts-bot/releases/tag/v2.0.0",
		})
	})

	var buf bytes.Buffer
	Check(context.Background(), testLogger(&buf), "v1.0.0")
	out := buf.String()
	if !strings.Contains(out, "newer version") {
		t.Errorf("expected 'newer version' warning, got: %s", out)
	}
	if !strings.Contains(out, "v2.0.0") {
		t.Errorf("expected latest version in log, got: %s", out)
	}
}

func TestCheckSameVersionNoLog(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(Release{TagName: "v1.0.0", HTMLURL: "https://example.com"})
	})

	var buf bytes.Buffer
	Check(context.Background(), testLogger(&buf), "v1.0.0")
	if strings.Contains(buf.String(), "newer version") {
		t.Errorf("should not warn when versions match, got: %s", buf.String())
	}
}

func TestCheckSilentOnHTTPError(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	var buf bytes.Buffer
	Check(context.Background(), testLogger(&buf), "v1.0.0")
	if strings.Contains(buf.String(), "newer version") {
		t.Error("should not log about newer version on HTTP error")
	}
}

func TestCheckSilentOnBadJSON(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("{bad json"))
	})

	var buf bytes.Buffer
	Check(context.Background(), testLogger(&buf), "v1.0.0")
	if buf.Len() > 0 {
		t.Errorf("expected no log on bad JSON, got: %s", buf.String())
	}
}

func TestCheckSilentOnCancelledContext(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(Release{TagName: "v9.0.0"})
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var buf bytes.Buffer
	Check(ctx, testLogger(&buf), "v1.0.0")
	if strings.Contains(buf.String(), "newer version") {
		t.Error("should not log when context is cancelled")
	}
}

func TestCheckStripsVPrefix(t *testing.T) {
	withTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(Release{TagName: "v1.5.0"})
	})

	var buf bytes.Buffer
	// Pass without "v" prefix — should still compare correctly.
	Check(context.Background(), testLogger(&buf), "1.5.0")
	if strings.Contains(buf.String(), "newer version") {
		t.Error("v1.5.0 should match 1.5.0 after prefix stripping")
	}
}

func TestCheckSendsAcceptHeader(t *testing.T) {
	var gotAccept string
	withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		json.NewEncoder(w).Encode(Release{TagName: "v1.0.0"})
	})

	var buf bytes.Buffer
	Check(context.Background(), testLogger(&buf), "v1.0.0")
	if gotAccept != "application/vnd.github+json" {
		t.Errorf("Accept header: %q", gotAccept)
	}
}

package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/heinrichb/steamgifts-bot/internal/ratelimit"
)

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c, err := New("test-cookie", "ua/test",
		WithBaseURL(srv.URL),
		WithLimiter(ratelimit.New(0, 0)),
		WithTimeout(2*time.Second),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestGetSendsCookieAndUserAgent(t *testing.T) {
	var gotUA, gotCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		if c, err := r.Cookie("PHPSESSID"); err == nil {
			gotCookie = c.Value
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	body, err := c.Get(context.Background(), "/")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(body) != "ok" {
		t.Errorf("body: %q", body)
	}
	if gotUA != "ua/test" {
		t.Errorf("user-agent: %q", gotUA)
	}
	if gotCookie != "test-cookie" {
		t.Errorf("cookie: %q", gotCookie)
	}
}

func TestPostFormEncodesBody(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.PostForm
		_, _ = io.WriteString(w, `{"type":"success"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	form := url.Values{"xsrf_token": {"abc"}, "do": {"entry_insert"}, "code": {"X1"}}
	body, err := c.PostForm(context.Background(), "/ajax.php", form)
	if err != nil {
		t.Fatalf("PostForm: %v", err)
	}
	if !strings.Contains(string(body), "success") {
		t.Errorf("body: %q", body)
	}
	if gotForm.Get("xsrf_token") != "abc" || gotForm.Get("code") != "X1" {
		t.Errorf("form not encoded correctly: %v", gotForm)
	}
}

func TestNon2xxReturnsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, "nope")
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Get(context.Background(), "/")
	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("expected *HTTPError, got %T: %v", err, err)
	}
	if httpErr.Status != http.StatusForbidden {
		t.Errorf("status: %d", httpErr.Status)
	}
}

func TestEmptyCookieRejected(t *testing.T) {
	if _, err := New("   ", "ua"); err == nil {
		t.Fatal("expected empty-cookie error")
	}
}

package steamgifts

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/heinrichb/steamgifts-bot/internal/client"
	"github.com/heinrichb/steamgifts-bot/internal/ratelimit"
)

func newClient(t *testing.T, srv *httptest.Server) *client.Client {
	t.Helper()
	c, err := client.New("cookie", "ua",
		client.WithBaseURL(srv.URL),
		client.WithLimiter(ratelimit.New(0, 0)),
		client.WithTimeout(2*time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEnterSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("xsrf_token") != "tok" || r.PostForm.Get("code") != "ABC1" {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, `{"type":"success","entry_count":"43","points":87}`)
	}))
	defer srv.Close()

	res, err := Enter(context.Background(), newClient(t, srv), "ABC1", "tok")
	if err != nil {
		t.Fatalf("Enter: %v", err)
	}
	if res.Type != "success" || res.Points != 87 || res.EntryCount != "43" {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestEnterServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"type":"error","msg":"Already entered"}`)
	}))
	defer srv.Close()

	_, err := Enter(context.Background(), newClient(t, srv), "ABC1", "tok")
	if err == nil || !strings.Contains(err.Error(), "Already entered") {
		t.Fatalf("expected server error to surface, got: %v", err)
	}
}

func TestFilterURL(t *testing.T) {
	cases := map[string]string{
		"wishlist":    "/?type=wishlist",
		"group":       "/?type=group",
		"recommended": "/?type=recommended",
		"new":         "/?type=new",
		"all":         "/",
	}
	for in, want := range cases {
		got, err := FilterURL(in)
		if err != nil {
			t.Errorf("%s: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("%s: got %q, want %q", in, got, want)
		}
	}
	if _, err := FilterURL("bogus"); err == nil {
		t.Error("expected error for unknown filter")
	}
}

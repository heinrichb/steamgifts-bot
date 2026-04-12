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
	if res.Type != "success" || res.PointsValue() != 87 || res.EntryCount != "43" {
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

func TestEnterErrorWithStringPoints(t *testing.T) {
	// Steamgifts sends points as a string in error responses (unlike
	// success responses where it's an int).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"type":"error","msg":"Level 2 Required","points":"398"}`)
	}))
	defer srv.Close()

	res, err := Enter(context.Background(), newClient(t, srv), "ABC1", "tok")
	if err == nil || !strings.Contains(err.Error(), "Level 2 Required") {
		t.Fatalf("expected level-required error, got: %v", err)
	}
	if res.PointsValue() != 398 {
		t.Errorf("string points not parsed: got %d, want 398", res.PointsValue())
	}
}

func TestFilterURL(t *testing.T) {
	cases := map[string]string{
		"wishlist":    "/giveaways/search?type=wishlist",
		"group":       "/giveaways/search?type=group",
		"recommended": "/giveaways/search?type=recommended",
		"new":         "/giveaways/search?type=new",
		"dlc":         "/giveaways/search?dlc=true",
		"multicopy":   "/giveaways/search?copy_min=2",
		"all":         "/giveaways/search",
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

func TestWithPage(t *testing.T) {
	cases := []struct {
		path string
		page int
		want string
	}{
		{"/giveaways/search?type=wishlist", 1, "/giveaways/search?type=wishlist"},
		{"/giveaways/search?type=wishlist", 2, "/giveaways/search?type=wishlist&page=2"},
		{"/giveaways/search", 1, "/giveaways/search"},
		{"/giveaways/search", 3, "/giveaways/search?page=3"},
		{"/giveaways/search?dlc=true", 5, "/giveaways/search?dlc=true&page=5"},
	}
	for _, tc := range cases {
		got := WithPage(tc.path, tc.page)
		if got != tc.want {
			t.Errorf("WithPage(%q, %d) = %q, want %q", tc.path, tc.page, got, tc.want)
		}
	}
}

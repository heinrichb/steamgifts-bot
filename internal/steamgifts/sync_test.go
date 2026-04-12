package steamgifts

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSyncAccountSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("xsrf_token") != "tok" || r.PostForm.Get("do") != "sync" {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, `{"sync_privacy_requirements":false,"type":"success","msg":"Success. Synced with Steam."}`)
	}))
	defer srv.Close()

	res, err := SyncAccount(context.Background(), newClient(t, srv), "tok")
	if err != nil {
		t.Fatalf("SyncAccount: %v", err)
	}
	if res.Type != "success" {
		t.Errorf("type: got %q, want success", res.Type)
	}
	if !strings.Contains(res.Msg, "Synced") {
		t.Errorf("msg: %q", res.Msg)
	}
}

func TestSyncAccountFailureSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"type":"error","msg":"You can only sync once per day."}`)
	}))
	defer srv.Close()

	_, err := SyncAccount(context.Background(), newClient(t, srv), "tok")
	if err == nil || !strings.Contains(err.Error(), "once per day") {
		t.Fatalf("expected cooldown error to surface, got: %v", err)
	}
}

// Package web provides an optional embedded dashboard for monitoring the
// bot via a browser. Enable with --dashboard-addr :8080.
package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/heinrichb/steamgifts-bot/internal/account"
)

//go:embed templates/*.html
var templateFS embed.FS

var tmpl = template.Must(template.New("").Funcs(template.FuncMap{
	"timeAgo": func(t time.Time) string {
		if t.IsZero() {
			return "never"
		}
		d := time.Since(t).Round(time.Second)
		if d < time.Minute {
			return fmt.Sprintf("%ds ago", int(d.Seconds()))
		}
		if d < time.Hour {
			return fmt.Sprintf("%dm ago", int(d.Minutes()))
		}
		return fmt.Sprintf("%dh %dm ago", int(d.Hours()), int(d.Minutes())%60)
	},
	"timeUntil": func(t time.Time) string {
		if t.IsZero() {
			return "-"
		}
		d := time.Until(t).Round(time.Second)
		if d <= 0 {
			return "now"
		}
		if d < time.Minute {
			return fmt.Sprintf("%ds", int(d.Seconds()))
		}
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	},
}).ParseFS(templateFS, "templates/*.html"))

// Serve starts the dashboard HTTP server. Blocks until ctx is cancelled.
func Serve(ctx context.Context, addr string, orch *account.Orchestrator) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data := struct {
			Accounts []account.Status
			Now      time.Time
		}{
			Accounts: orch.Snapshot(),
			Now:      time.Now(),
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "dashboard.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("dashboard: %w", err)
	}
	return nil
}

// Package metrics exposes Prometheus counters and gauges for the bot.
// Start the HTTP server with Serve(); register counters before the
// first scan cycle by importing this package.
package metrics

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	EntriesAttempted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "steamgifts_entries_attempted_total",
		Help: "Total giveaway entries attempted.",
	}, []string{"account"})

	EntriesSucceeded = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "steamgifts_entries_succeeded_total",
		Help: "Total giveaway entries that succeeded.",
	}, []string{"account"})

	EntriesFailed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "steamgifts_entries_failed_total",
		Help: "Total giveaway entries that failed.",
	}, []string{"account"})

	Points = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "steamgifts_points",
		Help: "Current point balance per account.",
	}, []string{"account"})

	CyclesCompleted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "steamgifts_cycles_completed_total",
		Help: "Total scan cycles completed.",
	}, []string{"account"})

	CandidatesScanned = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "steamgifts_candidates_scanned",
		Help: "Number of joinable candidates found in the last cycle.",
	}, []string{"account"})

	SyncSucceeded = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "steamgifts_sync_succeeded_total",
		Help: "Total successful Steam sync operations.",
	}, []string{"account"})

	WinsDetected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "steamgifts_wins_detected_total",
		Help: "Total new wins detected.",
	}, []string{"account"})
)

func init() {
	prometheus.MustRegister(
		EntriesAttempted,
		EntriesSucceeded,
		EntriesFailed,
		Points,
		CyclesCompleted,
		CandidatesScanned,
		SyncSucceeded,
		WinsDetected,
	)
}

// Serve starts an HTTP server on the given address exposing /metrics.
// Blocks until ctx is cancelled.
func Serve(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("metrics: %w", err)
	}
	return nil
}

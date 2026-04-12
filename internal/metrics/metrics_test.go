package metrics

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestMetricsRegistered(t *testing.T) {
	// The init() function registers all metrics. Verify they are non-nil
	// and can be used without panic.
	EntriesAttempted.WithLabelValues("test").Inc()
	EntriesSucceeded.WithLabelValues("test").Inc()
	EntriesFailed.WithLabelValues("test").Inc()
	Points.WithLabelValues("test").Set(100)
	CyclesCompleted.WithLabelValues("test").Inc()
	CandidatesScanned.WithLabelValues("test").Set(50)
	SyncSucceeded.WithLabelValues("test").Inc()
	WinsDetected.WithLabelValues("test").Inc()
}

func TestServeStartsAndStops(t *testing.T) {
	// Find a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- Serve(ctx, addr)
	}()

	// Wait for server to start.
	time.Sleep(50 * time.Millisecond)

	// Verify the metrics endpoint responds.
	resp, err := http.Get("http://" + addr + "/metrics")
	if err != nil {
		cancel()
		t.Fatalf("GET /metrics: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: %d", resp.StatusCode)
	}

	// Stop the server.
	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("Serve: %v", err)
	}
}

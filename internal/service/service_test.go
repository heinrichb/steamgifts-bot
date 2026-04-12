package service

import (
	"runtime"
	"testing"
)

func TestSupported(t *testing.T) {
	got := Supported()
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
		if !got {
			t.Errorf("expected Supported()=true on %s", runtime.GOOS)
		}
	default:
		if got {
			t.Errorf("expected Supported()=false on %s", runtime.GOOS)
		}
	}
}

// Package service installs and removes per-user background services so the
// bot starts automatically on login.
//
// Each platform has its own implementation behind build-tag-gated files:
//
//   - service_linux.go   — systemd user unit
//   - service_windows.go — Scheduled Task via schtasks.exe
//   - service_other.go   — graceful "not supported on this OS" stubs
package service

import "runtime"

// Platform returns the GOOS-style identifier of the current system.
// Exposed so the wizard can show platform-appropriate copy.
func Platform() string { return runtime.GOOS }

// Supported reports whether `Install` does anything meaningful here.
func Supported() bool {
	switch runtime.GOOS {
	case "linux", "windows":
		return true
	default:
		return false
	}
}

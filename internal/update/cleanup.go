package update

import (
	"os"
	"path/filepath"
)

// CleanupOldBinary removes leftover .old files from a previous self-update.
// Safe to call on any platform — silently does nothing if no .old exists.
func CleanupOldBinary() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return
	}
	_ = os.Remove(exe + ".old")
}

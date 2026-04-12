//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	getConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
)

func ensureConsole() {}

// launchedFromExplorer reports whether this process is the sole occupant
// of its console — true when double-clicked from Explorer.
func launchedFromExplorer() bool {
	var pids [4]uint32
	count, _, _ := getConsoleProcessList.Call(
		uintptr(unsafe.Pointer(&pids[0])),
		4,
	)
	return count <= 1
}

// waitBeforeClose pauses so the user can read output before the
// Explorer-spawned console vanishes.
func waitBeforeClose() {
	if launchedFromExplorer() {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Press Enter to close this window...")
		fmt.Scanln()
	}
}

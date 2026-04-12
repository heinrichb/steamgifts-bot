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

// ensureConsole is a no-op for console-subsystem builds. Windows allocates
// a console automatically: the parent's when launched from cmd/PowerShell,
// or a fresh one when double-clicked from Explorer.
func ensureConsole() {}

// launchedFromExplorer returns true when this process is the only one on
// the console — meaning Windows created a throwaway console for a
// double-click launch. In that case the window would vanish instantly
// when main() returns, so waitBeforeClose pauses for the user.
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

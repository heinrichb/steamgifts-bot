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
	setConsoleTitle       = kernel32.NewProc("SetConsoleTitleW")
	allocConsole          = kernel32.NewProc("AllocConsole")
	getConsoleWindow      = kernel32.NewProc("GetConsoleWindow")
)

// launchedFromExplorer returns true if this process is the only one
// attached to its console — meaning Explorer spawned a new cmd window
// just for us (the user double-clicked the .exe). When launched from an
// existing cmd/PowerShell, count is >= 2.
func launchedFromExplorer() bool {
	var pids [4]uint32
	count, _, _ := getConsoleProcessList.Call(
		uintptr(unsafe.Pointer(&pids[0])),
		4,
	)
	return count <= 1
}

// ensureConsole makes sure we have a visible console window. Some
// Windows configurations (e.g. Windows Terminal as default) can
// interfere with console app double-click launches.
func ensureConsole() {
	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd == 0 {
		allocConsole.Call()
	}
	title, _ := syscall.UTF16PtrFromString("steamgifts-bot")
	setConsoleTitle.Call(uintptr(unsafe.Pointer(title)))
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

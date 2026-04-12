//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const attachParentProcess = ^uint32(0) // ATTACH_PARENT_PROCESS = -1

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	attachConsole         = kernel32.NewProc("AttachConsole")
	allocConsole          = kernel32.NewProc("AllocConsole")
	getConsoleWindow      = kernel32.NewProc("GetConsoleWindow")
	getConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
	setConsoleTitle       = kernel32.NewProc("SetConsoleTitleW")
)

// ensureConsole sets up a visible console for a GUI-subsystem binary.
//
// With -H windowsgui, Windows doesn't auto-allocate a console. We:
//  1. Try AttachConsole(ATTACH_PARENT_PROCESS) — succeeds when launched
//     from cmd.exe or PowerShell, reusing the parent's console.
//  2. If that fails (double-click from Explorer), AllocConsole creates
//     a new one.
//  3. Reopen os.Stdout/Stderr/Stdin to CONOUT$/CONIN$ so Go's runtime
//     and all libraries write to the visible console.
func ensureConsole() {
	// Try to attach to the parent's console (launched from cmd/PowerShell).
	ret, _, _ := attachConsole.Call(uintptr(attachParentProcess))
	if ret == 0 {
		// No parent console — allocate our own (double-click from Explorer).
		allocConsole.Call()
	}
	reopenStdHandles()
	title, _ := syscall.UTF16PtrFromString("steamgifts-bot")
	setConsoleTitle.Call(uintptr(unsafe.Pointer(title)))
}

// reopenStdHandles points os.Stdout/Stderr/Stdin at the console we just
// attached or allocated. For GUI-subsystem binaries, Go's runtime may
// have set these to invalid handles at startup.
func reopenStdHandles() {
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout = f
		os.Stderr = f
	}
	if f, err := os.OpenFile("CONIN$", os.O_RDONLY, 0); err == nil {
		os.Stdin = f
	}
}

// launchedFromExplorer returns true if we're the only process on this
// console — meaning we allocated it ourselves (Explorer double-click).
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

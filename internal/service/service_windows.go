//go:build windows

package service

import (
	"fmt"
	"os"
	"os/exec"
)

const taskName = "steamgifts-bot"

// Install registers a per-user Scheduled Task that runs the bot at logon.
// No admin elevation is required.
func Install() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("service: locate self: %w", err)
	}
	args := []string{
		"/Create", "/F",
		"/SC", "ONLOGON",
		"/TN", taskName,
		"/TR", fmt.Sprintf("\"%s\" run", exe),
		"/RL", "LIMITED",
	}
	if out, err := exec.Command("schtasks", args...).CombinedOutput(); err != nil {
		return "", fmt.Errorf("service: schtasks failed: %w (%s)", err, string(out))
	}
	return "Scheduled Task: " + taskName, nil
}

// Uninstall removes the Scheduled Task.
func Uninstall() error {
	if out, err := exec.Command("schtasks", "/Delete", "/F", "/TN", taskName).CombinedOutput(); err != nil {
		return fmt.Errorf("service: schtasks delete failed: %w (%s)", err, string(out))
	}
	return nil
}

// Status reports whether the Scheduled Task exists.
func Status() (string, error) {
	out, err := exec.Command("schtasks", "/Query", "/TN", taskName).CombinedOutput()
	if err != nil {
		return "not installed", nil
	}
	return "installed: " + taskName + "\n" + string(out), nil
}

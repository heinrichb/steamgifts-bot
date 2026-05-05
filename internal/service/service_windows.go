//go:build windows

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const startupFileName = "steamgifts-bot.bat"

// startupFolder returns the per-user Startup directory. Anything placed
// here runs automatically when the user logs in — no admin required.
func startupFolder() (string, error) {
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		return "", fmt.Errorf("service: %%APPDATA%% is not set")
	}
	return filepath.Join(appdata, "Microsoft", "Windows", "Start Menu", "Programs", "Startup"), nil
}

func startupPath() (string, error) {
	dir, err := startupFolder()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, startupFileName), nil
}

// Install creates a small .bat in the user's Startup folder that launches
// the bot minimized on login, then starts the bot immediately so a reboot
// is not required. No admin elevation required.
func Install() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("service: locate self: %w", err)
	}
	path, err := startupPath()
	if err != nil {
		return "", err
	}
	script := fmt.Sprintf("@echo off\r\nstart \"steamgifts-bot\" /MIN \"%s\" run\r\n", exe)
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		return "", fmt.Errorf("service: write startup script: %w", err)
	}
	// Start the bot now so the user doesn't have to log out and back in.
	cmd := exec.Command(exe, "run")
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Start()
	return path, nil
}

// Uninstall removes the startup .bat.
func Uninstall() error {
	path, err := startupPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("service: remove startup script: %w", err)
	}
	return nil
}

// IsInstalled reports whether the startup .bat exists.
func IsInstalled() bool {
	path, err := startupPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// Status reports whether the startup .bat exists.
func Status() (string, error) {
	path, err := startupPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err != nil {
		return "not installed", nil
	}
	return fmt.Sprintf("installed at %s", path), nil
}

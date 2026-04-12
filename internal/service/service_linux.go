//go:build linux

package service

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const unitName = "steamgifts-bot.service"

func unitPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("service: locate user config dir: %w", err)
	}
	return filepath.Join(dir, "systemd", "user", unitName), nil
}

// Install writes a systemd user unit and enables it via systemctl.
// Returns the path of the unit file it wrote.
func Install() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("service: locate self: %w", err)
	}
	path, err := unitPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("service: mkdir: %w", err)
	}
	unit := fmt.Sprintf(`[Unit]
Description=steamgifts-bot — multi-account giveaway bot
After=network-online.target
Wants=network-online.target

[Service]
ExecStart="%s" run
Restart=on-failure
RestartSec=30
Environment=NO_COLOR=1

[Install]
WantedBy=default.target
`, exe)
	if err := os.WriteFile(path, []byte(unit), 0o644); err != nil {
		return "", fmt.Errorf("service: write unit: %w", err)
	}

	// Best-effort enable + start. We surface errors but don't roll back.
	if _, err := exec.LookPath("systemctl"); err == nil {
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		if out, err := exec.Command("systemctl", "--user", "enable", "--now", unitName).CombinedOutput(); err != nil {
			return path, fmt.Errorf("service: systemctl enable failed: %w (%s)", err, string(out))
		}
	} else {
		return path, errors.New("service: systemctl not on PATH — unit written but not enabled")
	}
	return path, nil
}

// Uninstall stops, disables, and removes the systemd user unit.
func Uninstall() error {
	path, err := unitPath()
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("systemctl"); err == nil {
		_ = exec.Command("systemctl", "--user", "disable", "--now", unitName).Run()
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("service: remove unit: %w", err)
	}
	return nil
}

// Status returns a human-readable status line.
func Status() (string, error) {
	path, err := unitPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err != nil {
		return "not installed", nil
	}
	// systemctl is-active exits non-zero for inactive/failed units, but its
	// stdout still carries the actual state — capture both.
	out, _ := exec.Command("systemctl", "--user", "is-active", unitName).CombinedOutput()
	state := strings.TrimSpace(string(out))
	if state == "" {
		state = "unknown"
	}
	return fmt.Sprintf("installed at %s — %s", path, state), nil
}

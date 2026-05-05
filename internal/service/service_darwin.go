//go:build darwin

package service

import (
	"fmt"
	"os"
	"path/filepath"
)

const plistLabel = "com.heinrichb.steamgifts-bot"

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("service: home dir: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", plistLabel+".plist"), nil
}

// Install writes a LaunchAgent plist that starts the bot on login.
func Install() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("service: locate self: %w", err)
	}
	path, err := plistPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("service: mkdir: %w", err)
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>run</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
    </dict>
    <key>StandardOutPath</key>
    <string>/tmp/steamgifts-bot.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/steamgifts-bot.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin</string>
    </dict>
</dict>
</plist>
`, plistLabel, exe)
	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		return "", fmt.Errorf("service: write plist: %w", err)
	}
	return path, nil
}

// Uninstall removes the LaunchAgent plist.
func Uninstall() error {
	path, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("service: remove plist: %w", err)
	}
	return nil
}

// IsInstalled reports whether the LaunchAgent plist exists.
func IsInstalled() bool {
	path, err := plistPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// Status reports whether the LaunchAgent is installed.
func Status() (string, error) {
	path, err := plistPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err != nil {
		return "not installed", nil
	}
	return fmt.Sprintf("installed at %s", path), nil
}

//go:build !linux && !windows && !darwin

package service

import "errors"

// Install on unsupported platforms returns a friendly error.
func Install() (string, error) {
	return "", errors.New("service install is not yet supported on this OS — see TODO.md")
}

// Uninstall on unsupported platforms returns a friendly error.
func Uninstall() error {
	return errors.New("service install is not yet supported on this OS")
}

// Status on unsupported platforms returns a stub.
func Status() (string, error) {
	return "not supported on this OS", nil
}

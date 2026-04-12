//go:build !windows

package cli

import (
	"os"
	"os/signal"
	"syscall"
)

func sighupChan() <-chan os.Signal {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	return ch
}

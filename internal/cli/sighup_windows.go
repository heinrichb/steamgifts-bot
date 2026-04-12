//go:build windows

package cli

import "os"

func sighupChan() <-chan os.Signal {
	return make(chan os.Signal) // never fires
}

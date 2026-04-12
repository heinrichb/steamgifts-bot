// Command steamgifts-bot is a multi-account giveaway bot for steamgifts.com.
package main

import (
	"fmt"
	"os"

	"github.com/heinrichb/steamgifts-bot/internal/cli"
)

// Build-time injected by GoReleaser / `go build -ldflags`.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	ensureConsole()

	code := 0
	if err := cli.NewRootCmd(version, commit, date).Execute(); err != nil {
		if msg := err.Error(); msg != "" {
			fmt.Fprintln(os.Stderr, "error:", msg)
		}
		code = 1
	}

	waitBeforeClose()

	if code != 0 {
		os.Exit(code)
	}
}

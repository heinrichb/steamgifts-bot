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
		fmt.Fprintln(os.Stderr, "error:", err)
		code = 1
	}

	// Must run BEFORE os.Exit — defer doesn't fire after os.Exit.
	waitBeforeClose()

	if code != 0 {
		os.Exit(code)
	}
}

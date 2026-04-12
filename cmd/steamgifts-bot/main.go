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
	if err := cli.NewRootCmd(version, commit, date).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

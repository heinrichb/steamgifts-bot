// Command refresh-fixtures fetches current steamgifts listing pages and
// saves them as parser test fixtures. Requires a valid config.yml with
// at least one account cookie.
//
// Usage: go run ./cmd/refresh-fixtures
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/heinrichb/steamgifts-bot/internal/client"
	"github.com/heinrichb/steamgifts-bot/internal/ratelimit"
	"go.yaml.in/yaml/v3"
)

type cfg struct {
	Accounts []struct {
		Cookie string `yaml:"cookie"`
	} `yaml:"accounts"`
}

var cookieRe = regexp.MustCompile(`PHPSESSID=[^;]+`)

func main() {
	b, err := os.ReadFile("config.yml")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: need config.yml with at least one account cookie")
		os.Exit(1)
	}
	var c cfg
	if err := yaml.Unmarshal(b, &c); err != nil || len(c.Accounts) == 0 {
		fmt.Fprintln(os.Stderr, "error: config.yml must have at least one account")
		os.Exit(1)
	}

	cl, err := client.New(c.Accounts[0].Cookie, client.DefaultUserAgent,
		client.WithLimiter(ratelimit.New(1*time.Second, 2*time.Second)),
		client.WithTimeout(15*time.Second),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	ctx := context.Background()
	dir := filepath.Join("internal", "steamgifts", "testdata")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	pages := map[string]string{
		"list_real_all.html":      "/giveaways/search",
		"list_real_wishlist.html": "/giveaways/search?type=wishlist",
		"list_real_group.html":    "/giveaways/search?type=group",
	}

	for name, path := range pages {
		fmt.Printf("fetching %s → %s ... ", path, name)
		body, err := cl.Get(ctx, path)
		if err != nil {
			fmt.Printf("FAILED: %v\n", err)
			continue
		}
		// Redact cookies from any inline scripts or meta tags.
		redacted := cookieRe.ReplaceAll(body, []byte("PHPSESSID=REDACTED"))
		out := filepath.Join(dir, name)
		if err := os.WriteFile(out, redacted, 0o644); err != nil {
			fmt.Printf("FAILED: %v\n", err)
			continue
		}
		fmt.Printf("ok (%d bytes)\n", len(redacted))
	}
	fmt.Println("done. Review the files and commit if they look correct.")
}

// Package update checks GitHub Releases for a newer version of the bot.
// The check is non-blocking, opt-in (log message only), and never
// auto-downloads anything.
package update

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// releaseURL is the GitHub API endpoint for the latest release.
// Tests override this to point at an httptest server.
var releaseURL = "https://api.github.com/repos/heinrichb/steamgifts-bot/releases/latest"

// httpClient is the HTTP client used for update checks.
// Tests override this to avoid real network calls.
var httpClient = http.DefaultClient

type release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Check compares the running version against the latest GitHub release.
// Logs a message if a newer version is available. Silently returns on
// error (network down, rate-limited, etc.) — never interrupts startup.
func Check(ctx context.Context, logger *slog.Logger, currentVersion string) {
	if currentVersion == "" || currentVersion == "dev" {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return
	}

	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return
	}

	latest := strings.TrimPrefix(rel.TagName, "v")
	current := strings.TrimPrefix(currentVersion, "v")
	if latest != "" && latest != current {
		logger.Warn("a newer version is available",
			"current", currentVersion,
			"latest", rel.TagName,
			"download", rel.HTMLURL,
		)
	}
}

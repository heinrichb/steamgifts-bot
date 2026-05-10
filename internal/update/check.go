// Package update checks GitHub Releases for a newer version of the bot
// and can self-update the running binary.
package update

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// releaseURL is the GitHub API endpoint for the latest release.
// Tests override this to point at an httptest server.
var releaseURL = "https://api.github.com/repos/heinrichb/steamgifts-bot/releases/latest"

// httpClient is the HTTP client used for update checks.
// Tests override this to avoid real network calls.
var httpClient = http.DefaultClient

// Release holds metadata about an available release.
type Release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

// Asset is a downloadable file attached to a GitHub release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Result is the outcome of an update check.
type Result struct {
	Available      bool
	CurrentVersion string
	LatestVersion  string
	DownloadURL    string
	Release        Release
}

var (
	latestMu     sync.RWMutex
	latestResult *Result
)

// Latest returns the most recent check result, or nil if no check has completed.
func Latest() *Result {
	latestMu.RLock()
	defer latestMu.RUnlock()
	if latestResult == nil {
		return nil
	}
	r := *latestResult
	return &r
}

// Check compares the running version against the latest GitHub release.
// Stores the result for later retrieval via Latest(). Logs a message if a
// newer version is available. Silently returns on error.
func Check(ctx context.Context, logger *slog.Logger, currentVersion string) {
	result := check(ctx, currentVersion)
	if result == nil {
		return
	}

	latestMu.Lock()
	latestResult = result
	latestMu.Unlock()

	if result.Available {
		logger.Warn("a newer version is available",
			"current", currentVersion,
			"latest", result.LatestVersion,
			"download", result.Release.HTMLURL,
		)
	}
}

func check(ctx context.Context, currentVersion string) *Result {
	if currentVersion == "" || strings.HasPrefix(currentVersion, "dev") {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil
	}

	latest := strings.TrimPrefix(rel.TagName, "v")
	current := strings.TrimPrefix(currentVersion, "v")

	return &Result{
		Available:      latest != "" && latest != current,
		CurrentVersion: currentVersion,
		LatestVersion:  rel.TagName,
		DownloadURL:    rel.HTMLURL,
		Release:        rel,
	}
}

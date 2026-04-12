// Package notify sends notifications when the bot wins a giveaway.
// Currently supports Discord webhooks; other targets (Telegram, generic
// webhook) can be added as additional Send* functions.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Notifier sends win notifications to configured targets.
type Notifier struct {
	DiscordURL string
	httpClient *http.Client
}

// New creates a Notifier. If discordURL is empty, notifications are disabled.
func New(discordURL string) *Notifier {
	return &Notifier{
		DiscordURL: discordURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Enabled reports whether any notification target is configured.
func (n *Notifier) Enabled() bool {
	return n.DiscordURL != ""
}

// Win represents a won giveaway to notify about.
type Win struct {
	GameName    string
	GiveawayURL string
	AccountName string
}

// SendWin sends a win notification to all configured targets.
func (n *Notifier) SendWin(ctx context.Context, win Win) error {
	if n.DiscordURL == "" {
		return nil
	}
	return n.sendDiscord(ctx, win)
}

func (n *Notifier) sendDiscord(ctx context.Context, win Win) error {
	embed := map[string]any{
		"title":       fmt.Sprintf("🎉 Won: %s", win.GameName),
		"description": fmt.Sprintf("Account **%s** won a giveaway!", win.AccountName),
		"color":       0x00FF00,
		"fields": []map[string]any{
			{"name": "Game", "value": win.GameName, "inline": true},
			{"name": "Account", "value": win.AccountName, "inline": true},
		},
	}
	if win.GiveawayURL != "" {
		embed["url"] = win.GiveawayURL
	}

	payload := map[string]any{
		"embeds": []any{embed},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notify: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.DiscordURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notify: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("notify: discord webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("notify: discord returned %d", resp.StatusCode)
	}
	return nil
}

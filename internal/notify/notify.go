// Package notify sends notifications when the bot wins a giveaway.
// Supports Discord webhooks and Telegram bot messages.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const discordColorGreen = 0x00FF00

// telegramBaseURL is the Telegram Bot API base URL.
// Tests override this to point at an httptest server.
var telegramBaseURL = "https://api.telegram.org"

// Notifier sends win notifications to configured targets.
type Notifier struct {
	DiscordURL    string
	TelegramToken string
	TelegramChat  string
	httpClient    *http.Client
}

// New creates a Notifier. All fields are optional — empty = disabled.
func New(discordURL, telegramToken, telegramChat string) *Notifier {
	return &Notifier{
		DiscordURL:    discordURL,
		TelegramToken: telegramToken,
		TelegramChat:  telegramChat,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Enabled reports whether any notification target is configured.
func (n *Notifier) Enabled() bool {
	return n.DiscordURL != "" || (n.TelegramToken != "" && n.TelegramChat != "")
}

// Win represents a won giveaway to notify about.
type Win struct {
	GameName    string
	GiveawayURL string
	AccountName string
}

// SendWin sends a win notification to all configured targets.
func (n *Notifier) SendWin(ctx context.Context, win Win) error {
	var firstErr error
	if n.DiscordURL != "" {
		if err := n.sendDiscord(ctx, win); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if n.TelegramToken != "" && n.TelegramChat != "" {
		if err := n.sendTelegram(ctx, win); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (n *Notifier) sendDiscord(ctx context.Context, win Win) error {
	embed := map[string]any{
		"title":       fmt.Sprintf("🎉 Won: %s", win.GameName),
		"description": fmt.Sprintf("Account **%s** won a giveaway!", win.AccountName),
		"color":       discordColorGreen,
		"fields": []map[string]any{
			{"name": "Game", "value": win.GameName, "inline": true},
			{"name": "Account", "value": win.AccountName, "inline": true},
		},
	}
	if win.GiveawayURL != "" {
		embed["url"] = win.GiveawayURL
	}
	payload := map[string]any{"embeds": []any{embed}}
	return n.postJSON(ctx, n.DiscordURL, payload, "discord")
}

func (n *Notifier) sendTelegram(ctx context.Context, win Win) error {
	text := fmt.Sprintf("🎉 *Won: %s*\nAccount: %s", win.GameName, win.AccountName)
	if win.GiveawayURL != "" {
		text += fmt.Sprintf("\n[View giveaway](%s)", win.GiveawayURL)
	}
	url := fmt.Sprintf("%s/bot%s/sendMessage", telegramBaseURL, n.TelegramToken)
	payload := map[string]any{
		"chat_id":    n.TelegramChat,
		"text":       text,
		"parse_mode": "Markdown",
	}
	return n.postJSON(ctx, url, payload, "telegram")
}

func (n *Notifier) postJSON(ctx context.Context, url string, payload any, label string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notify: %s marshal: %w", label, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notify: %s build request: %w", label, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("notify: %s: %w", label, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("notify: %s returned %d", label, resp.StatusCode)
	}
	return nil
}

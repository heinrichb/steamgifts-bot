package steamgifts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/heinrichb/steamgifts-bot/internal/client"
)

// SyncResult is the JSON shape steamgifts returns from /ajax.php?do=sync.
// On cooldown the site returns a non-"success" Type with an explanatory Msg.
type SyncResult struct {
	Type string `json:"type"`
	Msg  string `json:"msg,omitempty"`
}

// SyncAccount POSTs the "sync" action to /ajax.php, asking steamgifts to
// re-sync the account with Steam. The site does this once a day automatically;
// triggering it manually refunds points for newly-acquired games and filters
// owned games out of future giveaway listings.
//
// The xsrf token must come from a recent ParseListPage call — steamgifts
// rotates it.
func SyncAccount(ctx context.Context, c *client.Client, xsrf string) (*SyncResult, error) {
	form := url.Values{
		"xsrf_token": {xsrf},
		"do":         {"sync"},
	}
	body, err := c.PostForm(ctx, ajaxPath, form)
	if err != nil {
		return nil, fmt.Errorf("sync: %w", err)
	}
	var res SyncResult
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("sync: decode response: %w (body: %s)", err, client.Snippet(body))
	}
	if res.Type != responseTypeSuccess {
		return &res, fmt.Errorf("sync: server returned %s: %s", res.Type, res.Msg)
	}
	return &res, nil
}

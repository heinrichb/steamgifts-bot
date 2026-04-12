package steamgifts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/heinrichb/steamgifts-bot/internal/client"
)

// EntryResult is the JSON shape steamgifts returns from /ajax.php?do=entry_insert.
//
// Successful response example:
//
//	{"type":"success","entry_count":"43","points":"87"}
type EntryResult struct {
	Type       string `json:"type"`
	EntryCount string `json:"entry_count,omitempty"`
	Points     int    `json:"points,omitempty"`
	Msg        string `json:"msg,omitempty"`
}

// Enter submits an entry for the given giveaway code using the supplied
// XSRF token. The token must come from the most recent ParseListPage call
// — steamgifts rotates it.
func Enter(ctx context.Context, c *client.Client, code, xsrf string) (*EntryResult, error) {
	form := url.Values{
		"xsrf_token": {xsrf},
		"do":         {"entry_insert"},
		"code":       {code},
	}
	body, err := c.PostForm(ctx, "/ajax.php", form)
	if err != nil {
		return nil, fmt.Errorf("enter %s: %w", code, err)
	}
	var res EntryResult
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("enter %s: decode response: %w (body: %s)", code, err, truncate(body, 200))
	}
	if res.Type != "success" {
		return &res, fmt.Errorf("enter %s: server returned %s: %s", code, res.Type, res.Msg)
	}
	return &res, nil
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}

package steamgifts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/heinrichb/steamgifts-bot/internal/client"
)

const (
	ajaxPath            = "/ajax.php"
	responseTypeSuccess = "success"
)

// EntryResult is the JSON shape steamgifts returns from /ajax.php?do=entry_insert.
//
// Steamgifts is inconsistent with the points field: success responses send
// it as a number (87), error responses send it as a string ("398"). The
// Points field uses any to handle both; call PointsValue() for the int.
type EntryResult struct {
	Type       string `json:"type"`
	EntryCount string `json:"entry_count,omitempty"`
	Points     any    `json:"points,omitempty"`
	Msg        string `json:"msg,omitempty"`
}

// PointsValue returns Points as an int, handling both the int (success)
// and string (error) JSON representations steamgifts sends.
func (r *EntryResult) PointsValue() int {
	switch v := r.Points.(type) {
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default:
		return 0
	}
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
	body, err := c.PostForm(ctx, ajaxPath, form)
	if err != nil {
		return nil, fmt.Errorf("enter %s: %w", code, err)
	}
	var res EntryResult
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("enter %s: decode response: %w (body: %s)", code, err, client.Snippet(body))
	}
	if res.Type != responseTypeSuccess {
		return &res, fmt.Errorf("enter %s: server returned %s: %s", code, res.Type, res.Msg)
	}
	return &res, nil
}

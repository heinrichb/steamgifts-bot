package steamgifts

import (
	"fmt"
	"strings"
)

// FilterURL maps a filter name (as used in config) to the relative path
// the bot should fetch to list giveaways for that filter.
//
// Steamgifts uses query-string filters on the root listing page.
func FilterURL(name string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "wishlist":
		return "/?type=wishlist", nil
	case "group":
		return "/?type=group", nil
	case "recommended":
		return "/?type=recommended", nil
	case "new":
		return "/?type=new", nil
	case "all", "":
		return "/", nil
	default:
		return "", fmt.Errorf("unknown filter %q", name)
	}
}

package steamgifts

import (
	"fmt"
	"strings"
)

// FilterURL maps a filter name (as used in config) to the relative path
// the bot should fetch to list giveaways for that filter.
//
// Steamgifts exposes filters as query parameters on /giveaways/search.
// The canonical URLs (taken from the site itself) are:
//
//	/giveaways/search?type=wishlist     — only games on your Steam wishlist
//	/giveaways/search?type=group        — group-only giveaways you're eligible for
//	/giveaways/search?type=recommended  — Steamgifts recommendations
//	/giveaways/search?type=new          — newest giveaways
//	/giveaways/search?dlc=true          — DLC-only
//	/giveaways/search?copy_min=2        — multi-copy giveaways (>=2 copies)
//	/giveaways/search                   — everything
//
// Future work (see TODO.md): support parameterized filters (e.g. copy_min=N
// for arbitrary N) and combined filters via a structured config schema.
func FilterURL(name string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "wishlist":
		return "/giveaways/search?type=wishlist", nil
	case "group":
		return "/giveaways/search?type=group", nil
	case "recommended":
		return "/giveaways/search?type=recommended", nil
	case "new":
		return "/giveaways/search?type=new", nil
	case "dlc":
		return "/giveaways/search?dlc=true", nil
	case "multicopy", "multi-copy", "copies":
		return "/giveaways/search?copy_min=2", nil
	case "all", "":
		return "/giveaways/search", nil
	default:
		return "", fmt.Errorf("unknown filter %q", name)
	}
}

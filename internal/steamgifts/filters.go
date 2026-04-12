package steamgifts

import (
	"fmt"
	"strings"
)

// Filter names accepted in the user's YAML `filters` list.
// Each maps to a canonical /giveaways/search URL via FilterURL.
const (
	FilterWishlist    = "wishlist"
	FilterGroup       = "group"
	FilterRecommended = "recommended"
	FilterNew         = "new"
	FilterDLC         = "dlc"
	FilterMultiCopy   = "multicopy"
	FilterAll         = "all"
)

// ValidFilterNames returns the canonical set of filter identifiers, in a
// stable order suitable for error messages and documentation.
func ValidFilterNames() []string {
	return []string{
		FilterWishlist, FilterGroup, FilterRecommended, FilterNew,
		FilterDLC, FilterMultiCopy, FilterAll,
	}
}

// IsValidFilter reports whether name is a recognised filter identifier.
// Thin wrapper over FilterURL so there's one source of truth.
func IsValidFilter(name string) bool {
	_, err := FilterURL(name)
	return err == nil
}

// FilterURL maps a filter name to the relative path the bot should fetch
// to list giveaways for that filter.
//
// Steamgifts exposes filters as query parameters on /giveaways/search.
// Future work (see TODO.md): support parameterized filters (e.g. copy_min=N
// for arbitrary N) and combined filters via a structured config schema.
func FilterURL(name string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case FilterWishlist:
		return "/giveaways/search?type=wishlist", nil
	case FilterGroup:
		return "/giveaways/search?type=group", nil
	case FilterRecommended:
		return "/giveaways/search?type=recommended", nil
	case FilterNew:
		return "/giveaways/search?type=new", nil
	case FilterDLC:
		return "/giveaways/search?dlc=true", nil
	case FilterMultiCopy:
		return "/giveaways/search?copy_min=2", nil
	case FilterAll, "":
		return "/giveaways/search", nil
	default:
		return "", fmt.Errorf("unknown filter %q", name)
	}
}

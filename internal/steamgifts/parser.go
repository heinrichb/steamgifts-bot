package steamgifts

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ParseListPage extracts the signed-in account state and the list of
// giveaways from a steamgifts.com listing page (e.g. /, /?type=wishlist).
func ParseListPage(html []byte) (AccountState, []Giveaway, error) {
	if isCloudflareChallenge(html) {
		return AccountState{}, nil, errors.New(
			"parse: received a Cloudflare challenge page instead of real content — " +
				"the cookie may have expired or the site is temporarily blocking requests")
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return AccountState{}, nil, fmt.Errorf("parse: %w", err)
	}

	state, err := parseAccountState(doc)
	if err != nil {
		return AccountState{}, nil, err
	}

	giveaways := make([]Giveaway, 0, 32)
	doc.Find(".giveaway__row-inner-wrap").Each(func(_ int, s *goquery.Selection) {
		g, ok := parseGiveawayRow(s)
		if ok {
			giveaways = append(giveaways, g)
		}
	})

	return state, giveaways, nil
}

func parseAccountState(doc *goquery.Document) (AccountState, error) {
	var st AccountState

	// XSRF token lives in a hidden form input on every authenticated page.
	doc.Find(`input[name="xsrf_token"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if v, ok := s.Attr("value"); ok && v != "" {
			st.XSRFToken = v
			return false
		}
		return true
	})
	if st.XSRFToken == "" {
		return st, errors.New("parse: xsrf token missing — cookie likely invalid or expired")
	}

	// Username is in the nav avatar's data-tooltip / link href.
	if href, ok := doc.Find(`a.nav__avatar-outer-wrap`).Attr("href"); ok {
		// /user/<username>
		parts := strings.Split(strings.Trim(href, "/"), "/")
		if len(parts) >= 2 && parts[0] == "user" {
			st.Username = parts[1]
		}
	}

	// Points are shown in the nav as ".nav__points".
	pts := strings.TrimSpace(doc.Find(`.nav__points`).First().Text())
	if pts != "" {
		st.Points = atoiSafe(pts)
	}

	// Account level: <span title="6.38">Level 6</span>, sibling of .nav__points.
	// Scope to the parent of nav__points so we don't accidentally match a
	// giveaway-row span if the site ever changes from <div> to <span>.
	doc.Find(`.nav__points`).First().Parent().Find(`span[title]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if m := levelRe.FindStringSubmatch(s.Text()); len(m) == 2 {
			st.Level = atoiSafe(m[1])
			return false
		}
		return true
	})

	return st, nil
}

var (
	costRe    = regexp.MustCompile(`\((\d+)\s*P\)`)
	copiesRe  = regexp.MustCompile(`(\d+)\s+Copies?`)
	entriesRe = regexp.MustCompile(`(\d[\d,]*)\s+entries?`)
	codeRe    = regexp.MustCompile(`/giveaway/([A-Za-z0-9]+)/`)
	levelRe   = regexp.MustCompile(`Level\s+(\d+)`)
)

func parseGiveawayRow(s *goquery.Selection) (Giveaway, bool) {
	g := Giveaway{}

	heading := s.Find(".giveaway__heading__name").First()
	g.Name = strings.TrimSpace(heading.Text())
	if g.Name == "" {
		return g, false
	}

	if href, ok := heading.Attr("href"); ok {
		g.URL = href
		if m := codeRe.FindStringSubmatch(href); len(m) == 2 {
			g.Code = m[1]
		}
	}
	if g.Code == "" {
		return g, false
	}

	// Cost & copies live in `.giveaway__heading__thin` siblings, e.g.
	//   <span class="giveaway__heading__thin">(50P)</span>
	//   <span class="giveaway__heading__thin">(3 Copies)</span>
	s.Find(".giveaway__heading__thin").Each(func(_ int, t *goquery.Selection) {
		txt := t.Text()
		if m := costRe.FindStringSubmatch(txt); len(m) == 2 {
			g.Cost = atoiSafe(m[1])
		}
		if m := copiesRe.FindStringSubmatch(txt); len(m) == 2 {
			g.Copies = atoiSafe(m[1])
		}
	})
	if g.Copies == 0 {
		g.Copies = 1
	}

	// Entry count.
	s.Find(".giveaway__links a").Each(func(_ int, a *goquery.Selection) {
		if m := entriesRe.FindStringSubmatch(a.Text()); len(m) == 2 {
			g.Entries = atoiSafe(strings.ReplaceAll(m[1], ",", ""))
		}
	})

	// End timestamp — steamgifts puts a `data-timestamp` on the countdown span.
	s.Find(".giveaway__columns span[data-timestamp]").EachWithBreak(func(_ int, t *goquery.Selection) bool {
		if ts, ok := t.Attr("data-timestamp"); ok {
			if i, err := strconv.ParseInt(ts, 10, 64); err == nil {
				g.EndsAt = time.Unix(i, 0)
				return false
			}
		}
		return true
	})

	// Required contributor level, e.g. <div class="giveaway__column--contributor-level ...">Level 2+</div>
	s.Find(`[class*="giveaway__column--contributor-level"]`).EachWithBreak(func(_ int, el *goquery.Selection) bool {
		if m := levelRe.FindStringSubmatch(el.Text()); len(m) == 2 {
			g.Level = atoiSafe(m[1])
			return false
		}
		return true
	})

	// Pinned giveaways live inside `.pinned-giveaways__inner-wrap`.
	if s.Closest(".pinned-giveaways__outer-wrap").Length() > 0 {
		g.Pinned = true
	}

	// Steamgifts puts is-faded on either the outer wrap (more common) or
	// the inner wrap — check both. is-faded means "already entered" or
	// otherwise unjoinable (region-locked, login-required, etc.).
	if s.HasClass("is-faded") || s.HasClass("is-unjoinable") ||
		s.Closest(".giveaway__row-outer-wrap").HasClass("is-faded") {
		g.Unjoinable = true
	}
	if s.Find(`.fa.fa-check`).Length() > 0 {
		g.Entered = true
	}

	return g, true
}

// isCloudflareChallenge detects Cloudflare's "Just a moment..." interstitial.
// When steamgifts is under heavy load or the bot's IP is flagged, Cloudflare
// serves a JS challenge page instead of real content. The page has a
// distinctive title and body structure.
func isCloudflareChallenge(html []byte) bool {
	return bytes.Contains(html, []byte("Just a moment...")) &&
		bytes.Contains(html, []byte("cf-browser-verification"))
}

func atoiSafe(s string) int {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

package steamgifts

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// WonGame represents a game the user has won on steamgifts.
type WonGame struct {
	Name string
	URL  string
	Code string
}

// ParseWinsPage extracts won games from /giveaways/won.
func ParseWinsPage(html []byte) ([]WonGame, error) {
	if isCloudflareChallenge(html) {
		return nil, fmt.Errorf("parse wins: Cloudflare challenge")
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse wins: %w", err)
	}

	var wins []WonGame
	doc.Find(".table__row-inner-wrap").Each(func(_ int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Find(".table__column__heading").First().Text())
		if name == "" {
			return
		}
		w := WonGame{Name: name}
		if href, ok := s.Find(".table__column__heading").Attr("href"); ok {
			w.URL = href
			if m := codeRe.FindStringSubmatch(href); len(m) == 2 {
				w.Code = m[1]
			}
		}
		wins = append(wins, w)
	})
	return wins, nil
}

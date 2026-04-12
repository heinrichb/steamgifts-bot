package steamgifts

import (
	"testing"
)

func TestValidFilterNames(t *testing.T) {
	names := ValidFilterNames()
	if len(names) != 7 {
		t.Errorf("expected 7 filter names, got %d: %v", len(names), names)
	}
	expected := map[string]bool{
		"wishlist": true, "group": true, "recommended": true, "new": true,
		"dlc": true, "multicopy": true, "all": true,
	}
	for _, n := range names {
		if !expected[n] {
			t.Errorf("unexpected filter name: %s", n)
		}
	}
}

func TestIsValidFilter(t *testing.T) {
	if !IsValidFilter("wishlist") {
		t.Error("wishlist should be valid")
	}
	if !IsValidFilter("all") {
		t.Error("all should be valid")
	}
	if !IsValidFilter("/custom/path") {
		t.Error("raw URL path should be valid")
	}
	if IsValidFilter("bogus") {
		t.Error("bogus should not be valid")
	}
}

func TestFilterURLAllFilters(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"wishlist", "/giveaways/search?type=wishlist"},
		{"group", "/giveaways/search?type=group"},
		{"recommended", "/giveaways/search?type=recommended"},
		{"new", "/giveaways/search?type=new"},
		{"dlc", "/giveaways/search?dlc=true"},
		{"multicopy", "/giveaways/search?copy_min=2"},
		{"all", "/giveaways/search"},
		{"", "/giveaways/search"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FilterURL(tc.name)
			if err != nil {
				t.Fatalf("FilterURL(%q): %v", tc.name, err)
			}
			if got != tc.want {
				t.Errorf("FilterURL(%q) = %q; want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestFilterURLRawPath(t *testing.T) {
	got, err := FilterURL("/giveaways/search?type=group&copy_min=3")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/giveaways/search?type=group&copy_min=3" {
		t.Errorf("raw path: %s", got)
	}
}

func TestFilterURLUnknown(t *testing.T) {
	_, err := FilterURL("bogus")
	if err == nil {
		t.Fatal("expected error for unknown filter")
	}
}

func TestEntryResultPointsValueFloat(t *testing.T) {
	r := EntryResult{Points: float64(190)}
	if got := r.PointsValue(); got != 190 {
		t.Errorf("PointsValue float64: %d", got)
	}
}

func TestEntryResultPointsValueString(t *testing.T) {
	r := EntryResult{Points: "398"}
	if got := r.PointsValue(); got != 398 {
		t.Errorf("PointsValue string: %d", got)
	}
}

func TestEntryResultPointsValueNil(t *testing.T) {
	r := EntryResult{Points: nil}
	if got := r.PointsValue(); got != 0 {
		t.Errorf("PointsValue nil: %d", got)
	}
}

func TestEntryResultPointsValueUnexpectedType(t *testing.T) {
	r := EntryResult{Points: true}
	if got := r.PointsValue(); got != 0 {
		t.Errorf("PointsValue bool: %d", got)
	}
}

func TestParseWinsPageEmpty(t *testing.T) {
	html := `<html><body><div class="page__inner-wrap"></div></body></html>`
	wins, err := ParseWinsPage([]byte(html))
	if err != nil {
		t.Fatalf("ParseWinsPage: %v", err)
	}
	if len(wins) != 0 {
		t.Errorf("expected 0 wins, got %d", len(wins))
	}
}

func TestParseWinsPageWithWins(t *testing.T) {
	html := `<html><body>
	<div class="table__row-inner-wrap">
		<a class="table__column__heading" href="/giveaway/XYZ1/cool-game">Cool Game</a>
	</div>
	<div class="table__row-inner-wrap">
		<a class="table__column__heading" href="/giveaway/ABC2/other-game">Other Game</a>
	</div>
	</body></html>`
	wins, err := ParseWinsPage([]byte(html))
	if err != nil {
		t.Fatalf("ParseWinsPage: %v", err)
	}
	if len(wins) != 2 {
		t.Fatalf("expected 2 wins, got %d", len(wins))
	}
	if wins[0].Name != "Cool Game" {
		t.Errorf("win 0 name: %s", wins[0].Name)
	}
	if wins[0].Code != "XYZ1" {
		t.Errorf("win 0 code: %s", wins[0].Code)
	}
	if wins[1].Code != "ABC2" {
		t.Errorf("win 1 code: %s", wins[1].Code)
	}
}

func TestParseWinsPageCloudflare(t *testing.T) {
	html := `<html><head><title>Just a moment...</title></head><body>cf-browser-verification</body></html>`
	_, err := ParseWinsPage([]byte(html))
	if err == nil {
		t.Fatal("expected cloudflare error")
	}
}

func TestIsCloudflareChallenge(t *testing.T) {
	if isCloudflareChallenge([]byte("normal page content")) {
		t.Error("normal page should not be detected as cloudflare")
	}
	cf := []byte("Just a moment... cf-browser-verification something")
	if !isCloudflareChallenge(cf) {
		t.Error("cloudflare page should be detected")
	}
}

func TestIsCaptchaPage(t *testing.T) {
	if isCaptchaPage([]byte("normal page")) {
		t.Error("normal page should not be captcha")
	}
	if !isCaptchaPage([]byte("g-recaptcha")) {
		t.Error("recaptcha should be detected")
	}
	if !isCaptchaPage([]byte("h-captcha")) {
		t.Error("hcaptcha should be detected")
	}
}

func TestAtoiSafe(t *testing.T) {
	if got := atoiSafe("42"); got != 42 {
		t.Errorf("atoiSafe('42') = %d", got)
	}
	if got := atoiSafe("1,234"); got != 1234 {
		t.Errorf("atoiSafe('1,234') = %d", got)
	}
	if got := atoiSafe("  100  "); got != 100 {
		t.Errorf("atoiSafe('  100  ') = %d", got)
	}
	if got := atoiSafe("nope"); got != 0 {
		t.Errorf("atoiSafe('nope') = %d", got)
	}
}

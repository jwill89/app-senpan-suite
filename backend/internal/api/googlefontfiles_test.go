package api

import "testing"

// A family is fetched only when it is on the Google Fonts list, or when there is no
// list to check it against (no key, or the list couldn't be read).
func TestFamilyListed(t *testing.T) {
	list := []string{"Arapey", "Noto Sans JP"}
	for family, want := range map[string]bool{
		"Arapey":       true,
		"Noto Sans JP": true,
		"noto sans jp": false,
		"Arapey ":      false,
		"Not A Font":   false,
	} {
		if got := familyListed(family, list); got != want {
			t.Errorf("familyListed(%q) = %v, want %v", family, got, want)
		}
	}
	if !familyListed("Anything", nil) || !familyListed("Anything", []string{}) {
		t.Error("with no list, a family isn't let through")
	}
}

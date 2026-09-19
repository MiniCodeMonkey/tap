package themes

import "testing"

func TestAll_ContainsBaseFirst(t *testing.T) {
	list := All()
	if len(list) == 0 {
		t.Fatal("All() returned no themes")
	}
	if list[0].Slug != "base" {
		t.Errorf("All()[0].Slug = %q, want %q", list[0].Slug, "base")
	}
}

func TestAll_HasTwentyOneThemes(t *testing.T) {
	list := All()
	if len(list) != 21 {
		t.Errorf("All() returned %d themes, want 21 (base + 20)", len(list))
	}
}

func TestAll_EveryThemeHasRequiredFields(t *testing.T) {
	for _, theme := range All() {
		if theme.Slug == "" {
			t.Errorf("theme has empty slug: %+v", theme)
		}
		if theme.Name == "" {
			t.Errorf("theme %q has empty name", theme.Slug)
		}
		if theme.Polarity != "light" && theme.Polarity != "dark" {
			t.Errorf("theme %q has invalid polarity %q, want light or dark", theme.Slug, theme.Polarity)
		}
		if theme.Pitch == "" {
			t.Errorf("theme %q has empty pitch", theme.Slug)
		}
	}
}

func TestAll_ReturnsACopy(t *testing.T) {
	list := All()
	list[0].Slug = "mutated"

	list2 := All()
	if list2[0].Slug == "mutated" {
		t.Error("All() should return a copy; mutating the result affected a later call")
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		slug string
		want bool
	}{
		{"base", true},
		{"terminal", true},
		{"swiss", true},
		{"transit", true},
		{"unknown", false},
		{"", false},
		{"BASE", false},
	}

	for _, tt := range tests {
		if got := IsValid(tt.slug); got != tt.want {
			t.Errorf("IsValid(%q) = %v, want %v", tt.slug, got, tt.want)
		}
	}
}

func TestSlugs_MatchesAll(t *testing.T) {
	slugs := Slugs()
	all := All()

	if len(slugs) != len(all) {
		t.Fatalf("Slugs() returned %d entries, All() returned %d", len(slugs), len(all))
	}
	for i, theme := range all {
		if slugs[i] != theme.Slug {
			t.Errorf("Slugs()[%d] = %q, want %q", i, slugs[i], theme.Slug)
		}
	}
}

func TestSlugs_NoDuplicates(t *testing.T) {
	seen := make(map[string]bool)
	for _, slug := range Slugs() {
		if seen[slug] {
			t.Errorf("duplicate slug %q in Slugs()", slug)
		}
		seen[slug] = true
	}
}

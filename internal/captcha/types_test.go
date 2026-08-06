package captcha

import "testing"

func TestCatalogEntriesAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, tt := range Catalog {
		if tt.Name == "" {
			t.Fatal("catalog entry with empty Name")
		}
		if seen[tt.Name] {
			t.Errorf("duplicate catalog entry: %s", tt.Name)
		}
		seen[tt.Name] = true
		if tt.Family == "" {
			t.Errorf("%s: missing Family", tt.Name)
		}
		if len(tt.SolutionKeys) == 0 {
			t.Errorf("%s: missing SolutionKeys", tt.Name)
		}
	}
}

func TestByNameFound(t *testing.T) {
	tt, ok := ByName("RecaptchaV2TaskProxyless")
	if !ok {
		t.Fatal("expected RecaptchaV2TaskProxyless to be found")
	}
	if tt.Family != "recaptcha" {
		t.Errorf("Family = %s, want recaptcha", tt.Family)
	}
}

func TestByNameNotFound(t *testing.T) {
	if _, ok := ByName("NotARealType"); ok {
		t.Fatal("expected NotARealType to be not found")
	}
}

func TestByFamilyFiltersCaseInsensitive(t *testing.T) {
	entries := ByFamily("RECAPTCHA")
	if len(entries) == 0 {
		t.Fatal("expected at least one recaptcha entry")
	}
	for _, e := range entries {
		if e.Family != "recaptcha" {
			t.Errorf("got family %s in recaptcha filter", e.Family)
		}
	}
}

func TestByFamilyEmptyReturnsAll(t *testing.T) {
	if len(ByFamily("")) != len(Catalog) {
		t.Errorf("ByFamily(\"\") = %d entries, want %d", len(ByFamily("")), len(Catalog))
	}
}

func TestMissingRequired(t *testing.T) {
	tt, _ := ByName("RecaptchaV2TaskProxyless")
	missing := MissingRequired(tt, map[string]any{"websiteURL": "https://example.com"})
	if len(missing) != 1 || missing[0] != "websiteKey" {
		t.Errorf("missing = %v, want [websiteKey]", missing)
	}
	if len(MissingRequired(tt, map[string]any{"websiteURL": "x", "websiteKey": "y"})) != 0 {
		t.Error("expected no missing fields when all required fields present")
	}
}

func TestSuggestNames(t *testing.T) {
	suggestions := SuggestNames("turnstile")
	if len(suggestions) == 0 {
		t.Error("expected suggestions for 'turnstile'")
	}
}

func TestTokenExtractsFirstMatchingKey(t *testing.T) {
	tt, _ := ByName("RecaptchaV2TaskProxyless")
	got := Token(tt, map[string]any{"token": "fallback", "gRecaptchaResponse": "primary"})
	if got != "primary" {
		t.Errorf("Token() = %s, want primary (first SolutionKeys match)", got)
	}
	if Token(tt, map[string]any{}) != "" {
		t.Error("Token() on empty solution should be empty string")
	}
}

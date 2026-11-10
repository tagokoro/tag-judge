package domain

import "testing"

// TestValidateSubmissionURL is the authoritative gate for self-declared
// submission link URLs. It must accept only absolute HTTP(S) URLs with a real
// host (no control/whitespace, at most 2048 bytes), trim surrounding spaces,
// and reject the hostless authorities the DB CHECK previously let through.
func TestValidateSubmissionURL(t *testing.T) {
	invalid := []struct{ name, url string }{
		{"hostless query", "https://?x"},
		{"hostless at", "https://@"},
		{"hostless port", "https://:443"},
		{"scheme only", "https://"},
		{"no scheme", "not-a-url"},
		{"wrong scheme", "ftp://example.com/x"},
		{"opaque", "https:example.com"},
		{"userinfo no host", "https://user@"},
		{"leading dot host", "https://.example.com"},
		{"control char", "https://example.com/\x07"},
		{"internal space", "https://exa mple.com"},
		{"empty", ""},
		{"only spaces", "   "},
		{"too long", "https://e.com/" + repeat('a', 2049-len("https://e.com/"))},
	}
	for _, tc := range invalid {
		if _, err := ValidateSubmissionURL(tc.url); err == nil {
			t.Errorf("ValidateSubmissionURL(%q) [%s] = nil error, want rejection", tc.url, tc.name)
		}
	}

	valid := []struct{ name, url string }{
		{"simple", "https://example.com"},
		{"localhost port query", "http://localhost:3000/submit?q=1"},
		{"ipv4", "http://127.0.0.1:8080/x"},
		{"ipv6", "http://[::1]:8080/path"},
		{"query no path", "https://example.com?q=1"},
		{"fragment", "https://example.com#frag"},
		{"port path query fragment", "https://example.com:8443/x?y=2#z"},
		{"max length 2048", "https://e.com/" + repeat('a', 2048-len("https://e.com/"))},
		{"padded", "  https://example.com/x  "},
	}
	for _, tc := range valid {
		got, err := ValidateSubmissionURL(tc.url)
		if err != nil {
			t.Errorf("ValidateSubmissionURL(%q) [%s] returned error: %v", tc.url, tc.name, err)
			continue
		}
		want := tc.url
		if tc.name == "padded" {
			want = "https://example.com/x"
		}
		if got != want {
			t.Errorf("ValidateSubmissionURL(%q) = %q, want %q", tc.url, got, want)
		}
	}
}

func repeat(b byte, n int) string {
	s := make([]byte, n)
	for i := range s {
		s[i] = b
	}
	return string(s)
}

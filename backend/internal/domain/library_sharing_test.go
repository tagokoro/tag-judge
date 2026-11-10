package domain

import "testing"

// TestValidateSubmissionURL is the authoritative gate for self-declared
// submission link URLs. It must accept ANY absolute HTTP(S) URL with a real
// host (including internationalized/Unicode hosts, no control/whitespace, at
// most 2048 bytes), trim surrounding spaces, and reject all no-host forms.
func TestValidateSubmissionURL(t *testing.T) {
	invalid := []struct{ name, url string }{
		{"hostless query", "https://?x"},
		{"hostless at", "https://@"},
		{"hostless port", "https://:443"},
		{"hostless fragment", "https://#frag"},
		{"hostless path", "https:///path"},
		{"scheme only", "https://"},
		{"no scheme", "not-a-url"},
		{"wrong scheme", "ftp://example.com/x"},
		{"opaque", "https:example.com"},
		{"userinfo no host", "https://user@"},
		{"userinfo password no host", "https://user:pass@"},
		{"userinfo no host trailing slash", "https://user@/"},
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
		{"userinfo with host", "https://user@example.com/sub"},
		{"unicode host", "https://例え.jp/submissions/1"},
		{"unicode host with query", "https://例え.jp?q=1"},
		{"unicode host with port", "http://日本.jp:8080/x"},
		{"punycode idn host", "https://xn--r8jz45g.jp/sub/1"},
		{"max length 2048", "https://e.com/" + repeat('a', 2048-len("https://e.com/"))},
		{"padded", "  https://example.com/x  "},
		{"uppercase scheme", "HTTPS://Example.com/X"},
		{"mixed case scheme", "Http://example.com"},
	}
	normalized := map[string]string{
		"padded":            "https://example.com/x",
		"uppercase scheme":  "https://Example.com/X",
		"mixed case scheme": "http://example.com",
	}
	for _, tc := range valid {
		got, err := ValidateSubmissionURL(tc.url)
		if err != nil {
			t.Errorf("ValidateSubmissionURL(%q) [%s] returned error: %v", tc.url, tc.name, err)
			continue
		}
		want := tc.url
		if n, ok := normalized[tc.name]; ok {
			want = n
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

package domain

import (
	"strings"
	"testing"
)

func TestValidHandle(t *testing.T) {
	for _, handle := range []string{"ab", "seed-author", strings.Repeat("a", 16), "edit"} {
		if !ValidHandle(handle) {
			t.Fatalf("ValidHandle(%q) = false, want valid", handle)
		}
	}
	for _, handle := range []string{"a", "A1", "-ab", "a--b", "a_b", strings.Repeat("a", 17), ""} {
		if ValidHandle(handle) {
			t.Fatalf("ValidHandle(%q) = true, want invalid", handle)
		}
	}
}

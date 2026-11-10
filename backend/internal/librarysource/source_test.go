package librarysource_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/tagokoro/tag-judge/backend/internal/librarysource"
)

func TestContentSHA256(t *testing.T) {
	library := "def add(a, b):\n    return a + b\n\nif __name__ == \"__main__\":\n    print(\"library-main\")\n"
	if librarysource.ContentSHA256(library) != "b24916acf9a8670a4e088af2bbc2591d02eca1f33ddad1aec0ed3a41d57568af" {
		t.Fatalf("library hash = %s", librarysource.ContentSHA256(library))
	}
}

func TestValidateSourceBoundaries(t *testing.T) {
	if !errors.Is(librarysource.ValidateSource(" \n"), librarysource.ErrEmptySource) {
		t.Fatal("blank source must be rejected")
	}
	if !errors.Is(librarysource.ValidateSource("a\x00b"), librarysource.ErrNULSource) {
		t.Fatal("NUL source must be rejected")
	}
	if !errors.Is(librarysource.ValidateSource(strings.Repeat("a", librarysource.MaxSourceBytes+1)), librarysource.ErrSourceTooLarge) {
		t.Fatal("oversized source must be rejected")
	}
	if err := librarysource.ValidateSource(strings.Repeat("a", librarysource.MaxSourceBytes)); err != nil {
		t.Fatalf("max-size source must be accepted: %v", err)
	}
	if err := librarysource.ValidateSource("print(1)"); err != nil {
		t.Fatalf("valid source must be accepted: %v", err)
	}
}

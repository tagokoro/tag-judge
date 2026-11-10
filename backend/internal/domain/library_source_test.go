package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
)

func TestLibrarySourceSHA256(t *testing.T) {
	library := "def add(a, b):\n    return a + b\n"
	if domain.LibrarySourceSHA256(library) != "ba1a531f581d2e6094e978ed6f7aca7a8d92eeb62c6e7ad73ee692f7f18bc772" {
		t.Fatalf("library hash = %s", domain.LibrarySourceSHA256(library))
	}
}

func TestValidateLibrarySourceBoundaries(t *testing.T) {
	if !errors.Is(domain.ValidateLibrarySource(" \n"), domain.ErrEmptyLibrarySource) {
		t.Fatal("blank source must be rejected")
	}
	if !errors.Is(domain.ValidateLibrarySource("a\x00b"), domain.ErrNULLibrarySource) {
		t.Fatal("NUL source must be rejected")
	}
	if !errors.Is(domain.ValidateLibrarySource(strings.Repeat("a", domain.MaxLibrarySourceBytes+1)), domain.ErrLibrarySourceTooLarge) {
		t.Fatal("oversized source must be rejected")
	}
	if err := domain.ValidateLibrarySource(strings.Repeat("a", domain.MaxLibrarySourceBytes)); err != nil {
		t.Fatalf("max-size source must be accepted: %v", err)
	}
	if err := domain.ValidateLibrarySource("print(1)"); err != nil {
		t.Fatalf("valid source must be accepted: %v", err)
	}
}

package librarycombine_test

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/tagokoro/tag-judge/backend/internal/librarycombine"
)

func TestCombineRunsAdapterWithoutLibraryMain(t *testing.T) {
	library := "def add(a, b):\n    return a + b\n\nif __name__ == \"__main__\":\n    print(\"library-main\")\n"
	adapter := "a, b = map(int, input().split())\nprint(add(a, b))\n"
	if librarycombine.ContentSHA256(library) != "b24916acf9a8670a4e088af2bbc2591d02eca1f33ddad1aec0ed3a41d57568af" {
		t.Fatalf("library hash = %s", librarycombine.ContentSHA256(library))
	}
	if librarycombine.ContentSHA256(adapter) != "b8d685a728f43ee0c53f5f299234c42d5ebbe0f423bb5b18d16a7ecaf2c03f21" {
		t.Fatalf("adapter hash = %s", librarycombine.ContentSHA256(adapter))
	}
	got, err := librarycombine.Combine(library, adapter)
	if err != nil {
		t.Fatalf("Combine: %v", err)
	}
	want := "import base64\n_library = base64.b64decode('ZGVmIGFkZChhLCBiKToKICAgIHJldHVybiBhICsgYgoKaWYgX19uYW1lX18gPT0gIl9fbWFpbl9fIjoKICAgIHByaW50KCJsaWJyYXJ5LW1haW4iKQo=').decode(\"utf-8\")\n_adapter = base64.b64decode('YSwgYiA9IG1hcChpbnQsIGlucHV0KCkuc3BsaXQoKSkKcHJpbnQoYWRkKGEsIGIpKQo=').decode(\"utf-8\")\n_namespace = {\"__name__\": \"library\"}\nexec(compile(_library, \"<library>\", \"exec\"), _namespace)\n_namespace[\"__name__\"] = \"__main__\"\nexec(compile(_adapter, \"<adapter>\", \"exec\"), _namespace)\n"
	if got != want {
		t.Fatalf("combined source mismatch:\n%s", got)
	}
	if len(got) == 0 || got[len(got)-1] != '\n' {
		t.Fatalf("combined source must end with a single newline: %q", got)
	}
	cmd := exec.Command("python3", "-c", got)
	cmd.Stdin = strings.NewReader("1 2\n")
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "3\n" {
		t.Fatalf("python output = %q, err = %v", out, err)
	}
}

func TestValidateSourceBoundaries(t *testing.T) {
	if !errors.Is(librarycombine.ValidateSource(" \n"), librarycombine.ErrEmptySource) {
		t.Fatal("blank source must be empty")
	}
	if !errors.Is(librarycombine.ValidateSource("a\x00b"), librarycombine.ErrNULSource) {
		t.Fatal("NUL must be rejected")
	}
	if !errors.Is(librarycombine.ValidateSource(strings.Repeat("a", librarycombine.MaxSourceBytes+1)), librarycombine.ErrSourceTooLarge) {
		t.Fatal("100001 bytes must be rejected")
	}
	if err := librarycombine.ValidateSource(strings.Repeat("a", librarycombine.MaxSourceBytes)); err != nil {
		t.Fatalf("100000 bytes must pass: %v", err)
	}
}

func TestCombineRejectsInvalidSources(t *testing.T) {
	library := "def add(a, b):\n    return a + b\n"
	adapter := "print(add(1, 2))\n"
	if _, err := librarycombine.Combine(library, " \t"); !errors.Is(err, librarycombine.ErrEmptySource) {
		t.Fatalf("adapter = %v", err)
	}
	if _, err := librarycombine.Combine("", adapter); !errors.Is(err, librarycombine.ErrEmptySource) {
		t.Fatalf("library = %v", err)
	}
	if _, err := librarycombine.Combine(library, strings.Repeat("x", librarycombine.MaxSourceBytes+1)); !errors.Is(err, librarycombine.ErrSourceTooLarge) {
		t.Fatalf("oversized adapter = %v", err)
	}
	if _, err := librarycombine.Combine("a\x00b", adapter); !errors.Is(err, librarycombine.ErrNULSource) {
		t.Fatalf("NUL library = %v", err)
	}
}

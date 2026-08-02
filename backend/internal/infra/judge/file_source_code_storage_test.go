package judgeinfra

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

func TestFileSourceCodeStorageWritesUnderRoot(t *testing.T) {
	root := t.TempDir()
	storage := NewFileSourceCodeStorage(root)

	err := storage.PutSourceCode(context.Background(), repository.PutSourceCodeParams{
		Key:        "submissions/user/problems/42/main.py",
		Language:   domain.SubmissionLanguagePython,
		SourceCode: "print(42)\n",
	})
	if err != nil {
		t.Fatalf("PutSourceCode returned error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(root, "submissions/user/problems/42/main.py"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(got) != "print(42)\n" {
		t.Fatalf("source code = %q", got)
	}
}

func TestFileSourceCodeStorageRejectsEscapingKey(t *testing.T) {
	root := t.TempDir()
	storage := NewFileSourceCodeStorage(root)

	for _, key := range []string{"../main.py", "/tmp/main.py", ""} {
		err := storage.PutSourceCode(context.Background(), repository.PutSourceCodeParams{
			Key:        key,
			Language:   domain.SubmissionLanguagePython,
			SourceCode: "print(42)\n",
		})
		if err == nil {
			t.Fatalf("PutSourceCode(%q) returned nil, want error", key)
		}
	}
}

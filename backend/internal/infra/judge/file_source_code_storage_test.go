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

func TestFileStorageWritesTestCases(t *testing.T) {
	root := t.TempDir()
	storage := NewFileSourceCodeStorage(root)
	params := repository.PutTestCasesParams{
		ProblemID: 42,
		TestCases: []domain.TestCase{
			{Input: "1 2\n", ExpectedOutput: "3\n", IsSample: true, Score: 1},
			{Input: "2 3\n", ExpectedOutput: "5\n", Score: 1},
		},
	}

	key, err := storage.PutTestCases(context.Background(), params)
	if err != nil {
		t.Fatalf("PutTestCases returned error: %v", err)
	}
	if filepath.Dir(key) != "problems/42/testcases" || filepath.Ext(key) != ".json" {
		t.Fatalf("key = %q", key)
	}

	got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(key)))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	want := `[{"input":"1 2\n","expectedOutput":"3\n"},{"input":"2 3\n","expectedOutput":"5\n"}]`
	if string(got) != want {
		t.Fatalf("test cases = %s, want %s", got, want)
	}

	sameKey, err := storage.PutTestCases(context.Background(), params)
	if err != nil {
		t.Fatalf("second PutTestCases returned error: %v", err)
	}
	if sameKey != key {
		t.Fatalf("same artifact key = %q, want %q", sameKey, key)
	}

	params.TestCases[1].ExpectedOutput = "6\n"
	changedKey, err := storage.PutTestCases(context.Background(), params)
	if err != nil {
		t.Fatalf("changed PutTestCases returned error: %v", err)
	}
	if changedKey == key {
		t.Fatal("changed test cases should use a different artifact key")
	}
}

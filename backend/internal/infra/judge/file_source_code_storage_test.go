package judgeinfra

import (
	"context"
	"encoding/json"
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

func TestFileSourceCodeStorageKeepsContentAddressedTestCaseSnapshots(t *testing.T) {
	root := t.TempDir()
	storage := NewFileSourceCodeStorage(root)
	first := repository.PutTestCasesParams{ProblemID: 42, TestCases: []domain.TestCase{{
		ID: 1, ProblemID: 42, Input: "1 2\\n", ExpectedOutput: "3\\n", Score: 1,
	}}}
	firstKey, err := storage.PutTestCases(context.Background(), first)
	if err != nil {
		t.Fatalf("PutTestCases(first) returned error: %v", err)
	}
	firstBody, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(firstKey)))
	if err != nil {
		t.Fatalf("ReadFile(first) returned error: %v", err)
	}

	// This models an author editing the database case after a submission.
	second := first
	second.TestCases = append([]domain.TestCase(nil), first.TestCases...)
	second.TestCases[0].ExpectedOutput = "4\\n"
	secondKey, err := storage.PutTestCases(context.Background(), second)
	if err != nil {
		t.Fatalf("PutTestCases(second) returned error: %v", err)
	}
	if firstKey == secondKey {
		t.Fatalf("snapshot key = %q after changed test case; want a new content-addressed key", secondKey)
	}
	gotFirstBody, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(firstKey)))
	if err != nil {
		t.Fatalf("ReadFile(first after edit) returned error: %v", err)
	}
	if string(gotFirstBody) != string(firstBody) {
		t.Fatalf("first snapshot was overwritten: got %s, want %s", gotFirstBody, firstBody)
	}

	var workerCases []struct {
		Input          string `json:"input"`
		ExpectedOutput string `json:"expectedOutput"`
	}
	if err := json.Unmarshal(firstBody, &workerCases); err != nil {
		t.Fatalf("worker-compatible JSON failed to decode: %v", err)
	}
	if len(workerCases) != 1 || workerCases[0].Input != "1 2\\n" || workerCases[0].ExpectedOutput != "3\\n" {
		t.Fatalf("worker cases = %+v", workerCases)
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

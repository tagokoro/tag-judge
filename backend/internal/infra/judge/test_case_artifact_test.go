package judgeinfra

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

func TestMarshalTestCasesIsDeterministicAndWorkerCompatible(t *testing.T) {
	params := repository.PutTestCasesParams{ProblemID: 42, TestCases: []domain.TestCase{
		{ID: 1, Score: 2, Input: "1 2\\n", ExpectedOutput: "3\\n"},
		{ID: 2, Score: 3, Input: "4 5\\n", ExpectedOutput: "9\\n"},
	}}
	firstBody, firstKey, err := marshalTestCases(params)
	if err != nil {
		t.Fatalf("marshalTestCases returned error: %v", err)
	}
	secondBody, secondKey, err := marshalTestCases(params)
	if err != nil {
		t.Fatalf("marshalTestCases returned error: %v", err)
	}
	if string(firstBody) != string(secondBody) || firstKey != secondKey {
		t.Fatalf("snapshot is not deterministic: %q / %q", firstKey, secondKey)
	}
	if !strings.HasPrefix(firstKey, "problems/42/testcases/") || !strings.HasSuffix(firstKey, ".json") {
		t.Fatalf("key = %q", firstKey)
	}
	var workerCases []struct {
		Input          string `json:"input"`
		ExpectedOutput string `json:"expectedOutput"`
	}
	if err := json.Unmarshal(firstBody, &workerCases); err != nil {
		t.Fatalf("worker-compatible JSON failed to decode: %v", err)
	}
	if len(workerCases) != 2 || workerCases[1].ExpectedOutput != "9\\n" {
		t.Fatalf("worker cases = %+v", workerCases)
	}
}

func TestMarshalTestCasesChangesKeyWhenTestCaseChanges(t *testing.T) {
	params := repository.PutTestCasesParams{ProblemID: 42, TestCases: []domain.TestCase{{
		ID: 1, Score: 1, Input: "1\\n", ExpectedOutput: "1\\n",
	}}}
	_, before, err := marshalTestCases(params)
	if err != nil {
		t.Fatalf("marshal before returned error: %v", err)
	}
	params.TestCases[0].ExpectedOutput = "2\\n"
	_, after, err := marshalTestCases(params)
	if err != nil {
		t.Fatalf("marshal after returned error: %v", err)
	}
	if before == after {
		t.Fatalf("key did not change after a test case edit: %q", after)
	}
}

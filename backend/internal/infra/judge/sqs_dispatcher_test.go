package judgeinfra

import (
	"encoding/json"
	"testing"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

func TestMarshalJudgeJobUsesWorkerJSONShape(t *testing.T) {
	body, err := marshalJudgeJob(repository.JudgeJob{
		SubmissionID:  "123",
		Language:      domain.SubmissionLanguagePython,
		SourceCodeKey: "submissions/user/problems/42/main.py",
		TestCasesKey:  "problems/42/testcases.json",
		TimeLimitMS:   1000,
	})
	if err != nil {
		t.Fatalf("marshalJudgeJob returned error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}

	want := map[string]any{
		"submissionId":  "123",
		"language":      "python",
		"sourceCodeKey": "submissions/user/problems/42/main.py",
		"testCasesKey":  "problems/42/testcases.json",
		"timeLimitMs":   float64(1000),
	}
	for key, wantValue := range want {
		if got[key] != wantValue {
			t.Fatalf("%s = %v, want %v; body=%s", key, got[key], wantValue, body)
		}
	}
}

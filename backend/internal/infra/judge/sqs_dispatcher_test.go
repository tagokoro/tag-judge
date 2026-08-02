package judgeinfra

import (
	"encoding/json"
	"testing"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

func TestMarshalExecutionJobUsesWorkerJSONShape(t *testing.T) {
	job := newProblemExecutionJobForTest(t)

	body, err := marshalExecutionJob(job)
	if err != nil {
		t.Fatalf("marshalExecutionJob returned error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}

	want := map[string]any{
		"executionMode": "problem",
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
	reference, ok := got["reference"].(map[string]any)
	if !ok {
		t.Fatalf("reference is missing or invalid: %v", got["reference"])
	}
	if reference["kind"] != "submission" || reference["id"] != "123" {
		t.Fatalf("reference = %+v, want submission 123", reference)
	}
	problemExecution, ok := got["problemExecution"].(map[string]any)
	if !ok {
		t.Fatalf("problemExecution is missing or invalid: %v", got["problemExecution"])
	}
	if problemExecution["testCasesKey"] != "problems/42/testcases.json" || problemExecution["timeLimitMs"] != float64(1000) {
		t.Fatalf("problemExecution = %+v", problemExecution)
	}
}

func TestMarshalExecutionJobIsStableForResend(t *testing.T) {
	job := newProblemExecutionJobForTest(t)

	first, err := marshalExecutionJob(job)
	if err != nil {
		t.Fatalf("marshalExecutionJob returned error: %v", err)
	}
	second, err := marshalExecutionJob(job)
	if err != nil {
		t.Fatalf("marshalExecutionJob returned error: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("payload changed between resend attempts:\nfirst=%s\nsecond=%s", first, second)
	}
}

func TestMarshalExecutionJobSupportsStdinMode(t *testing.T) {
	reference, err := repository.NewCodeRunExecutionReference("run-123")
	if err != nil {
		t.Fatalf("NewCodeRunExecutionReference returned error: %v", err)
	}
	mode, err := repository.NewStdinExecution("1 2\n", 500)
	if err != nil {
		t.Fatalf("NewStdinExecution returned error: %v", err)
	}
	job, err := repository.NewExecutionJob(reference, domain.ExecutionLanguagePython, "code-runs/run-123/main.py", mode)
	if err != nil {
		t.Fatalf("NewExecutionJob returned error: %v", err)
	}

	body, err := marshalExecutionJob(job)
	if err != nil {
		t.Fatalf("marshalExecutionJob returned error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if got["executionMode"] != "stdin" || got["codeRunId"] != "run-123" || got["stdin"] != "1 2\n" || got["timeLimitMs"] != float64(500) {
		t.Fatalf("stdin payload = %+v", got)
	}
	if _, exists := got["testCasesKey"]; exists {
		t.Fatalf("stdin payload should not include testCasesKey: %s", body)
	}
}

func newProblemExecutionJobForTest(t *testing.T) repository.ExecutionJob {
	t.Helper()

	reference, err := repository.NewSubmissionExecutionReference("123")
	if err != nil {
		t.Fatalf("NewSubmissionExecutionReference returned error: %v", err)
	}
	mode, err := repository.NewProblemExecution("problems/42/testcases.json", 1000)
	if err != nil {
		t.Fatalf("NewProblemExecution returned error: %v", err)
	}
	job, err := repository.NewExecutionJob(
		reference,
		domain.ExecutionLanguagePython,
		"submissions/user/problems/42/main.py",
		mode,
	)
	if err != nil {
		t.Fatalf("NewExecutionJob returned error: %v", err)
	}
	return job
}

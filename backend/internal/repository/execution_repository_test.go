package repository

import (
	"testing"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
)

func TestNewExecutionJobAcceptsProblemSubmission(t *testing.T) {
	reference, err := NewSubmissionExecutionReference("123")
	if err != nil {
		t.Fatalf("NewSubmissionExecutionReference returned error: %v", err)
	}
	mode, err := NewProblemExecution("problems/42/testcases.json", 1000)
	if err != nil {
		t.Fatalf("NewProblemExecution returned error: %v", err)
	}

	job, err := NewExecutionJob(reference, domain.ExecutionLanguagePython, "submissions/123/main.py", mode)
	if err != nil {
		t.Fatalf("NewExecutionJob returned error: %v", err)
	}

	if job.Reference().Kind() != ExecutionReferenceSubmission || job.Mode().Kind() != ExecutionModeProblem {
		t.Fatalf("job = %+v, want submission problem execution", job)
	}
}

func TestNewExecutionJobAcceptsStdinCodeRun(t *testing.T) {
	reference, err := NewCodeRunExecutionReference("run-123")
	if err != nil {
		t.Fatalf("NewCodeRunExecutionReference returned error: %v", err)
	}
	mode, err := NewStdinExecution("", 1000)
	if err != nil {
		t.Fatalf("NewStdinExecution returned error: %v", err)
	}

	job, err := NewExecutionJob(reference, domain.ExecutionLanguagePython, "code-runs/run-123/main.py", mode)
	if err != nil {
		t.Fatalf("NewExecutionJob returned error: %v", err)
	}

	if job.Reference().Kind() != ExecutionReferenceCodeRun || job.Mode().Kind() != ExecutionModeStdin {
		t.Fatalf("job = %+v, want code_run stdin execution", job)
	}
}

func TestNewExecutionJobRejectsInvalidModeReferenceCombinations(t *testing.T) {
	submissionReference, err := NewSubmissionExecutionReference("123")
	if err != nil {
		t.Fatalf("NewSubmissionExecutionReference returned error: %v", err)
	}
	codeRunReference, err := NewCodeRunExecutionReference("run-123")
	if err != nil {
		t.Fatalf("NewCodeRunExecutionReference returned error: %v", err)
	}
	problemMode, err := NewProblemExecution("problems/42/testcases.json", 1000)
	if err != nil {
		t.Fatalf("NewProblemExecution returned error: %v", err)
	}
	stdinMode, err := NewStdinExecution("input\n", 1000)
	if err != nil {
		t.Fatalf("NewStdinExecution returned error: %v", err)
	}

	if _, err := NewExecutionJob(codeRunReference, domain.ExecutionLanguagePython, "code-runs/run-123/main.py", problemMode); err == nil {
		t.Fatal("NewExecutionJob accepted problem execution with code_run reference")
	}
	if _, err := NewExecutionJob(submissionReference, domain.ExecutionLanguagePython, "submissions/123/main.py", stdinMode); err == nil {
		t.Fatal("NewExecutionJob accepted stdin execution with submission reference")
	}
}

func TestExecutionConstructorsRejectMissingRequiredFields(t *testing.T) {
	if _, err := NewSubmissionExecutionReference(" "); err == nil {
		t.Fatal("NewSubmissionExecutionReference accepted empty id")
	}
	if _, err := NewProblemExecution(" ", 1000); err == nil {
		t.Fatal("NewProblemExecution accepted empty test cases key")
	}
	if _, err := NewProblemExecution("problems/42/testcases.json", 0); err == nil {
		t.Fatal("NewProblemExecution accepted zero time limit")
	}
	if _, err := NewStdinExecution("", 0); err == nil {
		t.Fatal("NewStdinExecution accepted zero time limit")
	}
}

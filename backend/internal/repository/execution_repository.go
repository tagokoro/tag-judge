package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
)

type PutSourceCodeParams struct {
	Key        string
	Language   domain.ExecutionLanguage
	SourceCode string
}

type SourceCodeStorage interface {
	PutSourceCode(ctx context.Context, params PutSourceCodeParams) error
}

type ExecutionReferenceKind string

const (
	ExecutionReferenceSubmission ExecutionReferenceKind = "submission"
	ExecutionReferenceCodeRun    ExecutionReferenceKind = "code_run"
)

type ExecutionReference struct {
	kind ExecutionReferenceKind
	id   string
}

func NewSubmissionExecutionReference(id string) (ExecutionReference, error) {
	return newExecutionReference(ExecutionReferenceSubmission, id)
}

func NewCodeRunExecutionReference(id string) (ExecutionReference, error) {
	return newExecutionReference(ExecutionReferenceCodeRun, id)
}

func newExecutionReference(kind ExecutionReferenceKind, id string) (ExecutionReference, error) {
	reference := ExecutionReference{kind: kind, id: id}
	if err := reference.Validate(); err != nil {
		return ExecutionReference{}, err
	}
	return reference, nil
}

func (r ExecutionReference) Kind() ExecutionReferenceKind {
	return r.kind
}

func (r ExecutionReference) ID() string {
	return r.id
}

func (r ExecutionReference) Validate() error {
	switch r.kind {
	case ExecutionReferenceSubmission, ExecutionReferenceCodeRun:
	default:
		return fmt.Errorf("execution reference kind must be submission or code_run")
	}
	if strings.TrimSpace(r.id) == "" {
		return fmt.Errorf("execution reference id must not be empty")
	}
	return nil
}

type ExecutionModeKind string

const (
	ExecutionModeProblem ExecutionModeKind = "problem"
	ExecutionModeStdin   ExecutionModeKind = "stdin"
)

type ProblemExecution struct {
	TestCasesKey string
	TimeLimitMS  int
}

type StdinExecution struct {
	Stdin       string
	TimeLimitMS int
}

type ExecutionMode struct {
	kind    ExecutionModeKind
	problem ProblemExecution
	stdin   StdinExecution
}

func NewProblemExecution(testCasesKey string, timeLimitMS int) (ExecutionMode, error) {
	mode := ExecutionMode{
		kind: ExecutionModeProblem,
		problem: ProblemExecution{
			TestCasesKey: testCasesKey,
			TimeLimitMS:  timeLimitMS,
		},
	}
	if err := mode.Validate(); err != nil {
		return ExecutionMode{}, err
	}
	return mode, nil
}

func NewStdinExecution(stdin string, timeLimitMS int) (ExecutionMode, error) {
	mode := ExecutionMode{
		kind: ExecutionModeStdin,
		stdin: StdinExecution{
			Stdin:       stdin,
			TimeLimitMS: timeLimitMS,
		},
	}
	if err := mode.Validate(); err != nil {
		return ExecutionMode{}, err
	}
	return mode, nil
}

func (m ExecutionMode) Kind() ExecutionModeKind {
	return m.kind
}

func (m ExecutionMode) Problem() (ProblemExecution, bool) {
	return m.problem, m.kind == ExecutionModeProblem
}

func (m ExecutionMode) Stdin() (StdinExecution, bool) {
	return m.stdin, m.kind == ExecutionModeStdin
}

func (m ExecutionMode) Validate() error {
	switch m.kind {
	case ExecutionModeProblem:
		if strings.TrimSpace(m.problem.TestCasesKey) == "" {
			return fmt.Errorf("problem execution test cases key must not be empty")
		}
		if m.problem.TimeLimitMS <= 0 {
			return fmt.Errorf("problem execution time limit must be positive")
		}
	case ExecutionModeStdin:
		if m.stdin.TimeLimitMS <= 0 {
			return fmt.Errorf("stdin execution time limit must be positive")
		}
	default:
		return fmt.Errorf("execution mode must be problem or stdin")
	}
	return nil
}

type ExecutionJob struct {
	reference     ExecutionReference
	language      domain.ExecutionLanguage
	sourceCodeKey string
	mode          ExecutionMode
}

func NewExecutionJob(reference ExecutionReference, language domain.ExecutionLanguage, sourceCodeKey string, mode ExecutionMode) (ExecutionJob, error) {
	job := ExecutionJob{
		reference:     reference,
		language:      language,
		sourceCodeKey: sourceCodeKey,
		mode:          mode,
	}
	if err := job.Validate(); err != nil {
		return ExecutionJob{}, err
	}
	return job, nil
}

func (j ExecutionJob) Reference() ExecutionReference {
	return j.reference
}

func (j ExecutionJob) Language() domain.ExecutionLanguage {
	return j.language
}

func (j ExecutionJob) SourceCodeKey() string {
	return j.sourceCodeKey
}

func (j ExecutionJob) Mode() ExecutionMode {
	return j.mode
}

func (j ExecutionJob) Validate() error {
	if err := j.reference.Validate(); err != nil {
		return err
	}
	if !j.language.Valid() {
		return fmt.Errorf("execution language must be supported")
	}
	if strings.TrimSpace(j.sourceCodeKey) == "" {
		return fmt.Errorf("execution source code key must not be empty")
	}
	if err := j.mode.Validate(); err != nil {
		return err
	}
	switch j.mode.Kind() {
	case ExecutionModeProblem:
		if j.reference.Kind() != ExecutionReferenceSubmission {
			return fmt.Errorf("problem execution requires a submission reference")
		}
	case ExecutionModeStdin:
		if j.reference.Kind() != ExecutionReferenceCodeRun {
			return fmt.Errorf("stdin execution requires a code_run reference")
		}
	}
	return nil
}

type ExecutionDispatcher interface {
	Dispatch(ctx context.Context, job ExecutionJob) error
}

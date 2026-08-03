package domain

import "time"

type ProblemVisibility string

const (
	ProblemVisibilityPublic  ProblemVisibility = "public"
	ProblemVisibilityPrivate ProblemVisibility = "private"
	ProblemVisibilityGroup   ProblemVisibility = "group"
)

type Problem struct {
	ID            int64
	Title         string
	Statement     string
	TimeLimitMS   int
	MemoryLimitKB int
	Visibility    ProblemVisibility
	CreatedBy     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (v ProblemVisibility) Valid() bool {
	switch v {
	case ProblemVisibilityPublic, ProblemVisibilityPrivate, ProblemVisibilityGroup:
		return true
	default:
		return false
	}
}

type TestCase struct {
	ID             int64
	ProblemID      int64
	Input          string
	ExpectedOutput string
	IsSample       bool
	Score          int
	CreatedAt      time.Time
}

type SubmissionStatus string

const (
	SubmissionStatusPending SubmissionStatus = "pending"
	SubmissionStatusRunning SubmissionStatus = "running"
	SubmissionStatusAC      SubmissionStatus = "ac"
	SubmissionStatusWA      SubmissionStatus = "wa"
	SubmissionStatusTLE     SubmissionStatus = "tle"
	SubmissionStatusMLE     SubmissionStatus = "mle"
	SubmissionStatusRE      SubmissionStatus = "re"
	SubmissionStatusCE      SubmissionStatus = "ce"
	SubmissionStatusIE      SubmissionStatus = "ie"
)

type Submission struct {
	ID         int64
	UserID     string
	ProblemID  int64
	Language   string
	SourceCode string
	Status     SubmissionStatus
	Score      int
	CreatedAt  time.Time
	JudgedAt   *time.Time
	Results    []SubmissionResult
}

type SubmissionResult struct {
	ID              int64
	SubmissionID    int64
	TestCaseID      int64
	Status          SubmissionStatus
	ExecutionTimeMS *int
	MemoryKB        *int
	Stdout          string
	Stderr          string
	CreatedAt       time.Time
}

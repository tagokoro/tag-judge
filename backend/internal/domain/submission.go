package domain

import "time"

type SubmissionLanguage = ExecutionLanguage

const (
	SubmissionLanguagePython = ExecutionLanguagePython
)

type SubmissionStatus string

const (
	SubmissionStatusPending SubmissionStatus = SubmissionStatus(ExecutionStatusPending)
	SubmissionStatusRunning SubmissionStatus = SubmissionStatus(ExecutionStatusRunning)
	SubmissionStatusAC      SubmissionStatus = "ac"
	SubmissionStatusWA      SubmissionStatus = "wa"
	SubmissionStatusTLE     SubmissionStatus = "tle"
	SubmissionStatusMLE     SubmissionStatus = "mle"
	SubmissionStatusRE      SubmissionStatus = "re"
	SubmissionStatusCE      SubmissionStatus = "ce"
	SubmissionStatusIE      SubmissionStatus = "ie"
)

type Submission struct {
	ID            int64
	UserID        string
	ProblemID     int64
	Language      SubmissionLanguage
	SourceCodeKey string
	TestCasesKey  string
	Status        SubmissionStatus
	Score         int
	// IsPublic is fixed when the submission is accepted: true only when the
	// problem was public then. Anyone may read a public submission.
	IsPublic  bool
	CreatedAt time.Time
	JudgedAt  *time.Time
	// ProblemTitle and SubmitterHandle are filled by reads; a newly created
	// submission leaves them empty. SubmitterHandle is empty when the
	// submitter has no handle.
	ProblemTitle    string
	SubmitterHandle string
}

// PublicSubmission is what anyone may see about a submission to a public
// problem: no source and no user id. SubmitterHandle is empty when the
// submitter has no handle.
type PublicSubmission struct {
	ID              int64
	ProblemID       int64
	ProblemTitle    string
	Language        SubmissionLanguage
	Status          SubmissionStatus
	Score           int
	CreatedAt       time.Time
	JudgedAt        *time.Time
	SubmitterHandle string
}

type SubmissionDetail struct {
	Submission
	SourceCode  string
	CaseResults []SubmissionCaseResult
}

// SubmissionCaseResult contains only metadata safe to expose to submitters.
// Captured output may reveal hidden test inputs and is intentionally excluded.
type SubmissionCaseResult struct {
	TestCaseID      int64
	Status          SubmissionStatus
	ExecutionTimeMS *int
	MemoryKB        *int
}

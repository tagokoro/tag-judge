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
	CreatedAt     time.Time
	JudgedAt      *time.Time
}

// PublicSubmission is what anyone may see about a submission to a public
// problem: no source and no user id. SubmitterNamespaceSlug is empty when the
// submitter has no namespace.
type PublicSubmission struct {
	ID                     int64
	ProblemID              int64
	ProblemTitle           string
	Language               SubmissionLanguage
	Status                 SubmissionStatus
	Score                  int
	CreatedAt              time.Time
	JudgedAt               *time.Time
	SubmitterNamespaceSlug string
}

type SubmissionDetail struct {
	Submission
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

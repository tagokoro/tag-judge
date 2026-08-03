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
	Status        SubmissionStatus
	Score         int
	CreatedAt     time.Time
	JudgedAt      *time.Time
}

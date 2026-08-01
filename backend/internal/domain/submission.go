package domain

import "time"

type SubmissionLanguage string

const (
	SubmissionLanguagePython SubmissionLanguage = "python"
)

func (l SubmissionLanguage) Valid() bool {
	return l == SubmissionLanguagePython
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
	Language   SubmissionLanguage
	SourceCode string
	Status     SubmissionStatus
	Score      int
	CreatedAt  time.Time
	JudgedAt   *time.Time
}

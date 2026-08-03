package domain

import "time"

type ProblemVisibility string

const (
	ProblemVisibilityPublic  ProblemVisibility = "public"
	ProblemVisibilityPrivate ProblemVisibility = "private"
	ProblemVisibilityGroup   ProblemVisibility = "group"
)

func (v ProblemVisibility) Valid() bool {
	switch v {
	case ProblemVisibilityPublic, ProblemVisibilityPrivate, ProblemVisibilityGroup:
		return true
	default:
		return false
	}
}

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

type TestCase struct {
	ID             int64
	ProblemID      int64
	Input          string
	ExpectedOutput string
	IsSample       bool
	Score          int
	CreatedAt      time.Time
}

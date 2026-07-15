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

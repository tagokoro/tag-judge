package domain

import "time"

type CodeRunStatus string

const (
	CodeRunStatusPending CodeRunStatus = CodeRunStatus(ExecutionStatusPending)
	CodeRunStatusRunning CodeRunStatus = CodeRunStatus(ExecutionStatusRunning)
	CodeRunStatusAC      CodeRunStatus = "ac"
	CodeRunStatusTLE     CodeRunStatus = "tle"
	CodeRunStatusRE      CodeRunStatus = "re"
	CodeRunStatusIE      CodeRunStatus = "ie"
)

type CodeRun struct {
	ID              int64
	UserID          string
	Language        ExecutionLanguage
	SourceCode      string
	Stdin           string
	Stdout          string
	Stderr          string
	Status          CodeRunStatus
	ExecutionTimeMS *int
	MemoryKB        *int
	CreatedAt       time.Time
}

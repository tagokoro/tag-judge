package domain

import "time"

type CodeRunStatus string

const (
	CodeRunStatusPending CodeRunStatus = "pending"
	CodeRunStatusRunning CodeRunStatus = "running"
	CodeRunStatusAC      CodeRunStatus = "ac"
	CodeRunStatusTLE     CodeRunStatus = "tle"
	CodeRunStatusRE      CodeRunStatus = "re"
	CodeRunStatusIE      CodeRunStatus = "ie"
)

type CodeRun struct {
	ID              int64
	UserID          string
	Language        string
	SourceCode      string
	Stdin           string
	Stdout          string
	Stderr          string
	Status          CodeRunStatus
	ExecutionTimeMS *int
	MemoryKB        *int
	CreatedAt       time.Time
}

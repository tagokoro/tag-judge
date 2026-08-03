package domain

type ExecutionLanguage string

const (
	ExecutionLanguagePython ExecutionLanguage = "python"
)

func (l ExecutionLanguage) Valid() bool {
	return l == ExecutionLanguagePython
}

type ExecutionStatus string

const (
	ExecutionStatusPending ExecutionStatus = "pending"
	ExecutionStatusRunning ExecutionStatus = "running"
)

func (s ExecutionStatus) Valid() bool {
	return s == ExecutionStatusPending || s == ExecutionStatusRunning
}

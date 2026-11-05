package domain

type ExecutionLanguage string

const (
	ExecutionLanguagePython ExecutionLanguage = "python"
)

func (l ExecutionLanguage) Valid() bool {
	return l == ExecutionLanguagePython
}

func (l ExecutionLanguage) SourceExtension() string {
	switch l {
	case ExecutionLanguagePython:
		return ".py"
	default:
		return ".txt"
	}
}

type ExecutionStatus string

const (
	ExecutionStatusPending ExecutionStatus = "pending"
	ExecutionStatusRunning ExecutionStatus = "running"
)

func (s ExecutionStatus) Valid() bool {
	return s == ExecutionStatusPending || s == ExecutionStatusRunning
}

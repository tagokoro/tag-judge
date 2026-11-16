package domain

// Size limits for submissions and code runs, in UTF-8 bytes. Stdin travels
// inside the queue message, whose limit is 256 KiB, so it gets the headroom
// JSON escaping needs.
const (
	MaxExecutionSourceBytes = 64 * 1024
	MaxCodeRunStdinBytes    = 128 * 1024
)

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

package judgeinfra

import (
	"context"
	"log/slog"

	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type LoggingDispatcher struct {
	logger *slog.Logger
}

func NewLoggingDispatcher(logger *slog.Logger) *LoggingDispatcher {
	return &LoggingDispatcher{logger: logger}
}

func (d *LoggingDispatcher) Dispatch(_ context.Context, job repository.ExecutionJob) error {
	reference := job.Reference()
	mode := job.Mode()
	attrs := []any{
		"reference_kind", reference.Kind(),
		"reference_id", reference.ID(),
		"mode", mode.Kind(),
		"language", job.Language(),
		"source_code_key", job.SourceCodeKey(),
	}
	if problem, ok := mode.Problem(); ok {
		attrs = append(attrs,
			"test_cases_key", problem.TestCasesKey,
			"time_limit_ms", problem.TimeLimitMS,
		)
	}
	if stdin, ok := mode.Stdin(); ok {
		attrs = append(attrs,
			"stdin_bytes", len(stdin.Stdin),
			"time_limit_ms", stdin.TimeLimitMS,
		)
	}
	d.logger.Info("execution job dispatch skipped by local dispatcher", attrs...)
	return nil
}

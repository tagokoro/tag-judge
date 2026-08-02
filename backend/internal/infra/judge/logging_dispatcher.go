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

func (d *LoggingDispatcher) Dispatch(_ context.Context, job repository.JudgeJob) error {
	d.logger.Info(
		"judge job dispatch skipped by local dispatcher",
		"submission_id", job.SubmissionID,
		"language", job.Language,
		"source_code_key", job.SourceCodeKey,
		"test_cases_key", job.TestCasesKey,
		"time_limit_ms", job.TimeLimitMS,
	)
	return nil
}

package usecase

import (
	"fmt"
	"strings"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
)

func sourceExtension(language domain.ExecutionLanguage) string {
	return language.SourceExtension()
}

// validateSourceCode is shared by submissions and code runs.
func validateSourceCode(sourceCode string) error {
	if strings.TrimSpace(sourceCode) == "" {
		return apperror.InvalidInput("INVALID_SOURCE_CODE", "source_code must not be empty")
	}
	if len(sourceCode) > domain.MaxExecutionSourceBytes {
		return apperror.InvalidInput("SOURCE_CODE_TOO_LARGE", fmt.Sprintf("source_code must be at most %d bytes", domain.MaxExecutionSourceBytes))
	}
	return nil
}

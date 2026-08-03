package usecase

import "github.com/tagokoro/tag-judge/backend/internal/domain"

func sourceExtension(language domain.ExecutionLanguage) string {
	switch language {
	case domain.ExecutionLanguagePython:
		return ".py"
	default:
		return ".txt"
	}
}

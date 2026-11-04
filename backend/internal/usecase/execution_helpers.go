package usecase

import "github.com/tagokoro/tag-judge/backend/internal/domain"

func sourceExtension(language domain.ExecutionLanguage) string {
	return language.SourceExtension()
}

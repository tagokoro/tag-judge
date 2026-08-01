package usecase

import (
	"context"
	"strings"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

const (
	defaultCodeRunLimit = 20
	maxCodeRunLimit     = 100
)

type CodeRunUsecase struct {
	codeRuns repository.CodeRunRepository
}

func NewCodeRunUsecase(codeRuns repository.CodeRunRepository) *CodeRunUsecase {
	return &CodeRunUsecase{codeRuns: codeRuns}
}

type CreateCodeRunInput struct {
	Language   string
	SourceCode string
	Stdin      string
}

type ListCodeRunsInput struct {
	Limit         int
	LimitProvided bool
	Offset        int
}

func (u *CodeRunUsecase) Create(ctx context.Context, actor auth.Actor, input CreateCodeRunInput) (*domain.CodeRun, error) {
	if actor.UserID == "" {
		return nil, apperror.Unauthorized("UNAUTHORIZED", "authentication required")
	}
	if input.Language != "python" {
		return nil, apperror.InvalidInput("INVALID_LANGUAGE", "language must be python")
	}
	if strings.TrimSpace(input.SourceCode) == "" {
		return nil, apperror.InvalidInput("INVALID_SOURCE_CODE", "source_code must not be empty")
	}

	return u.codeRuns.Create(ctx, repository.CreateCodeRunParams{
		UserID:     actor.UserID,
		Language:   input.Language,
		SourceCode: input.SourceCode,
		Stdin:      input.Stdin,
	})
}

func (u *CodeRunUsecase) Get(ctx context.Context, actor auth.Actor, id int64) (*domain.CodeRun, error) {
	if actor.UserID == "" {
		return nil, apperror.Unauthorized("UNAUTHORIZED", "authentication required")
	}
	if id <= 0 {
		return nil, apperror.InvalidInput("INVALID_CODE_RUN_ID", "code_run_id must be a positive integer")
	}

	codeRun, err := u.codeRuns.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if actor.Role != domain.UserRoleAdmin && codeRun.UserID != actor.UserID {
		return nil, apperror.Forbidden("CODE_RUN_FORBIDDEN", "code run is not accessible")
	}

	return codeRun, nil
}

func (u *CodeRunUsecase) List(ctx context.Context, actor auth.Actor, input ListCodeRunsInput) ([]domain.CodeRun, error) {
	if actor.UserID == "" {
		return nil, apperror.Unauthorized("UNAUTHORIZED", "authentication required")
	}

	limit := input.Limit
	if !input.LimitProvided {
		limit = defaultCodeRunLimit
	}
	if limit <= 0 || limit > maxCodeRunLimit {
		return nil, apperror.InvalidInput("INVALID_LIMIT", "limit must be between 1 and 100")
	}
	if input.Offset < 0 {
		return nil, apperror.InvalidInput("INVALID_OFFSET", "offset must be greater than or equal to 0")
	}

	return u.codeRuns.ListByUserID(ctx, repository.ListCodeRunsParams{
		UserID: actor.UserID,
		Limit:  limit,
		Offset: input.Offset,
	})
}

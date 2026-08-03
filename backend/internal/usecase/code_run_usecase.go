package usecase

import (
	"context"
	"fmt"
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
	codeRuns   repository.CodeRunRepository
	storage    repository.SourceCodeStorage
	dispatcher repository.ExecutionDispatcher
	timeLimit  int
}

func NewCodeRunUsecase(
	codeRuns repository.CodeRunRepository,
	storage repository.SourceCodeStorage,
	dispatcher repository.ExecutionDispatcher,
	timeLimitMS int,
) *CodeRunUsecase {
	return &CodeRunUsecase{
		codeRuns:   codeRuns,
		storage:    storage,
		dispatcher: dispatcher,
		timeLimit:  timeLimitMS,
	}
}

type CreateCodeRunInput struct {
	Language   domain.ExecutionLanguage
	SourceCode string
	Stdin      string
}

type ListCodeRunsInput struct {
	Limit         int
	LimitProvided bool
	Offset        int
}

func (u *CodeRunUsecase) Create(ctx context.Context, actor auth.Actor, input CreateCodeRunInput) (*domain.CodeRun, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	if !input.Language.Valid() {
		return nil, apperror.InvalidInput("INVALID_LANGUAGE", "language must be python")
	}
	if strings.TrimSpace(input.SourceCode) == "" {
		return nil, apperror.InvalidInput("INVALID_SOURCE_CODE", "source_code must not be empty")
	}

	codeRun, err := u.codeRuns.Create(ctx, repository.CreateCodeRunParams{
		UserID:     actor.UserID,
		Language:   input.Language,
		SourceCode: input.SourceCode,
		Stdin:      input.Stdin,
	})
	if err != nil {
		return nil, err
	}

	sourceCodeKey := codeRunSourceCodeKey(*codeRun)
	if err := u.storage.PutSourceCode(ctx, repository.PutSourceCodeParams{
		Key:        sourceCodeKey,
		Language:   input.Language,
		SourceCode: input.SourceCode,
	}); err != nil {
		return nil, err
	}

	reference, err := repository.NewCodeRunExecutionReference(fmt.Sprintf("%d", codeRun.ID))
	if err != nil {
		return nil, err
	}
	mode, err := repository.NewStdinExecution(input.Stdin, u.timeLimit)
	if err != nil {
		return nil, err
	}
	job, err := repository.NewExecutionJob(reference, input.Language, sourceCodeKey, mode)
	if err != nil {
		return nil, err
	}
	if err := u.dispatcher.Dispatch(ctx, job); err != nil {
		return nil, err
	}

	return codeRun, nil
}

func (u *CodeRunUsecase) Get(ctx context.Context, actor auth.Actor, id int64) (*domain.CodeRun, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
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
	if err := validateActor(actor); err != nil {
		return nil, err
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

func codeRunSourceCodeKey(codeRun domain.CodeRun) string {
	return fmt.Sprintf("code-runs/%s/%d/main%s", codeRun.UserID, codeRun.ID, sourceExtension(codeRun.Language))
}

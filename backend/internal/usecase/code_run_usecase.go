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
	codeRuns             repository.CodeRunRepository
	storage              repository.SourceCodeStorage
	admission            repository.AdmissionControl
	timeLimit            int
	idempotencyKeyMaxLen int
}

func NewCodeRunUsecase(
	codeRuns repository.CodeRunRepository,
	storage repository.SourceCodeStorage,
	admission repository.AdmissionControl,
	timeLimitMS int,
	idempotencyKeyMaxLen int,
) *CodeRunUsecase {
	return &CodeRunUsecase{
		codeRuns:             codeRuns,
		storage:              storage,
		admission:            admission,
		timeLimit:            timeLimitMS,
		idempotencyKeyMaxLen: idempotencyKeyMaxLen,
	}
}

type CreateCodeRunInput struct {
	Language       domain.ExecutionLanguage
	SourceCode     string
	Stdin          string
	IdempotencyKey string
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
	idempotency, err := buildCodeRunIdempotency(actor.UserID, u.idempotencyKeyMaxLen, input)
	if err != nil {
		return nil, err
	}

	// Reserve the ID and store the artifact before the admission transaction.
	// This guarantees that any accepted CodeRun has an existing artifact before
	// the publisher can dispatch the job. A rejected admission leaves an orphan
	// artifact.
	reservedID, err := u.admission.ReserveCodeRunID(ctx)
	if err != nil {
		return nil, err
	}
	sourceCodeKey := codeRunSourceCodeKey(actor.UserID, reservedID, input.Language)
	if err := u.storage.PutSourceCode(ctx, repository.PutSourceCodeParams{
		Key:        sourceCodeKey,
		Language:   input.Language,
		SourceCode: input.SourceCode,
	}); err != nil {
		return nil, err
	}

	codeRun, err := u.admission.CreateCodeRun(ctx, repository.CreateCodeRunParams{
		UserID:        actor.UserID,
		Language:      input.Language,
		SourceCode:    input.SourceCode,
		Stdin:         input.Stdin,
		SourceCodeKey: sourceCodeKey,
	}, idempotency, u.timeLimit, reservedID)
	if err != nil {
		return nil, err
	}

	// If the call was an idempotent replay, the returned record is the original
	// one and the reserved ID was not used. The original artifact already exists,
	// so there is nothing more to store.
	if codeRun.ID != reservedID {
		return codeRun, nil
	}

	return codeRun, nil
}

func buildCodeRunIdempotency(userID string, maxLen int, input CreateCodeRunInput) (*repository.IdempotencyKeyInfo, error) {
	fingerprint, err := codeRunFingerprint(input)
	if err != nil {
		return nil, err
	}
	return newIdempotencyKey(userID, repository.IdempotencyNamespaceCodeRun, input.IdempotencyKey, fingerprint, maxLen)
}

func codeRunFingerprint(input CreateCodeRunInput) (string, error) {
	return canonicalFingerprint(struct {
		Language   string `json:"language"`
		SourceCode string `json:"source_code"`
		Stdin      string `json:"stdin"`
	}{
		Language:   string(input.Language),
		SourceCode: input.SourceCode,
		Stdin:      input.Stdin,
	})
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

func codeRunSourceCodeKey(userID string, codeRunID int64, language domain.ExecutionLanguage) string {
	return fmt.Sprintf("code-runs/%s/%d/main%s", userID, codeRunID, language.SourceExtension())
}

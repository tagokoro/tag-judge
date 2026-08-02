package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres/sqlc"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type CodeRunRepository struct {
	queries *sqlc.Queries
}

func NewCodeRunRepository(db *pgxpool.Pool) *CodeRunRepository {
	return &CodeRunRepository{queries: sqlc.New(db)}
}

func (r *CodeRunRepository) Create(ctx context.Context, params repository.CreateCodeRunParams) (*domain.CodeRun, error) {
	userID, err := parseUUID(params.UserID)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.CreateCodeRun(ctx, sqlc.CreateCodeRunParams{
		UserID:     userID,
		Language:   params.Language,
		SourceCode: params.SourceCode,
		Stdin:      params.Stdin,
	})
	if err != nil {
		return nil, err
	}

	codeRun := newCodeRun(row.ID, row.UserID, row.Language, row.SourceCode, row.Stdin, row.Stdout, row.Stderr, row.Status, row.ExecutionTimeMs, row.MemoryKb, row.CreatedAt)
	return &codeRun, nil
}

func (r *CodeRunRepository) FindByID(ctx context.Context, id int64) (*domain.CodeRun, error) {
	row, err := r.queries.GetCodeRun(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("CODE_RUN_NOT_FOUND", "code run not found")
	}
	if err != nil {
		return nil, err
	}

	codeRun := newCodeRun(row.ID, row.UserID, row.Language, row.SourceCode, row.Stdin, row.Stdout, row.Stderr, row.Status, row.ExecutionTimeMs, row.MemoryKb, row.CreatedAt)
	return &codeRun, nil
}

func (r *CodeRunRepository) ListByUserID(ctx context.Context, params repository.ListCodeRunsParams) ([]domain.CodeRun, error) {
	userID, err := parseUUID(params.UserID)
	if err != nil {
		return nil, err
	}

	rows, err := r.queries.ListCodeRunsByUserID(ctx, sqlc.ListCodeRunsByUserIDParams{
		UserID: userID,
		Limit:  int32(params.Limit),
		Offset: int32(params.Offset),
	})
	if err != nil {
		return nil, err
	}

	codeRuns := make([]domain.CodeRun, 0, len(rows))
	for _, row := range rows {
		codeRuns = append(codeRuns, newCodeRun(row.ID, row.UserID, row.Language, row.SourceCode, row.Stdin, row.Stdout, row.Stderr, row.Status, row.ExecutionTimeMs, row.MemoryKb, row.CreatedAt))
	}

	return codeRuns, nil
}

func newCodeRun(
	id int64,
	userID string,
	language string,
	sourceCode string,
	stdin string,
	stdout string,
	stderr string,
	status string,
	executionTimeMS pgtype.Int4,
	memoryKB pgtype.Int4,
	createdAt pgtype.Timestamptz,
) domain.CodeRun {
	return domain.CodeRun{
		ID:              id,
		UserID:          userID,
		Language:        language,
		SourceCode:      sourceCode,
		Stdin:           stdin,
		Stdout:          stdout,
		Stderr:          stderr,
		Status:          domain.CodeRunStatus(status),
		ExecutionTimeMS: optionalInt(executionTimeMS),
		MemoryKB:        optionalInt(memoryKB),
		CreatedAt:       createdAt.Time,
	}
}

func optionalInt(value pgtype.Int4) *int {
	if !value.Valid {
		return nil
	}
	converted := int(value.Int32)
	return &converted
}

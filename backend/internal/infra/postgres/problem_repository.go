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

type ProblemRepository struct {
	db      *pgxpool.Pool
	queries *sqlc.Queries
}

func NewProblemRepository(db *pgxpool.Pool) *ProblemRepository {
	return &ProblemRepository{db: db, queries: sqlc.New(db)}
}

func (r *ProblemRepository) Create(ctx context.Context, params repository.CreateProblemParams) (*domain.Problem, error) {
	var createdBy pgtype.UUID
	if err := createdBy.Scan(params.CreatedBy); err != nil {
		return nil, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := r.queries.WithTx(tx)
	row, err := queries.CreateProblem(ctx, sqlc.CreateProblemParams{
		Title:         params.Title,
		Statement:     params.Statement,
		TimeLimitMs:   int32(params.TimeLimitMS),
		MemoryLimitKb: int32(params.MemoryLimitKB),
		Visibility:    string(params.Visibility),
		CreatedBy:     createdBy,
	})
	if err != nil {
		return nil, err
	}

	for _, testCase := range params.TestCases {
		err = queries.CreateTestCase(ctx, sqlc.CreateTestCaseParams{
			ProblemID:      row.ID,
			Input:          testCase.Input,
			ExpectedOutput: testCase.ExpectedOutput,
			IsSample:       testCase.IsSample,
			Score:          int32(testCase.Score),
		})
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	problem := newProblem(row.ID, row.Title, row.Statement, row.TimeLimitMs, row.MemoryLimitKb, row.Visibility, row.CreatedBy, row.CreatedAt, row.UpdatedAt)
	return &problem, nil
}

func (r *ProblemRepository) ListPublic(ctx context.Context, params repository.ListProblemsParams) ([]domain.Problem, error) {
	rows, err := r.queries.ListPublicProblems(ctx, sqlc.ListPublicProblemsParams{
		Limit:  int32(params.Limit),
		Offset: int32(params.Offset),
	})
	if err != nil {
		return nil, err
	}

	problems := make([]domain.Problem, 0, len(rows))
	for _, row := range rows {
		problems = append(problems, newProblem(row.ID, row.Title, row.Statement, row.TimeLimitMs, row.MemoryLimitKb, row.Visibility, row.CreatedBy, row.CreatedAt, row.UpdatedAt))
	}

	return problems, nil
}

func (r *ProblemRepository) FindPublicByID(ctx context.Context, id int64) (*domain.Problem, error) {
	row, err := r.queries.GetPublicProblem(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("PROBLEM_NOT_FOUND", "problem not found")
	}
	if err != nil {
		return nil, err
	}

	problem := newProblem(row.ID, row.Title, row.Statement, row.TimeLimitMs, row.MemoryLimitKb, row.Visibility, row.CreatedBy, row.CreatedAt, row.UpdatedAt)
	return &problem, nil
}

func newProblem(
	id int64,
	title string,
	statement string,
	timeLimitMS int32,
	memoryLimitKB int32,
	visibility string,
	createdBy string,
	createdAt pgtype.Timestamptz,
	updatedAt pgtype.Timestamptz,
) domain.Problem {
	return domain.Problem{
		ID:            id,
		Title:         title,
		Statement:     statement,
		TimeLimitMS:   int(timeLimitMS),
		MemoryLimitKB: int(memoryLimitKB),
		Visibility:    domain.ProblemVisibility(visibility),
		CreatedBy:     createdBy,
		CreatedAt:     createdAt.Time,
		UpdatedAt:     updatedAt.Time,
	}
}

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres/sqlc"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type ProblemRepository struct {
	queries *sqlc.Queries
}

func NewProblemRepository(db *pgxpool.Pool) *ProblemRepository {
	return &ProblemRepository{queries: sqlc.New(db)}
}

func (r *ProblemRepository) Create(ctx context.Context, params repository.CreateProblemParams) (*domain.Problem, error) {
	var createdBy pgtype.UUID
	if err := createdBy.Scan(params.CreatedBy); err != nil {
		return nil, err
	}

	row, err := r.queries.CreateProblem(ctx, sqlc.CreateProblemParams{
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

	problem := newProblem(row.ID, row.Title, row.Statement, row.TimeLimitMs, row.MemoryLimitKb, row.Visibility, row.CreatedBy, row.CreatedAt, row.UpdatedAt)
	return &problem, nil
}

func (r *ProblemRepository) FindByID(ctx context.Context, id int64) (*domain.Problem, error) {
	row, err := r.queries.GetProblem(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("PROBLEM_NOT_FOUND", "problem not found")
	}
	if err != nil {
		return nil, err
	}

	problem := newProblem(row.ID, row.Title, row.Statement, row.TimeLimitMs, row.MemoryLimitKb, row.Visibility, row.CreatedBy, row.CreatedAt, row.UpdatedAt)
	return &problem, nil
}

func (r *ProblemRepository) Update(ctx context.Context, params repository.UpdateProblemParams) (*domain.Problem, error) {
	row, err := r.queries.UpdateProblem(ctx, sqlc.UpdateProblemParams{
		ID:            params.ID,
		Title:         params.Title,
		Statement:     params.Statement,
		TimeLimitMs:   int32(params.TimeLimitMS),
		MemoryLimitKb: int32(params.MemoryLimitKB),
		Visibility:    string(params.Visibility),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("PROBLEM_NOT_FOUND", "problem not found")
	}
	if err != nil {
		return nil, err
	}

	problem := newProblem(row.ID, row.Title, row.Statement, row.TimeLimitMs, row.MemoryLimitKb, row.Visibility, row.CreatedBy, row.CreatedAt, row.UpdatedAt)
	return &problem, nil
}

func (r *ProblemRepository) Delete(ctx context.Context, id int64) error {
	rows, err := r.queries.DeleteProblem(ctx, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return apperror.Conflict("PROBLEM_HAS_SUBMISSIONS", "problem with submissions cannot be deleted")
		}
		return err
	}
	if rows == 0 {
		return apperror.NotFound("PROBLEM_NOT_FOUND", "problem not found")
	}
	return nil
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

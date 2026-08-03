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

type TestCaseRepository struct {
	queries *sqlc.Queries
}

func NewTestCaseRepository(db *pgxpool.Pool) *TestCaseRepository {
	return &TestCaseRepository{queries: sqlc.New(db)}
}

func (r *TestCaseRepository) CountByProblemID(ctx context.Context, problemID int64) (int, error) {
	count, err := r.queries.CountTestCases(ctx, problemID)
	return int(count), err
}

func (r *TestCaseRepository) ListByProblemID(ctx context.Context, problemID int64) ([]domain.TestCase, error) {
	rows, err := r.queries.ListTestCases(ctx, problemID)
	if err != nil {
		return nil, err
	}
	testCases := make([]domain.TestCase, 0, len(rows))
	for _, row := range rows {
		testCases = append(testCases, newTestCase(row.ID, row.ProblemID, row.Input, row.ExpectedOutput, row.IsSample, row.Score, row.CreatedAt))
	}
	return testCases, nil
}

func (r *TestCaseRepository) FindByID(ctx context.Context, problemID, id int64) (*domain.TestCase, error) {
	row, err := r.queries.GetTestCase(ctx, sqlc.GetTestCaseParams{ProblemID: problemID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("TEST_CASE_NOT_FOUND", "test case not found")
	}
	if err != nil {
		return nil, err
	}
	testCase := newTestCase(row.ID, row.ProblemID, row.Input, row.ExpectedOutput, row.IsSample, row.Score, row.CreatedAt)
	return &testCase, nil
}

func (r *TestCaseRepository) Create(ctx context.Context, params repository.CreateTestCaseParams) (*domain.TestCase, error) {
	row, err := r.queries.CreateTestCase(ctx, sqlc.CreateTestCaseParams{
		ProblemID:      params.ProblemID,
		Input:          params.Input,
		ExpectedOutput: params.ExpectedOutput,
		IsSample:       params.IsSample,
		Score:          int32(params.Score),
	})
	if err != nil {
		return nil, err
	}
	testCase := newTestCase(row.ID, row.ProblemID, row.Input, row.ExpectedOutput, row.IsSample, row.Score, row.CreatedAt)
	return &testCase, nil
}

func (r *TestCaseRepository) Update(ctx context.Context, params repository.UpdateTestCaseParams) (*domain.TestCase, error) {
	row, err := r.queries.UpdateTestCase(ctx, sqlc.UpdateTestCaseParams{
		ID:             params.ID,
		ProblemID:      params.ProblemID,
		Input:          params.Input,
		ExpectedOutput: params.ExpectedOutput,
		IsSample:       params.IsSample,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("TEST_CASE_NOT_FOUND", "test case not found")
	}
	if err != nil {
		return nil, err
	}
	testCase := newTestCase(row.ID, row.ProblemID, row.Input, row.ExpectedOutput, row.IsSample, row.Score, row.CreatedAt)
	return &testCase, nil
}

func (r *TestCaseRepository) Delete(ctx context.Context, problemID, id int64) error {
	rows, err := r.queries.DeleteTestCase(ctx, sqlc.DeleteTestCaseParams{ProblemID: problemID, ID: id})
	if err != nil {
		return err
	}
	if rows == 0 {
		return apperror.NotFound("TEST_CASE_NOT_FOUND", "test case not found")
	}
	return nil
}

func newTestCase(id, problemID int64, input, expectedOutput string, isSample bool, score int32, createdAt pgtype.Timestamptz) domain.TestCase {
	return domain.TestCase{
		ID:             id,
		ProblemID:      problemID,
		Input:          input,
		ExpectedOutput: expectedOutput,
		IsSample:       isSample,
		Score:          int(score),
		CreatedAt:      createdAt.Time,
	}
}

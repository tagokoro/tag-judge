package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres/sqlc"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type SubmissionRepository struct {
	queries *sqlc.Queries
}

func NewSubmissionRepository(db *pgxpool.Pool) *SubmissionRepository {
	return &SubmissionRepository{queries: sqlc.New(db)}
}

func (r *SubmissionRepository) Create(ctx context.Context, params repository.CreateSubmissionParams) (*domain.Submission, error) {
	row, err := r.queries.CreateSubmission(ctx, sqlc.CreateSubmissionParams{
		UserID:        params.UserID,
		ProblemID:     params.ProblemID,
		Language:      string(params.Language),
		SourceCodeKey: params.SourceCodeKey,
		Status:        string(params.Status),
		Score:         int32(params.Score),
	})
	if err != nil {
		return nil, err
	}

	submission := newSubmission(row.ID, row.UserID, row.ProblemID, row.Language, row.SourceCodeKey, row.Status, row.Score, row.CreatedAt, row.JudgedAt)
	return &submission, nil
}

func (r *SubmissionRepository) FindByID(ctx context.Context, id int64) (*domain.Submission, error) {
	row, err := r.queries.GetSubmission(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("SUBMISSION_NOT_FOUND", "submission not found")
	}
	if err != nil {
		return nil, err
	}

	submission := newSubmission(row.ID, row.UserID, row.ProblemID, row.Language, row.SourceCodeKey, row.Status, row.Score, row.CreatedAt, row.JudgedAt)
	return &submission, nil
}

func (r *SubmissionRepository) ListByUserID(ctx context.Context, params repository.ListSubmissionsParams) ([]domain.Submission, error) {
	rows, err := r.queries.ListUserSubmissions(ctx, sqlc.ListUserSubmissionsParams{
		UserID:       params.UserID,
		ResultLimit:  int32(params.Limit),
		ResultOffset: int32(params.Offset),
	})
	if err != nil {
		return nil, err
	}

	submissions := make([]domain.Submission, 0, len(rows))
	for _, row := range rows {
		submissions = append(submissions, newSubmission(row.ID, row.UserID, row.ProblemID, row.Language, row.SourceCodeKey, row.Status, row.Score, row.CreatedAt, row.JudgedAt))
	}
	return submissions, nil
}

func newSubmission(
	id int64,
	userID string,
	problemID int64,
	language string,
	sourceCodeKey string,
	status string,
	score int32,
	createdAt pgtype.Timestamptz,
	judgedAt pgtype.Timestamptz,
) domain.Submission {
	var judgedAtTime *time.Time
	if judgedAt.Valid {
		value := judgedAt.Time
		judgedAtTime = &value
	}

	return domain.Submission{
		ID:            id,
		UserID:        userID,
		ProblemID:     problemID,
		Language:      domain.SubmissionLanguage(language),
		SourceCodeKey: sourceCodeKey,
		Status:        domain.SubmissionStatus(status),
		Score:         int(score),
		CreatedAt:     createdAt.Time,
		JudgedAt:      judgedAtTime,
	}
}

package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

func TestListPublicProblemsDefaultLimit(t *testing.T) {
	repo := &fakeProblemRepository{}
	uc := NewProblemUsecase(repo)

	_, err := uc.ListPublicProblems(context.Background(), ListProblemsInput{})
	if err != nil {
		t.Fatalf("ListPublicProblems returned error: %v", err)
	}

	if repo.listParams.Limit != defaultProblemLimit {
		t.Fatalf("limit = %d, want %d", repo.listParams.Limit, defaultProblemLimit)
	}
	if repo.listParams.Offset != 0 {
		t.Fatalf("offset = %d, want 0", repo.listParams.Offset)
	}
}

func TestListPublicProblemsInvalidLimit(t *testing.T) {
	repo := &fakeProblemRepository{}
	uc := NewProblemUsecase(repo)

	_, err := uc.ListPublicProblems(context.Background(), ListProblemsInput{Limit: maxProblemLimit + 1, LimitProvided: true})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if repo.listCalled {
		t.Fatal("repository should not be called")
	}
}

func TestListPublicProblemsZeroLimit(t *testing.T) {
	repo := &fakeProblemRepository{}
	uc := NewProblemUsecase(repo)

	_, err := uc.ListPublicProblems(context.Background(), ListProblemsInput{Limit: 0, LimitProvided: true})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if repo.listCalled {
		t.Fatal("repository should not be called")
	}
}

func TestGetPublicProblemInvalidID(t *testing.T) {
	repo := &fakeProblemRepository{}
	uc := NewProblemUsecase(repo)

	_, err := uc.GetPublicProblem(context.Background(), 0)
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if repo.findCalled {
		t.Fatal("repository should not be called")
	}
}

type fakeProblemRepository struct {
	listCalled bool
	listParams repository.ListProblemsParams
	findCalled bool
}

func (r *fakeProblemRepository) ListPublic(_ context.Context, params repository.ListProblemsParams) ([]domain.Problem, error) {
	r.listCalled = true
	r.listParams = params
	return []domain.Problem{}, nil
}

func (r *fakeProblemRepository) ListVisible(_ context.Context, params repository.ListProblemsParams, _ string, _ bool) ([]domain.Problem, error) {
	r.listCalled = true
	r.listParams = params
	return []domain.Problem{}, nil
}

func (r *fakeProblemRepository) FindPublicByID(_ context.Context, id int64) (*domain.Problem, error) {
	r.findCalled = true
	return &domain.Problem{ID: id}, nil
}

func (r *fakeProblemRepository) FindVisibleByID(_ context.Context, id int64, _ string, _ bool) (*domain.Problem, error) {
	r.findCalled = true
	return &domain.Problem{ID: id, CreatedBy: "00000000-0000-4000-8000-000000000001"}, nil
}

func (r *fakeProblemRepository) Create(_ context.Context, params repository.ProblemMutationParams) (*domain.Problem, error) {
	return &domain.Problem{ID: 1, Title: params.Title, CreatedBy: params.CreatedBy}, nil
}

func (r *fakeProblemRepository) Update(_ context.Context, id int64, params repository.ProblemMutationParams) (*domain.Problem, error) {
	return &domain.Problem{ID: id, Title: params.Title, CreatedBy: params.CreatedBy}, nil
}

func (r *fakeProblemRepository) Delete(_ context.Context, _ int64) error {
	return nil
}

func (r *fakeProblemRepository) ListTestCases(_ context.Context, problemID int64) ([]domain.TestCase, error) {
	return []domain.TestCase{{ID: 1, ProblemID: problemID}}, nil
}

func (r *fakeProblemRepository) CreateTestCase(_ context.Context, params repository.TestCaseMutationParams) (*domain.TestCase, error) {
	return &domain.TestCase{ID: 1, ProblemID: params.ProblemID, Score: params.Score}, nil
}

func (r *fakeProblemRepository) UpdateTestCase(_ context.Context, id int64, params repository.TestCaseMutationParams) (*domain.TestCase, error) {
	return &domain.TestCase{ID: id, ProblemID: params.ProblemID, Score: params.Score}, nil
}

func (r *fakeProblemRepository) DeleteTestCase(_ context.Context, _ int64, _ int64) error {
	return nil
}

func (r *fakeProblemRepository) CreateSubmission(_ context.Context, params repository.CreateSubmissionParams) (*domain.Submission, error) {
	return &domain.Submission{ID: 1, UserID: params.UserID, ProblemID: params.ProblemID, Language: params.Language}, nil
}

func (r *fakeProblemRepository) ListSubmissionsByProblem(_ context.Context, problemID int64, userID string, _ bool) ([]domain.Submission, error) {
	return []domain.Submission{{ID: 1, ProblemID: problemID, UserID: userID}}, nil
}

func (r *fakeProblemRepository) FindSubmissionByID(_ context.Context, id int64) (*domain.Submission, error) {
	return &domain.Submission{ID: id, UserID: "00000000-0000-4000-8000-000000000001", ProblemID: 1}, nil
}

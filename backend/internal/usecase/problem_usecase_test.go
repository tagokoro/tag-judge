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

func (r *fakeProblemRepository) FindPublicByID(_ context.Context, id int64) (*domain.Problem, error) {
	r.findCalled = true
	return &domain.Problem{ID: id}, nil
}

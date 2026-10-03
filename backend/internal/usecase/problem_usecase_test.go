package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

const (
	problemAuthorID = "00000000-0000-4000-8000-000000000001"
	problemAdminID  = "00000000-0000-4000-8000-000000000002"
	problemUserID   = "00000000-0000-4000-8000-000000000003"
)

func TestListProblems(t *testing.T) {
	repo := newFakeProblemRepository()
	uc := NewProblemUsecase(repo, &fakeTestCaseCounter{})

	_, err := uc.ListPublicProblems(context.Background(), ListProblemsInput{})
	if err != nil {
		t.Fatalf("ListPublicProblems returned error: %v", err)
	}
	if repo.listParams.Limit != defaultProblemLimit || repo.listParams.Offset != 0 {
		t.Fatalf("params = %+v", repo.listParams)
	}
}

func TestListProblemsLimit(t *testing.T) {
	repo := newFakeProblemRepository()
	uc := NewProblemUsecase(repo, &fakeTestCaseCounter{})

	_, err := uc.ListPublicProblems(context.Background(), ListProblemsInput{Limit: 0, LimitProvided: true})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if repo.listCalled {
		t.Fatal("repository should not be called")
	}
}

func TestGetProblemID(t *testing.T) {
	repo := newFakeProblemRepository()
	uc := NewProblemUsecase(repo, &fakeTestCaseCounter{})

	_, err := uc.GetPublicProblem(context.Background(), 0)
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestCreateProblem(t *testing.T) {
	repo := newFakeProblemRepository()
	uc := NewProblemUsecase(repo, &fakeTestCaseCounter{})

	problem, err := uc.CreateProblem(context.Background(), authorActor(), validCreateInput())
	if err != nil {
		t.Fatalf("CreateProblem returned error: %v", err)
	}
	if problem.ID != 42 || repo.createParams.CreatedBy != problemAuthorID {
		t.Fatalf("problem = %+v, params = %+v", problem, repo.createParams)
	}
}

func TestCreateProblemPublic(t *testing.T) {
	repo := newFakeProblemRepository()
	uc := NewProblemUsecase(repo, &fakeTestCaseCounter{})
	input := validCreateInput()
	input.Visibility = domain.ProblemVisibilityPublic

	_, err := uc.CreateProblem(context.Background(), authorActor(), input)
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if repo.createCalled {
		t.Fatal("repository should not be called")
	}
}

func TestCreateProblemForbidden(t *testing.T) {
	repo := newFakeProblemRepository()
	uc := NewProblemUsecase(repo, &fakeTestCaseCounter{})

	_, err := uc.CreateProblem(context.Background(), auth.Actor{UserID: problemUserID, Role: domain.UserRoleUser}, validCreateInput())
	if !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestCreateProblemValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CreateProblemInput)
	}{
		{name: "title", mutate: func(input *CreateProblemInput) { input.Title = " " }},
		{name: "statement", mutate: func(input *CreateProblemInput) { input.Statement = "" }},
		{name: "time", mutate: func(input *CreateProblemInput) { input.TimeLimitMS = 0 }},
		{name: "memory", mutate: func(input *CreateProblemInput) { input.MemoryLimitKB = 0 }},
		{name: "visibility", mutate: func(input *CreateProblemInput) { input.Visibility = "draft" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeProblemRepository()
			uc := NewProblemUsecase(repo, &fakeTestCaseCounter{})
			input := validCreateInput()
			tt.mutate(&input)
			_, err := uc.CreateProblem(context.Background(), authorActor(), input)
			if !errors.Is(err, apperror.ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestUpdateProblem(t *testing.T) {
	repo := newFakeProblemRepository()
	cases := &fakeTestCaseCounter{count: 1}
	uc := NewProblemUsecase(repo, cases)
	title := "Updated"
	visibility := domain.ProblemVisibilityPublic

	problem, err := uc.UpdateProblem(context.Background(), authorActor(), 42, UpdateProblemInput{Title: &title, Visibility: &visibility})
	if err != nil {
		t.Fatalf("UpdateProblem returned error: %v", err)
	}
	if problem.Title != title || problem.Visibility != visibility || cases.problemID != 42 {
		t.Fatalf("problem = %+v", problem)
	}
}

func TestUpdateProblemEmpty(t *testing.T) {
	repo := newFakeProblemRepository()
	uc := NewProblemUsecase(repo, &fakeTestCaseCounter{})

	_, err := uc.UpdateProblem(context.Background(), authorActor(), 42, UpdateProblemInput{})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestUpdateProblemPublic(t *testing.T) {
	repo := newFakeProblemRepository()
	uc := NewProblemUsecase(repo, &fakeTestCaseCounter{})
	visibility := domain.ProblemVisibilityPublic

	_, err := uc.UpdateProblem(context.Background(), authorActor(), 42, UpdateProblemInput{Visibility: &visibility})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if repo.updateCalled {
		t.Fatal("repository should not be called")
	}
}

func TestUpdateProblemOwner(t *testing.T) {
	repo := newFakeProblemRepository()
	repo.problem.CreatedBy = problemAdminID
	uc := NewProblemUsecase(repo, &fakeTestCaseCounter{})
	title := "Updated"

	_, err := uc.UpdateProblem(context.Background(), authorActor(), 42, UpdateProblemInput{Title: &title})
	if !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestDeleteProblem(t *testing.T) {
	repo := newFakeProblemRepository()
	uc := NewProblemUsecase(repo, &fakeTestCaseCounter{})

	if err := uc.DeleteProblem(context.Background(), authorActor(), 42); err != nil {
		t.Fatalf("DeleteProblem returned error: %v", err)
	}
	if !repo.deleteCalled || repo.deleteID != 42 {
		t.Fatal("repository delete was not called")
	}
}

func validCreateInput() CreateProblemInput {
	return CreateProblemInput{
		Title:         "A + B",
		Statement:     "Compute $A+B$.",
		TimeLimitMS:   1000,
		MemoryLimitKB: 262144,
		Visibility:    domain.ProblemVisibilityPrivate,
	}
}

func authorActor() auth.Actor {
	return auth.Actor{UserID: problemAuthorID, Role: domain.UserRoleAuthor}
}

type fakeProblemRepository struct {
	problem      domain.Problem
	listCalled   bool
	listParams   repository.ListProblemsParams
	createCalled bool
	createParams repository.CreateProblemParams
	updateCalled bool
	updateParams repository.UpdateProblemParams
	deleteCalled bool
	deleteID     int64
}

func newFakeProblemRepository() *fakeProblemRepository {
	return &fakeProblemRepository{problem: domain.Problem{
		ID:            42,
		Title:         "A + B",
		Statement:     "Compute.",
		TimeLimitMS:   1000,
		MemoryLimitKB: 262144,
		Visibility:    domain.ProblemVisibilityPrivate,
		CreatedBy:     problemAuthorID,
	}}
}

func (r *fakeProblemRepository) ListPublic(_ context.Context, params repository.ListProblemsParams) ([]domain.Problem, error) {
	r.listCalled = true
	r.listParams = params
	return []domain.Problem{}, nil
}

func (r *fakeProblemRepository) FindPublicByID(_ context.Context, id int64) (*domain.Problem, error) {
	problem := r.problem
	problem.ID = id
	return &problem, nil
}

func (r *fakeProblemRepository) FindByID(_ context.Context, id int64) (*domain.Problem, error) {
	problem := r.problem
	problem.ID = id
	return &problem, nil
}

func (r *fakeProblemRepository) Create(_ context.Context, params repository.CreateProblemParams) (*domain.Problem, error) {
	r.createCalled = true
	r.createParams = params
	problem := r.problem
	problem.Title = params.Title
	problem.CreatedBy = params.CreatedBy
	problem.Visibility = params.Visibility
	return &problem, nil
}

func (r *fakeProblemRepository) Update(_ context.Context, params repository.UpdateProblemParams) (*domain.Problem, error) {
	r.updateCalled = true
	r.updateParams = params
	problem := r.problem
	problem.Title = params.Title
	problem.Statement = params.Statement
	problem.TimeLimitMS = params.TimeLimitMS
	problem.MemoryLimitKB = params.MemoryLimitKB
	problem.Visibility = params.Visibility
	return &problem, nil
}

func (r *fakeProblemRepository) Delete(_ context.Context, id int64) error {
	r.deleteCalled = true
	r.deleteID = id
	return nil
}

func (r *fakeProblemRepository) SetCurrentTestCasesKey(_ context.Context, _ int64, key string) error {
	r.problem.CurrentTestCasesKey = key
	return nil
}

type fakeTestCaseCounter struct {
	count     int
	problemID int64
}

func (r *fakeTestCaseCounter) CountByProblemID(_ context.Context, problemID int64) (int, error) {
	r.problemID = problemID
	return r.count, nil
}

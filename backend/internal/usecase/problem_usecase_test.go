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

func TestCreateProblem(t *testing.T) {
	repo := &fakeProblemRepository{}
	uc := NewProblemUsecase(repo)
	actor := auth.Actor{UserID: "author-id", Role: domain.UserRoleAuthor}

	problem, err := uc.CreateProblem(context.Background(), actor, validCreateInput())
	if err != nil {
		t.Fatalf("CreateProblem returned error: %v", err)
	}
	if problem.ID != 42 {
		t.Fatalf("id = %d, want 42", problem.ID)
	}
	if repo.createParams.CreatedBy != actor.UserID {
		t.Fatalf("created_by = %q, want %q", repo.createParams.CreatedBy, actor.UserID)
	}
	if len(repo.createParams.TestCases) != 2 {
		t.Fatalf("test cases = %d, want 2", len(repo.createParams.TestCases))
	}
	if !repo.createParams.TestCases[0].IsSample {
		t.Fatal("sample flag was not preserved")
	}
	for _, testCase := range repo.createParams.TestCases {
		if testCase.Score != 1 {
			t.Fatalf("score = %d, want 1", testCase.Score)
		}
	}
}

func TestCreateProblemAdmin(t *testing.T) {
	repo := &fakeProblemRepository{}
	uc := NewProblemUsecase(repo)
	actor := auth.Actor{UserID: "admin-id", Role: domain.UserRoleAdmin}

	_, err := uc.CreateProblem(context.Background(), actor, validCreateInput())
	if err != nil {
		t.Fatalf("CreateProblem returned error: %v", err)
	}
}

func TestCreateProblemVisibilities(t *testing.T) {
	for _, visibility := range []domain.ProblemVisibility{
		domain.ProblemVisibilityPublic,
		domain.ProblemVisibilityPrivate,
		domain.ProblemVisibilityGroup,
	} {
		t.Run(string(visibility), func(t *testing.T) {
			repo := &fakeProblemRepository{}
			uc := NewProblemUsecase(repo)
			input := validCreateInput()
			input.Visibility = visibility

			_, err := uc.CreateProblem(context.Background(), auth.Actor{UserID: "author-id", Role: domain.UserRoleAuthor}, input)
			if err != nil {
				t.Fatalf("CreateProblem returned error: %v", err)
			}
			if repo.createParams.Visibility != visibility {
				t.Fatalf("visibility = %q, want %q", repo.createParams.Visibility, visibility)
			}
		})
	}
}

func TestCreateProblemForbidden(t *testing.T) {
	repo := &fakeProblemRepository{}
	uc := NewProblemUsecase(repo)
	actor := auth.Actor{UserID: "user-id", Role: domain.UserRoleUser}

	_, err := uc.CreateProblem(context.Background(), actor, validCreateInput())
	if !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if repo.createCalled {
		t.Fatal("repository should not be called")
	}
}

func TestCreateProblemValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CreateProblemInput)
	}{
		{name: "title", mutate: func(input *CreateProblemInput) { input.Title = " " }},
		{name: "statement", mutate: func(input *CreateProblemInput) { input.Statement = "" }},
		{name: "time_limit", mutate: func(input *CreateProblemInput) { input.TimeLimitMS = 0 }},
		{name: "memory_limit", mutate: func(input *CreateProblemInput) { input.MemoryLimitKB = 0 }},
		{name: "visibility", mutate: func(input *CreateProblemInput) { input.Visibility = "draft" }},
		{name: "test_cases", mutate: func(input *CreateProblemInput) { input.TestCases = nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeProblemRepository{}
			uc := NewProblemUsecase(repo)
			input := validCreateInput()
			tt.mutate(&input)

			_, err := uc.CreateProblem(context.Background(), auth.Actor{UserID: "author-id", Role: domain.UserRoleAuthor}, input)
			if !errors.Is(err, apperror.ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
			if repo.createCalled {
				t.Fatal("repository should not be called")
			}
		})
	}
}

func validCreateInput() CreateProblemInput {
	return CreateProblemInput{
		Title:         "A + B",
		Statement:     "Compute $A+B$.",
		TimeLimitMS:   1000,
		MemoryLimitKB: 262144,
		Visibility:    domain.ProblemVisibilityPrivate,
		TestCases: []CreateTestCaseInput{
			{Input: "1 2\n", ExpectedOutput: "3\n", IsSample: true},
			{Input: "2 3\n", ExpectedOutput: "5\n", IsSample: false},
		},
	}
}

type fakeProblemRepository struct {
	listCalled   bool
	listParams   repository.ListProblemsParams
	findCalled   bool
	createCalled bool
	createParams repository.CreateProblemParams
}

func (r *fakeProblemRepository) Create(_ context.Context, params repository.CreateProblemParams) (*domain.Problem, error) {
	r.createCalled = true
	r.createParams = params
	return &domain.Problem{ID: 42}, nil
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

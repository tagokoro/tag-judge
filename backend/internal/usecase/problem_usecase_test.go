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

func TestListMyProblemsListsTheActorsProblems(t *testing.T) {
	repo := newFakeProblemRepository()
	uc := NewProblemUsecase(repo, &fakeTestCaseCounter{})

	problems, err := uc.ListMyProblems(context.Background(), authorActor(), ListProblemsInput{})
	if err != nil {
		t.Fatalf("ListMyProblems returned error: %v", err)
	}
	if len(problems) != 1 {
		t.Fatalf("problems = %+v", problems)
	}
	want := repository.ListProblemsByCreatorParams{CreatedBy: problemAuthorID, Limit: defaultProblemLimit}
	if repo.listByCreatorParams == nil || *repo.listByCreatorParams != want {
		t.Fatalf("params = %+v, want %+v", repo.listByCreatorParams, want)
	}
}

func TestListMyProblemsRejectsAnonymousAndBadPaging(t *testing.T) {
	repo := newFakeProblemRepository()
	uc := NewProblemUsecase(repo, &fakeTestCaseCounter{})

	if _, err := uc.ListMyProblems(context.Background(), auth.Actor{}, ListProblemsInput{}); !errors.Is(err, apperror.ErrUnauthorized) {
		t.Fatalf("anonymous err = %v, want unauthorized", err)
	}
	for _, input := range []ListProblemsInput{{Limit: 0, LimitProvided: true}, {Limit: 101, LimitProvided: true}, {Offset: -1}} {
		if _, err := uc.ListMyProblems(context.Background(), authorActor(), input); !errors.Is(err, apperror.ErrInvalidInput) {
			t.Fatalf("input %+v err = %v, want invalid input", input, err)
		}
	}
	if repo.listByCreatorParams != nil {
		t.Fatal("repository must not be called for rejected input")
	}
}

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

	_, err := uc.GetProblem(context.Background(), auth.Actor{}, 0)
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

// A private (draft) or group problem is shown only to its creator and an
// admin; everyone else, signed in or not, gets the same 404 as a missing one.
func TestGetProblemVisibility(t *testing.T) {
	anonymous := auth.Actor{}
	creator := authorActor()
	other := auth.Actor{UserID: problemUserID, Role: domain.UserRoleUser}
	admin := auth.Actor{UserID: problemAdminID, Role: domain.UserRoleAdmin}
	tests := []struct {
		visibility domain.ProblemVisibility
		actor      auth.Actor
		visible    bool
	}{
		{domain.ProblemVisibilityPublic, anonymous, true},
		{domain.ProblemVisibilityPublic, other, true},
		{domain.ProblemVisibilityPublic, creator, true},
		{domain.ProblemVisibilityPrivate, anonymous, false},
		{domain.ProblemVisibilityPrivate, other, false},
		{domain.ProblemVisibilityPrivate, creator, true},
		{domain.ProblemVisibilityPrivate, admin, true},
		{domain.ProblemVisibilityGroup, other, false},
		{domain.ProblemVisibilityGroup, creator, true},
	}
	for _, tt := range tests {
		t.Run(string(tt.visibility)+"/"+string(tt.actor.Role), func(t *testing.T) {
			repo := newFakeProblemRepository()
			repo.problem.Visibility = tt.visibility
			testCases := &fakeTestCaseCounter{samples: []domain.ProblemSample{{Input: "1 2\n", Output: "3\n"}}}
			uc := NewProblemUsecase(repo, testCases)

			got, err := uc.GetProblem(context.Background(), tt.actor, 42)
			if tt.visible {
				if err != nil {
					t.Fatalf("GetProblem err = %v, want visible", err)
				}
				if got.ID != 42 || len(got.Samples) != 1 || testCases.samplesProblemID != 42 {
					t.Fatalf("detail = %+v, samples read for problem %d", got, testCases.samplesProblemID)
				}
				return
			}
			requireAppError(t, err, apperror.ErrNotFound, "PROBLEM_NOT_FOUND", "problem not found")
			if testCases.samplesProblemID != 0 {
				t.Fatal("samples must not be read for a hidden problem")
			}
		})
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

// Publishing is one-way, for the creator and an admin alike, so a public
// submission never becomes hidden again.
func TestUpdateProblemCannotUnpublish(t *testing.T) {
	for _, actor := range []auth.Actor{authorActor(), {UserID: problemAdminID, Role: domain.UserRoleAdmin}} {
		for _, visibility := range []domain.ProblemVisibility{domain.ProblemVisibilityPrivate, domain.ProblemVisibilityGroup} {
			repo := newFakeProblemRepository()
			repo.problem.Visibility = domain.ProblemVisibilityPublic
			uc := NewProblemUsecase(repo, &fakeTestCaseCounter{count: 1})

			_, err := uc.UpdateProblem(context.Background(), actor, 42, UpdateProblemInput{Visibility: &visibility})
			requireAppError(t, err, apperror.ErrConflict, "PROBLEM_ALREADY_PUBLIC", "a public problem cannot be made private or group")
			if repo.updateCalled {
				t.Fatal("repository should not be called")
			}
		}
	}
}

func TestUpdatePublicProblemKeepsItPublic(t *testing.T) {
	repo := newFakeProblemRepository()
	repo.problem.Visibility = domain.ProblemVisibilityPublic
	uc := NewProblemUsecase(repo, &fakeTestCaseCounter{count: 1})
	title := "Updated"
	visibility := domain.ProblemVisibilityPublic

	if _, err := uc.UpdateProblem(context.Background(), authorActor(), 42, UpdateProblemInput{Title: &title}); err != nil {
		t.Fatalf("title update err = %v", err)
	}
	if _, err := uc.UpdateProblem(context.Background(), authorActor(), 42, UpdateProblemInput{Visibility: &visibility}); err != nil {
		t.Fatalf("public to public err = %v", err)
	}
}

// Someone else's draft is hidden even from another author, like on a read.
func TestUpdateOthersPrivateProblemIsNotFound(t *testing.T) {
	repo := newFakeProblemRepository()
	repo.problem.CreatedBy = problemAdminID
	uc := NewProblemUsecase(repo, &fakeTestCaseCounter{})
	title := "Updated"

	_, err := uc.UpdateProblem(context.Background(), authorActor(), 42, UpdateProblemInput{Title: &title})
	requireAppError(t, err, apperror.ErrNotFound, "PROBLEM_NOT_FOUND", "problem not found")
	if err := uc.DeleteProblem(context.Background(), authorActor(), 42); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("delete err = %v, want ErrNotFound", err)
	}
}

func TestUpdateProblemOwner(t *testing.T) {
	repo := newFakeProblemRepository()
	repo.problem.CreatedBy = problemAdminID
	repo.problem.Visibility = domain.ProblemVisibilityPublic
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
	problem             domain.Problem
	listCalled          bool
	listParams          repository.ListProblemsParams
	listByCreatorParams *repository.ListProblemsByCreatorParams
	createCalled        bool
	createParams        repository.CreateProblemParams
	updateCalled        bool
	updateParams        repository.UpdateProblemParams
	deleteCalled        bool
	deleteID            int64
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

func (r *fakeProblemRepository) ListByCreator(_ context.Context, params repository.ListProblemsByCreatorParams) ([]domain.Problem, error) {
	r.listByCreatorParams = &params
	return []domain.Problem{r.problem}, nil
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
	count            int
	problemID        int64
	samples          []domain.ProblemSample
	samplesProblemID int64
}

func (r *fakeTestCaseCounter) ListSamplesByProblemID(_ context.Context, problemID int64) ([]domain.ProblemSample, error) {
	r.samplesProblemID = problemID
	return r.samples, nil
}

func (r *fakeTestCaseCounter) CountByProblemID(_ context.Context, problemID int64) (int, error) {
	r.problemID = problemID
	return r.count, nil
}

func requireAppError(t *testing.T, err error, wantSentinel error, wantCode, wantMessage string) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.Is(err, wantSentinel) || !errors.As(err, &appErr) {
		t.Fatalf("err = %v, want %v", err, wantSentinel)
	}
	if appErr.Code != wantCode || appErr.Message != wantMessage {
		t.Fatalf("error = %s %q, want %s %q", appErr.Code, appErr.Message, wantCode, wantMessage)
	}
}

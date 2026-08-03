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

func TestCreateTestCase(t *testing.T) {
	problems := newFakeProblemRepository()
	testCases := newFakeTestCaseRepository()
	uc := NewTestCaseUsecase(problems, testCases)

	testCase, err := uc.Create(context.Background(), authorActor(), 42, CreateTestCaseInput{
		Input: "1 2\n", ExpectedOutput: "3\n", IsSample: true,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if testCase.Score != 1 || !testCases.createParams.IsSample || testCases.createParams.ProblemID != 42 {
		t.Fatalf("test case = %+v, params = %+v", testCase, testCases.createParams)
	}
}

func TestListTestCasesOwner(t *testing.T) {
	problems := newFakeProblemRepository()
	problems.problem.CreatedBy = problemAdminID
	uc := NewTestCaseUsecase(problems, newFakeTestCaseRepository())

	_, err := uc.List(context.Background(), authorActor(), 42)
	if !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestListTestCasesAdmin(t *testing.T) {
	problems := newFakeProblemRepository()
	problems.problem.CreatedBy = problemAuthorID
	uc := NewTestCaseUsecase(problems, newFakeTestCaseRepository())
	admin := auth.Actor{UserID: problemAdminID, Role: domain.UserRoleAdmin}

	if _, err := uc.List(context.Background(), admin, 42); err != nil {
		t.Fatalf("List returned error: %v", err)
	}
}

func TestUpdateTestCase(t *testing.T) {
	testCases := newFakeTestCaseRepository()
	uc := NewTestCaseUsecase(newFakeProblemRepository(), testCases)
	output := "4\n"

	testCase, err := uc.Update(context.Background(), authorActor(), 42, 7, UpdateTestCaseInput{ExpectedOutput: &output})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if testCase.ExpectedOutput != output || testCases.updateParams.Input != "1 2\n" {
		t.Fatalf("test case = %+v, params = %+v", testCase, testCases.updateParams)
	}
}

func TestUpdateTestCaseEmpty(t *testing.T) {
	testCases := newFakeTestCaseRepository()
	uc := NewTestCaseUsecase(newFakeProblemRepository(), testCases)

	_, err := uc.Update(context.Background(), authorActor(), 42, 7, UpdateTestCaseInput{})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestDeleteTestCase(t *testing.T) {
	testCases := newFakeTestCaseRepository()
	uc := NewTestCaseUsecase(newFakeProblemRepository(), testCases)

	if err := uc.Delete(context.Background(), authorActor(), 42, 7); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if !testCases.deleteCalled {
		t.Fatal("repository delete was not called")
	}
}

func TestDeletePublicLastCase(t *testing.T) {
	problems := newFakeProblemRepository()
	problems.problem.Visibility = domain.ProblemVisibilityPublic
	testCases := newFakeTestCaseRepository()
	testCases.count = 1
	uc := NewTestCaseUsecase(problems, testCases)

	err := uc.Delete(context.Background(), authorActor(), 42, 7)
	if !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
	if testCases.deleteCalled {
		t.Fatal("repository delete should not be called")
	}
}

type fakeTestCaseRepository struct {
	testCase     domain.TestCase
	count        int
	createParams repository.CreateTestCaseParams
	updateParams repository.UpdateTestCaseParams
	deleteCalled bool
}

func newFakeTestCaseRepository() *fakeTestCaseRepository {
	return &fakeTestCaseRepository{
		testCase: domain.TestCase{
			ID: 7, ProblemID: 42, Input: "1 2\n", ExpectedOutput: "3\n", IsSample: true, Score: 1,
		},
		count: 1,
	}
}

func (r *fakeTestCaseRepository) CountByProblemID(_ context.Context, _ int64) (int, error) {
	return r.count, nil
}

func (r *fakeTestCaseRepository) ListByProblemID(_ context.Context, _ int64) ([]domain.TestCase, error) {
	return []domain.TestCase{r.testCase}, nil
}

func (r *fakeTestCaseRepository) FindByID(_ context.Context, _, _ int64) (*domain.TestCase, error) {
	testCase := r.testCase
	return &testCase, nil
}

func (r *fakeTestCaseRepository) Create(_ context.Context, params repository.CreateTestCaseParams) (*domain.TestCase, error) {
	r.createParams = params
	testCase := r.testCase
	testCase.ProblemID = params.ProblemID
	testCase.Input = params.Input
	testCase.ExpectedOutput = params.ExpectedOutput
	testCase.IsSample = params.IsSample
	testCase.Score = params.Score
	return &testCase, nil
}

func (r *fakeTestCaseRepository) Update(_ context.Context, params repository.UpdateTestCaseParams) (*domain.TestCase, error) {
	r.updateParams = params
	testCase := r.testCase
	testCase.Input = params.Input
	testCase.ExpectedOutput = params.ExpectedOutput
	testCase.IsSample = params.IsSample
	return &testCase, nil
}

func (r *fakeTestCaseRepository) Delete(_ context.Context, _, _ int64) error {
	r.deleteCalled = true
	return nil
}

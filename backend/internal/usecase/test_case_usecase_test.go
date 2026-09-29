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
	storage := &fakeTestCaseArtifactStorage{key: "problems/42/testcases/first.json"}
	uc := NewTestCaseUsecase(problems, testCases, storage)

	testCase, err := uc.Create(context.Background(), authorActor(), 42, CreateTestCaseInput{
		Input: "1 2\n", ExpectedOutput: "3\n", IsSample: true,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if testCase.Score != 1 || !testCases.createParams.IsSample || testCases.createParams.ProblemID != 42 {
		t.Fatalf("test case = %+v, params = %+v", testCase, testCases.createParams)
	}
	if !storage.putCalled || problems.problem.CurrentTestCasesKey != storage.key {
		t.Fatalf("snapshot storage=%+v, key=%q", storage, problems.problem.CurrentTestCasesKey)
	}
}

func TestListTestCasesOwner(t *testing.T) {
	problems := newFakeProblemRepository()
	problems.problem.CreatedBy = problemAdminID
	uc := newTestCaseUsecase(problems, newFakeTestCaseRepository())

	_, err := uc.List(context.Background(), authorActor(), 42)
	if !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestListTestCasesAdmin(t *testing.T) {
	problems := newFakeProblemRepository()
	problems.problem.CreatedBy = problemAuthorID
	uc := newTestCaseUsecase(problems, newFakeTestCaseRepository())
	admin := auth.Actor{UserID: problemAdminID, Role: domain.UserRoleAdmin}

	if _, err := uc.List(context.Background(), admin, 42); err != nil {
		t.Fatalf("List returned error: %v", err)
	}
}

func TestUpdateTestCase(t *testing.T) {
	problems := newFakeProblemRepository()
	testCases := newFakeTestCaseRepository()
	storage := &fakeTestCaseArtifactStorage{key: "problems/42/testcases/updated.json"}
	uc := NewTestCaseUsecase(problems, testCases, storage)
	output := "4\n"

	testCase, err := uc.Update(context.Background(), authorActor(), 42, 7, UpdateTestCaseInput{ExpectedOutput: &output})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if testCase.ExpectedOutput != output || testCases.updateParams.Input != "1 2\n" {
		t.Fatalf("test case = %+v, params = %+v", testCase, testCases.updateParams)
	}
	if !storage.putCalled || problems.problem.CurrentTestCasesKey != storage.key {
		t.Fatalf("snapshot storage=%+v, key=%q", storage, problems.problem.CurrentTestCasesKey)
	}
}

func TestUpdateTestCaseEmpty(t *testing.T) {
	testCases := newFakeTestCaseRepository()
	uc := newTestCaseUsecase(newFakeProblemRepository(), testCases)

	_, err := uc.Update(context.Background(), authorActor(), 42, 7, UpdateTestCaseInput{})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestDeleteTestCase(t *testing.T) {
	problems := newFakeProblemRepository()
	problems.problem.CurrentTestCasesKey = "problems/42/testcases/previous.json"
	testCases := newFakeTestCaseRepository()
	storage := &fakeTestCaseArtifactStorage{key: "unexpected"}
	uc := NewTestCaseUsecase(problems, testCases, storage)

	if err := uc.Delete(context.Background(), authorActor(), 42, 7); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if !testCases.deleteCalled {
		t.Fatal("repository delete was not called")
	}
	if storage.putCalled || problems.problem.CurrentTestCasesKey != "" {
		t.Fatalf("snapshot storage=%+v, key=%q", storage, problems.problem.CurrentTestCasesKey)
	}
}

func TestDeletePublicLastCase(t *testing.T) {
	problems := newFakeProblemRepository()
	problems.problem.Visibility = domain.ProblemVisibilityPublic
	testCases := newFakeTestCaseRepository()
	testCases.count = 1
	uc := newTestCaseUsecase(problems, testCases)

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

type fakeTestCaseArtifactStorage struct {
	putCalled bool
	params    repository.PutTestCasesParams
	key       string
	err       error
}

func (s *fakeTestCaseArtifactStorage) PutTestCases(_ context.Context, params repository.PutTestCasesParams) (string, error) {
	s.putCalled = true
	s.params = params
	return s.key, s.err
}

func newTestCaseUsecase(problems *fakeProblemRepository, testCases *fakeTestCaseRepository) *TestCaseUsecase {
	return NewTestCaseUsecase(problems, testCases, &fakeTestCaseArtifactStorage{key: "problems/42/testcases/current.json"})
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
	if r.count == 0 {
		return []domain.TestCase{}, nil
	}
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
	r.testCase = testCase
	return &testCase, nil
}

func (r *fakeTestCaseRepository) Update(_ context.Context, params repository.UpdateTestCaseParams) (*domain.TestCase, error) {
	r.updateParams = params
	testCase := r.testCase
	testCase.Input = params.Input
	testCase.ExpectedOutput = params.ExpectedOutput
	testCase.IsSample = params.IsSample
	r.testCase = testCase
	return &testCase, nil
}

func (r *fakeTestCaseRepository) Delete(_ context.Context, _, _ int64) error {
	r.deleteCalled = true
	r.count = 0
	return nil
}

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

const testUserID = "00000000-0000-4000-8000-000000000001"
const testCasesArtifactKey = "problems/42/testcases/content-hash.json"

func TestCreateSubmission(t *testing.T) {
	submissions := &fakeSubmissionRepository{}
	problems := &submissionProblemRepository{}
	storage := &fakeSubmissionSourceCodeStorage{}
	uc := NewSubmissionUsecase(submissions, problems, storage)

	got, err := uc.Create(context.Background(), CreateSubmissionInput{
		Actor:      submissionTestActor(domain.UserRoleUser),
		ProblemID:  42,
		Language:   domain.SubmissionLanguagePython,
		SourceCode: "print(42)",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if problems.findID != 42 {
		t.Fatalf("problem id = %d, want 42", problems.findID)
	}
	if submissions.createWithOutboxParams.UserID != testUserID || submissions.createWithOutboxParams.ProblemID != 42 {
		t.Fatalf("create params = %+v", submissions.createWithOutboxParams)
	}
	if submissions.createWithOutboxParams.SourceCodeKey == "" {
		t.Fatalf("source code key is empty")
	}
	if submissions.createWithOutboxParams.TestCasesKey != testCasesArtifactKey {
		t.Fatalf("test cases key = %q, want %q", submissions.createWithOutboxParams.TestCasesKey, testCasesArtifactKey)
	}
	if submissions.createWithOutboxParams.Status != domain.SubmissionStatusPending || submissions.createWithOutboxParams.Score != 0 {
		t.Fatalf("create params = %+v", submissions.createWithOutboxParams)
	}
	if !storage.putCalled || storage.putParams.SourceCode != "print(42)" {
		t.Fatalf("storage params = %+v", storage.putParams)
	}
	if !submissions.createWithOutboxCalled || submissions.createWithOutboxParams.TimeLimitMS != 1000 {
		t.Fatalf("outbox params = %+v", submissions.createWithOutboxParams)
	}
	if got.Status != domain.SubmissionStatusPending || got.Score != 0 {
		t.Fatalf("submission = %+v", got)
	}
}

func TestCreateSubmissionLanguage(t *testing.T) {
	submissions := &fakeSubmissionRepository{}
	problems := &submissionProblemRepository{}
	uc := newSubmissionUsecaseForTest(submissions, problems)

	_, err := uc.Create(context.Background(), CreateSubmissionInput{
		Actor:      submissionTestActor(domain.UserRoleUser),
		ProblemID:  1,
		Language:   "ruby",
		SourceCode: "puts 1",
	})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if problems.findCalled || submissions.createCalled {
		t.Fatal("repositories should not be called")
	}
}

func TestCreateSubmissionProblem(t *testing.T) {
	submissions := &fakeSubmissionRepository{}
	problems := &submissionProblemRepository{findErr: apperror.NotFound("PROBLEM_NOT_FOUND", "problem not found")}
	uc := newSubmissionUsecaseForTest(submissions, problems)

	_, err := uc.Create(context.Background(), CreateSubmissionInput{
		Actor:      submissionTestActor(domain.UserRoleUser),
		ProblemID:  99,
		Language:   domain.SubmissionLanguagePython,
		SourceCode: "print(1)",
	})
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if submissions.createCalled {
		t.Fatal("submission repository should not be called")
	}
}

func TestCreateSubmissionRequiresCurrentTestCaseSnapshot(t *testing.T) {
	submissions := &fakeSubmissionRepository{}
	problems := &submissionProblemRepository{withoutSnapshot: true}
	storage := &fakeSubmissionSourceCodeStorage{}
	uc := NewSubmissionUsecase(submissions, problems, storage)

	_, err := uc.Create(context.Background(), CreateSubmissionInput{
		Actor:      submissionTestActor(domain.UserRoleUser),
		ProblemID:  42,
		Language:   domain.SubmissionLanguagePython,
		SourceCode: "print(42)",
	})
	if !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
	if storage.putCalled || submissions.createCalled || submissions.createWithOutboxCalled {
		t.Fatalf("submission dependencies should not be called: storage=%v create=%v outbox=%v", storage.putCalled, submissions.createCalled, submissions.createWithOutboxCalled)
	}
}

func TestGetSubmissionOwner(t *testing.T) {
	submissions := &fakeSubmissionRepository{submission: domain.Submission{ID: 7, UserID: testUserID}}
	uc := newSubmissionUsecaseForTest(submissions, &submissionProblemRepository{})

	got, err := uc.Get(context.Background(), GetSubmissionInput{
		Actor:        submissionTestActor(domain.UserRoleAuthor),
		SubmissionID: 7,
	})
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.ID != 7 {
		t.Fatalf("id = %d, want 7", got.ID)
	}
}

func TestGetSubmissionAdmin(t *testing.T) {
	submissions := &fakeSubmissionRepository{submission: domain.Submission{ID: 7, UserID: "another-user"}}
	uc := newSubmissionUsecaseForTest(submissions, &submissionProblemRepository{})

	_, err := uc.Get(context.Background(), GetSubmissionInput{
		Actor:        submissionTestActor(domain.UserRoleAdmin),
		SubmissionID: 7,
	})
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
}

func TestGetSubmissionForbidden(t *testing.T) {
	submissions := &fakeSubmissionRepository{submission: domain.Submission{ID: 7, UserID: "another-user"}}
	uc := newSubmissionUsecaseForTest(submissions, &submissionProblemRepository{})

	_, err := uc.Get(context.Background(), GetSubmissionInput{
		Actor:        submissionTestActor(domain.UserRoleAuthor),
		SubmissionID: 7,
	})
	if !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestListSubmissionsDefaults(t *testing.T) {
	submissions := &fakeSubmissionRepository{}
	uc := newSubmissionUsecaseForTest(submissions, &submissionProblemRepository{})

	_, err := uc.List(context.Background(), ListSubmissionsInput{Actor: submissionTestActor(domain.UserRoleUser)})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if submissions.listParams.UserID != testUserID {
		t.Fatalf("user id = %q, want %q", submissions.listParams.UserID, testUserID)
	}
	if submissions.listParams.Limit != defaultSubmissionLimit || submissions.listParams.Offset != 0 {
		t.Fatalf("list params = %+v", submissions.listParams)
	}
}

func TestListSubmissionsLimit(t *testing.T) {
	submissions := &fakeSubmissionRepository{}
	uc := newSubmissionUsecaseForTest(submissions, &submissionProblemRepository{})

	_, err := uc.List(context.Background(), ListSubmissionsInput{
		Actor:         submissionTestActor(domain.UserRoleUser),
		Limit:         0,
		LimitProvided: true,
	})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if submissions.listCalled {
		t.Fatal("repository should not be called")
	}
}

func TestSubmissionAuthentication(t *testing.T) {
	uc := newSubmissionUsecaseForTest(&fakeSubmissionRepository{}, &submissionProblemRepository{})

	_, err := uc.List(context.Background(), ListSubmissionsInput{})
	if !errors.Is(err, apperror.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestSubmissionAuthenticationRejectsInvalidUserID(t *testing.T) {
	submissions := &fakeSubmissionRepository{}
	problems := &submissionProblemRepository{}
	storage := &fakeSubmissionSourceCodeStorage{}
	uc := NewSubmissionUsecase(submissions, problems, storage)

	_, err := uc.Create(context.Background(), CreateSubmissionInput{
		Actor:      auth.Actor{UserID: "not-a-uuid", Role: domain.UserRoleUser},
		ProblemID:  42,
		Language:   domain.SubmissionLanguagePython,
		SourceCode: "print(42)",
	})
	if !errors.Is(err, apperror.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	if problems.findCalled || storage.putCalled || submissions.createCalled || submissions.createWithOutboxCalled {
		t.Fatalf("dependencies should not be called: problem=%v storage=%v repo=%v outbox=%v", problems.findCalled, storage.putCalled, submissions.createCalled, submissions.createWithOutboxCalled)
	}
}

func TestCreateSubmissionDoesNotDependOnQueueAvailability(t *testing.T) {
	submissions := &fakeSubmissionRepository{}
	problems := &submissionProblemRepository{}
	storage := &fakeSubmissionSourceCodeStorage{}
	uc := NewSubmissionUsecase(submissions, problems, storage)

	_, err := uc.Create(context.Background(), CreateSubmissionInput{
		Actor:      submissionTestActor(domain.UserRoleUser),
		ProblemID:  42,
		Language:   domain.SubmissionLanguagePython,
		SourceCode: "print(42)",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if !storage.putCalled || !submissions.createWithOutboxCalled {
		t.Fatalf("storage=%v outbox=%v, want both called", storage.putCalled, submissions.createWithOutboxCalled)
	}
}

func submissionTestActor(role domain.UserRole) auth.Actor {
	return auth.Actor{UserID: testUserID, Role: role}
}

func newSubmissionUsecaseForTest(submissions repository.SubmissionRepository, problems repository.PublicProblemReader) *SubmissionUsecase {
	return NewSubmissionUsecase(
		submissions,
		problems,
		&fakeSubmissionSourceCodeStorage{},
	)
}

type fakeSubmissionRepository struct {
	createCalled           bool
	createParams           repository.CreateSubmissionParams
	createWithOutboxCalled bool
	createWithOutboxParams repository.CreateSubmissionWithOutboxParams
	findCalled             bool
	submission             domain.Submission
	findErr                error
	listCalled             bool
	listParams             repository.ListSubmissionsParams
}

func (r *fakeSubmissionRepository) Create(_ context.Context, params repository.CreateSubmissionParams) (*domain.Submission, error) {
	r.createCalled = true
	r.createParams = params
	return &domain.Submission{
		ID:            1,
		UserID:        params.UserID,
		ProblemID:     params.ProblemID,
		Language:      params.Language,
		SourceCodeKey: params.SourceCodeKey,
		TestCasesKey:  params.TestCasesKey,
		Status:        params.Status,
		Score:         params.Score,
	}, nil
}

func (r *fakeSubmissionRepository) CreateWithOutbox(_ context.Context, params repository.CreateSubmissionWithOutboxParams) (*domain.Submission, error) {
	r.createWithOutboxCalled = true
	r.createWithOutboxParams = params
	return &domain.Submission{
		ID:            1,
		UserID:        params.UserID,
		ProblemID:     params.ProblemID,
		Language:      params.Language,
		SourceCodeKey: params.SourceCodeKey,
		TestCasesKey:  params.TestCasesKey,
		Status:        params.Status,
		Score:         params.Score,
	}, nil
}

func (r *fakeSubmissionRepository) FindByID(_ context.Context, id int64) (*domain.Submission, error) {
	r.findCalled = true
	if r.findErr != nil {
		return nil, r.findErr
	}
	submission := r.submission
	submission.ID = id
	return &submission, nil
}

func (r *fakeSubmissionRepository) ListByUserID(_ context.Context, params repository.ListSubmissionsParams) ([]domain.Submission, error) {
	r.listCalled = true
	r.listParams = params
	return []domain.Submission{}, nil
}

type fakeSubmissionSourceCodeStorage struct {
	putCalled bool
	putParams repository.PutSourceCodeParams
	putErr    error
}

func (s *fakeSubmissionSourceCodeStorage) PutSourceCode(_ context.Context, params repository.PutSourceCodeParams) error {
	s.putCalled = true
	s.putParams = params
	return s.putErr
}

type submissionProblemRepository struct {
	findCalled      bool
	findID          int64
	findErr         error
	withoutSnapshot bool
}

func (r *submissionProblemRepository) FindPublicByID(_ context.Context, id int64) (*domain.Problem, error) {
	r.findCalled = true
	r.findID = id
	if r.findErr != nil {
		return nil, r.findErr
	}
	problem := &domain.Problem{ID: id, TimeLimitMS: 1000, CurrentTestCasesKey: testCasesArtifactKey}
	if r.withoutSnapshot {
		problem.CurrentTestCasesKey = ""
	}
	return problem, nil
}

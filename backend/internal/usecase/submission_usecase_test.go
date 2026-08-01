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

func TestCreateSubmission(t *testing.T) {
	submissions := &fakeSubmissionRepository{}
	problems := &submissionProblemRepository{}
	storage := &fakeSubmissionSourceCodeStorage{}
	dispatcher := &fakeSubmissionJudgeDispatcher{}
	uc := NewSubmissionUsecase(submissions, problems, storage, dispatcher)

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
	if submissions.createParams.UserID != testUserID || submissions.createParams.ProblemID != 42 {
		t.Fatalf("create params = %+v", submissions.createParams)
	}
	if submissions.createParams.SourceCodeKey == "" {
		t.Fatalf("source code key is empty")
	}
	if submissions.createParams.Status != domain.SubmissionStatusPending || submissions.createParams.Score != 0 {
		t.Fatalf("create params = %+v", submissions.createParams)
	}
	if !storage.putCalled || storage.putParams.SourceCode != "print(42)" {
		t.Fatalf("storage params = %+v", storage.putParams)
	}
	if !dispatcher.dispatchCalled {
		t.Fatal("judge dispatcher should be called")
	}
	if dispatcher.job.SubmissionID != "1" || dispatcher.job.SourceCodeKey != submissions.createParams.SourceCodeKey {
		t.Fatalf("judge job = %+v", dispatcher.job)
	}
	if dispatcher.job.TestCasesKey != "problems/42/testcases.json" || dispatcher.job.TimeLimitMS != 1000 {
		t.Fatalf("judge job = %+v", dispatcher.job)
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

func submissionTestActor(role domain.UserRole) auth.Actor {
	return auth.Actor{UserID: testUserID, Role: role}
}

func newSubmissionUsecaseForTest(submissions repository.SubmissionRepository, problems repository.ProblemRepository) *SubmissionUsecase {
	return NewSubmissionUsecase(
		submissions,
		problems,
		&fakeSubmissionSourceCodeStorage{},
		&fakeSubmissionJudgeDispatcher{},
	)
}

type fakeSubmissionRepository struct {
	createCalled bool
	createParams repository.CreateSubmissionParams
	findCalled   bool
	submission   domain.Submission
	findErr      error
	listCalled   bool
	listParams   repository.ListSubmissionsParams
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

type fakeSubmissionJudgeDispatcher struct {
	dispatchCalled bool
	job            repository.JudgeJob
	dispatchErr    error
}

func (d *fakeSubmissionJudgeDispatcher) Dispatch(_ context.Context, job repository.JudgeJob) error {
	d.dispatchCalled = true
	d.job = job
	return d.dispatchErr
}

type submissionProblemRepository struct {
	findCalled bool
	findID     int64
	findErr    error
}

func (r *submissionProblemRepository) ListPublic(_ context.Context, _ repository.ListProblemsParams) ([]domain.Problem, error) {
	return nil, nil
}

func (r *submissionProblemRepository) FindPublicByID(_ context.Context, id int64) (*domain.Problem, error) {
	r.findCalled = true
	r.findID = id
	if r.findErr != nil {
		return nil, r.findErr
	}
	return &domain.Problem{ID: id, TimeLimitMS: 1000}, nil
}

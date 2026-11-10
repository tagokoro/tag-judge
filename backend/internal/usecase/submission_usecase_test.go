package usecase

import (
	"context"
	"errors"
	"reflect"
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
	admission := &fakeAdmissionControl{createSubmissionResult: &domain.Submission{ID: 1, UserID: testUserID, ProblemID: 42, Status: domain.SubmissionStatusPending, Score: 0}}
	uc := NewSubmissionUsecase(submissions, problems, storage, admission, 255)

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
	if admission.createSubmissionParams.UserID != testUserID || admission.createSubmissionParams.ProblemID != 42 {
		t.Fatalf("create params = %+v", admission.createSubmissionParams)
	}
	if admission.createSubmissionParams.SourceCodeKey == "" {
		t.Fatalf("source code key is empty")
	}
	if admission.createSubmissionParams.TestCasesKey != testCasesArtifactKey {
		t.Fatalf("test cases key = %q, want %q", admission.createSubmissionParams.TestCasesKey, testCasesArtifactKey)
	}
	if admission.createSubmissionParams.Status != domain.SubmissionStatusPending || admission.createSubmissionParams.Score != 0 {
		t.Fatalf("create params = %+v", admission.createSubmissionParams)
	}
	if !storage.putCalled || storage.putParams.SourceCode != "print(42)" {
		t.Fatalf("storage params = %+v", storage.putParams)
	}
	if !admission.createSubmissionCalled || admission.createSubmissionParams.TimeLimitMS != 1000 {
		t.Fatalf("outbox params = %+v", admission.createSubmissionParams)
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
	uc := NewSubmissionUsecase(submissions, problems, storage, &fakeAdmissionControl{}, 255)

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
	if submissions.caseResultsCalled {
		t.Fatal("case results must not be read before authorization")
	}
}

func TestGetSubmissionCaseResults(t *testing.T) {
	runtime := 0
	memory := 1024
	want := []domain.SubmissionCaseResult{
		{TestCaseID: 10, Status: domain.SubmissionStatusAC, ExecutionTimeMS: &runtime, MemoryKB: &memory},
		{TestCaseID: 12, Status: domain.SubmissionStatusWA},
	}
	for _, tc := range []struct {
		name  string
		owner string
		role  domain.UserRole
	}{
		{"owner", testUserID, domain.UserRoleUser},
		{"admin", "another-user", domain.UserRoleAdmin},
	} {
		t.Run(tc.name, func(t *testing.T) {
			submissions := &fakeSubmissionRepository{
				submission:  domain.Submission{ID: 7, UserID: tc.owner, Status: domain.SubmissionStatusWA, Score: 1},
				caseResults: want,
			}
			uc := newSubmissionUsecaseForTest(submissions, &submissionProblemRepository{})
			got, err := uc.Get(context.Background(), GetSubmissionInput{Actor: submissionTestActor(tc.role), SubmissionID: 7})
			if err != nil {
				t.Fatalf("Get returned error: %v", err)
			}
			if got.ID != 7 || got.Status != domain.SubmissionStatusWA || got.Score != 1 || !reflect.DeepEqual(got.CaseResults, want) {
				t.Fatalf("detail = %+v, want WA with case results %+v", got, want)
			}
			if !submissions.caseResultsCalled || submissions.caseResultsID != 7 {
				t.Fatalf("queried case results for %d, want 7", submissions.caseResultsID)
			}
		})
	}
}

func TestGetUnfinishedSubmissionHasNoCaseResults(t *testing.T) {
	for _, status := range []domain.SubmissionStatus{domain.SubmissionStatusPending, domain.SubmissionStatusRunning} {
		t.Run(string(status), func(t *testing.T) {
			submissions := &fakeSubmissionRepository{
				submission: domain.Submission{UserID: testUserID, Status: status},
				// Simulate the worker committing after the submission status read.
				caseResults: []domain.SubmissionCaseResult{{TestCaseID: 1, Status: domain.SubmissionStatusAC}},
			}
			uc := newSubmissionUsecaseForTest(submissions, &submissionProblemRepository{})
			got, err := uc.Get(context.Background(), GetSubmissionInput{Actor: submissionTestActor(domain.UserRoleUser), SubmissionID: 7})
			if err != nil {
				t.Fatalf("Get returned error: %v", err)
			}
			if got.Status != status || got.CaseResults == nil || len(got.CaseResults) != 0 || submissions.caseResultsCalled {
				t.Fatalf("unfinished submission must have an empty result array without querying results: %+v", got)
			}
		})
	}
}

func TestGetSubmissionCaseResultsFailure(t *testing.T) {
	want := errors.New("database unavailable")
	submissions := &fakeSubmissionRepository{
		submission:     domain.Submission{UserID: testUserID, Status: domain.SubmissionStatusAC},
		caseResultsErr: want,
	}
	uc := newSubmissionUsecaseForTest(submissions, &submissionProblemRepository{})
	got, err := uc.Get(context.Background(), GetSubmissionInput{Actor: submissionTestActor(domain.UserRoleUser), SubmissionID: 7})
	if !errors.Is(err, want) || got != nil {
		t.Fatalf("Get = %+v, %v; want no partial success and the database error", got, err)
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
	uc := NewSubmissionUsecase(submissions, problems, storage, &fakeAdmissionControl{}, 255)

	_, err := uc.Create(context.Background(), CreateSubmissionInput{
		Actor:      auth.Actor{UserID: "not-a-uuid", Role: domain.UserRoleUser},
		ProblemID:  42,
		Language:   domain.SubmissionLanguagePython,
		SourceCode: "print(42)",
	})
	if !errors.Is(err, apperror.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	if problems.findCalled || storage.putCalled || submissions.createCalled || submissions.createWithOutboxCalled || uc.admission.(*fakeAdmissionControl).createSubmissionCalled {
		t.Fatalf("dependencies should not be called: problem=%v storage=%v repo=%v outbox=%v admission=%v", problems.findCalled, storage.putCalled, submissions.createCalled, submissions.createWithOutboxCalled, uc.admission.(*fakeAdmissionControl).createSubmissionCalled)
	}
}

func TestCreateSubmissionDoesNotDependOnQueueAvailability(t *testing.T) {
	submissions := &fakeSubmissionRepository{}
	problems := &submissionProblemRepository{}
	storage := &fakeSubmissionSourceCodeStorage{}
	admission := &fakeAdmissionControl{createSubmissionResult: &domain.Submission{ID: 1, UserID: testUserID, ProblemID: 42, Status: domain.SubmissionStatusPending, Score: 0}}
	uc := NewSubmissionUsecase(submissions, problems, storage, admission, 255)

	_, err := uc.Create(context.Background(), CreateSubmissionInput{
		Actor:      submissionTestActor(domain.UserRoleUser),
		ProblemID:  42,
		Language:   domain.SubmissionLanguagePython,
		SourceCode: "print(42)",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if !storage.putCalled || !admission.createSubmissionCalled {
		t.Fatalf("storage=%v admission=%v, want both called", storage.putCalled, admission.createSubmissionCalled)
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
		&fakeAdmissionControl{},
		255,
	)
}

type fakeSubmissionRepository struct {
	recentParams           *repository.ListRecentSubmissionsParams
	createCalled           bool
	createParams           repository.CreateSubmissionParams
	createWithOutboxCalled bool
	createWithOutboxParams repository.CreateSubmissionWithOutboxParams
	findCalled             bool
	submission             domain.Submission
	findErr                error
	listCalled             bool
	listParams             repository.ListSubmissionsParams
	caseResultsCalled      bool
	caseResultsID          int64
	caseResults            []domain.SubmissionCaseResult
	caseResultsErr         error
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

func (r *fakeSubmissionRepository) ListRecentPublic(_ context.Context, params repository.ListRecentSubmissionsParams) ([]domain.PublicSubmission, error) {
	r.recentParams = &params
	return []domain.PublicSubmission{{ID: 1, ProblemID: 2, ProblemTitle: "A + B"}}, nil
}

func (r *fakeSubmissionRepository) ListCaseResults(_ context.Context, submissionID int64) ([]domain.SubmissionCaseResult, error) {
	r.caseResultsCalled = true
	r.caseResultsID = submissionID
	return r.caseResults, r.caseResultsErr
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

type fakeAdmissionControl struct {
	createSubmissionCalled   bool
	createSubmissionParams   repository.CreateSubmissionWithOutboxParams
	createSubmissionKeyInfo  *repository.IdempotencyKeyInfo
	createSubmissionResult   *domain.Submission
	createSubmissionErr      error
	createCodeRunCalled      bool
	createCodeRunParams      repository.CreateCodeRunParams
	createCodeRunKeyInfo     *repository.IdempotencyKeyInfo
	createCodeRunTimeLimitMS int
	createCodeRunID          int64
	createCodeRunResult      *domain.CodeRun
	createCodeRunErr         error
	reserveCodeRunIDCalled   bool
	reserveCodeRunIDResult   int64
	reserveCodeRunIDErr      error
}

func (a *fakeAdmissionControl) CreateSubmission(_ context.Context, params repository.CreateSubmissionWithOutboxParams, idempotency *repository.IdempotencyKeyInfo) (*domain.Submission, error) {
	a.createSubmissionCalled = true
	a.createSubmissionParams = params
	a.createSubmissionKeyInfo = idempotency
	if a.createSubmissionResult != nil {
		return a.createSubmissionResult, a.createSubmissionErr
	}
	return &domain.Submission{
		ID:            1,
		UserID:        params.UserID,
		ProblemID:     params.ProblemID,
		Language:      params.Language,
		SourceCodeKey: params.SourceCodeKey,
		TestCasesKey:  params.TestCasesKey,
		Status:        params.Status,
		Score:         params.Score,
	}, a.createSubmissionErr
}

func (a *fakeAdmissionControl) ReserveCodeRunID(_ context.Context) (int64, error) {
	a.reserveCodeRunIDCalled = true
	if a.reserveCodeRunIDResult != 0 || a.reserveCodeRunIDErr != nil {
		return a.reserveCodeRunIDResult, a.reserveCodeRunIDErr
	}
	return 1, nil
}

func (a *fakeAdmissionControl) CreateCodeRun(_ context.Context, params repository.CreateCodeRunParams, idempotency *repository.IdempotencyKeyInfo, timeLimitMS int, codeRunID int64) (*domain.CodeRun, error) {
	a.createCodeRunCalled = true
	a.createCodeRunParams = params
	a.createCodeRunKeyInfo = idempotency
	a.createCodeRunTimeLimitMS = timeLimitMS
	a.createCodeRunID = codeRunID
	if a.createCodeRunResult != nil {
		return a.createCodeRunResult, a.createCodeRunErr
	}
	return &domain.CodeRun{
		ID:         codeRunID,
		UserID:     params.UserID,
		Language:   params.Language,
		SourceCode: params.SourceCode,
		Stdin:      params.Stdin,
		Status:     domain.CodeRunStatusPending,
	}, a.createCodeRunErr
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

func TestListRecentPublicSubmissionsNeedsNoActor(t *testing.T) {
	repo := &fakeSubmissionRepository{}
	uc := newSubmissionUsecaseForTest(repo, &submissionProblemRepository{})

	submissions, err := uc.ListRecentPublic(context.Background(), ListRecentSubmissionsInput{})
	if err != nil {
		t.Fatalf("ListRecentPublic returned error: %v", err)
	}
	if len(submissions) != 1 {
		t.Fatalf("submissions = %+v", submissions)
	}
	want := repository.ListRecentSubmissionsParams{Limit: defaultSubmissionLimit}
	if repo.recentParams == nil || *repo.recentParams != want {
		t.Fatalf("params = %+v, want %+v", repo.recentParams, want)
	}
}

func TestListRecentPublicSubmissionsRejectsBadPaging(t *testing.T) {
	repo := &fakeSubmissionRepository{}
	uc := newSubmissionUsecaseForTest(repo, &submissionProblemRepository{})

	for _, input := range []ListRecentSubmissionsInput{{Limit: 0, LimitProvided: true}, {Limit: 101, LimitProvided: true}, {Offset: -1}} {
		if _, err := uc.ListRecentPublic(context.Background(), input); !errors.Is(err, apperror.ErrInvalidInput) {
			t.Fatalf("input %+v err = %v, want invalid input", input, err)
		}
	}
	if repo.recentParams != nil {
		t.Fatal("repository must not be called for rejected input")
	}
}

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
	codeRunUserID  = "00000000-0000-4000-8000-000000000001"
	codeRunOtherID = "00000000-0000-4000-8000-000000000002"
)

func TestCreateCodeRun(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	storage := &fakeCodeRunStorage{}
	dispatcher := &fakeCodeRunDispatcher{}
	uc := NewCodeRunUsecase(repo, storage, dispatcher, 2500)

	result, err := uc.Create(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), CreateCodeRunInput{
		Language:   domain.ExecutionLanguagePython,
		SourceCode: "print(input())",
		Stdin:      "hello",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if result.Status != domain.CodeRunStatusPending {
		t.Fatalf("status = %q, want pending", result.Status)
	}
	if repo.createParams.UserID != codeRunUserID || repo.createParams.Stdin != "hello" {
		t.Fatalf("params = %+v", repo.createParams)
	}
	if storage.params.Key != "code-runs/00000000-0000-4000-8000-000000000001/1/main.py" {
		t.Fatalf("source key = %q", storage.params.Key)
	}
	if storage.params.SourceCode != "print(input())" {
		t.Fatalf("source code = %q", storage.params.SourceCode)
	}

	job := dispatcher.job
	if job.Reference().Kind() != repository.ExecutionReferenceCodeRun || job.Reference().ID() != "1" {
		t.Fatalf("reference = %s/%s", job.Reference().Kind(), job.Reference().ID())
	}
	stdin, ok := job.Mode().Stdin()
	if !ok || stdin.Stdin != "hello" || stdin.TimeLimitMS != 2500 {
		t.Fatalf("stdin execution = %+v, ok = %v", stdin, ok)
	}
	if job.SourceCodeKey() != storage.params.Key || job.Language() != domain.ExecutionLanguagePython {
		t.Fatalf("job source/language = %q/%q", job.SourceCodeKey(), job.Language())
	}
}

func TestCreateCodeRunLanguage(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	storage := &fakeCodeRunStorage{}
	dispatcher := &fakeCodeRunDispatcher{}
	uc := NewCodeRunUsecase(repo, storage, dispatcher, 2500)

	_, err := uc.Create(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), CreateCodeRunInput{
		Language:   domain.ExecutionLanguage("go"),
		SourceCode: "package main",
	})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	assertCodeRunCreateNotStarted(t, repo, storage, dispatcher)
}

func TestCreateCodeRunSource(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	storage := &fakeCodeRunStorage{}
	dispatcher := &fakeCodeRunDispatcher{}
	uc := NewCodeRunUsecase(repo, storage, dispatcher, 2500)

	_, err := uc.Create(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), CreateCodeRunInput{
		Language:   domain.ExecutionLanguagePython,
		SourceCode: "  ",
	})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	assertCodeRunCreateNotStarted(t, repo, storage, dispatcher)
}

func TestCreateCodeRunActor(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	storage := &fakeCodeRunStorage{}
	dispatcher := &fakeCodeRunDispatcher{}
	uc := NewCodeRunUsecase(repo, storage, dispatcher, 2500)

	_, err := uc.Create(context.Background(), codeRunActor("invalid", domain.UserRoleUser), CreateCodeRunInput{
		Language:   domain.ExecutionLanguagePython,
		SourceCode: "print(1)",
	})
	if !errors.Is(err, apperror.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	assertCodeRunCreateNotStarted(t, repo, storage, dispatcher)
}

func TestCreateCodeRunStorage(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	storageErr := errors.New("storage failed")
	storage := &fakeCodeRunStorage{err: storageErr}
	dispatcher := &fakeCodeRunDispatcher{}
	uc := NewCodeRunUsecase(repo, storage, dispatcher, 2500)

	_, err := uc.Create(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), CreateCodeRunInput{
		Language:   domain.ExecutionLanguagePython,
		SourceCode: "print(1)",
	})
	if !errors.Is(err, storageErr) {
		t.Fatalf("err = %v, want storage error", err)
	}
	if !repo.createCalled || !storage.called || dispatcher.called {
		t.Fatalf("calls = repo:%v storage:%v dispatcher:%v", repo.createCalled, storage.called, dispatcher.called)
	}
}

func TestCreateCodeRunDispatch(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	storage := &fakeCodeRunStorage{}
	dispatchErr := errors.New("dispatch failed")
	dispatcher := &fakeCodeRunDispatcher{err: dispatchErr}
	uc := NewCodeRunUsecase(repo, storage, dispatcher, 2500)

	_, err := uc.Create(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), CreateCodeRunInput{
		Language:   domain.ExecutionLanguagePython,
		SourceCode: "print(1)",
	})
	if !errors.Is(err, dispatchErr) {
		t.Fatalf("err = %v, want dispatch error", err)
	}
	if !storage.called || !dispatcher.called {
		t.Fatalf("calls = storage:%v dispatcher:%v", storage.called, dispatcher.called)
	}
}

func TestGetCodeRunOwner(t *testing.T) {
	repo := &fakeCodeRunRepository{findResult: &domain.CodeRun{ID: 1, UserID: codeRunUserID}}
	uc := newCodeRunUsecase(repo)

	result, err := uc.Get(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), 1)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if result.ID != 1 {
		t.Fatalf("id = %d, want 1", result.ID)
	}
}

func TestGetCodeRunAdmin(t *testing.T) {
	repo := &fakeCodeRunRepository{findResult: &domain.CodeRun{ID: 1, UserID: codeRunOtherID}}
	uc := newCodeRunUsecase(repo)

	_, err := uc.Get(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleAdmin), 1)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
}

func TestGetCodeRunForbidden(t *testing.T) {
	repo := &fakeCodeRunRepository{findResult: &domain.CodeRun{ID: 1, UserID: codeRunOtherID}}
	uc := newCodeRunUsecase(repo)

	_, err := uc.Get(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), 1)
	if !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestGetCodeRunID(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	uc := newCodeRunUsecase(repo)

	_, err := uc.Get(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), 0)
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if repo.findCalled {
		t.Fatal("repository should not be called")
	}
}

func TestListCodeRuns(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	uc := newCodeRunUsecase(repo)

	_, err := uc.List(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), ListCodeRunsInput{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if repo.listParams.UserID != codeRunUserID {
		t.Fatalf("user_id = %q", repo.listParams.UserID)
	}
	if repo.listParams.Limit != defaultCodeRunLimit || repo.listParams.Offset != 0 {
		t.Fatalf("params = %+v", repo.listParams)
	}
}

func TestListCodeRunsLimit(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	uc := newCodeRunUsecase(repo)

	_, err := uc.List(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), ListCodeRunsInput{
		Limit:         maxCodeRunLimit + 1,
		LimitProvided: true,
	})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if repo.listCalled {
		t.Fatal("repository should not be called")
	}
}

func TestListCodeRunsOffset(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	uc := newCodeRunUsecase(repo)

	_, err := uc.List(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), ListCodeRunsInput{Offset: -1})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if repo.listCalled {
		t.Fatal("repository should not be called")
	}
}

func newCodeRunUsecase(repo repository.CodeRunRepository) *CodeRunUsecase {
	return NewCodeRunUsecase(repo, &fakeCodeRunStorage{}, &fakeCodeRunDispatcher{}, 2500)
}

func codeRunActor(userID string, role domain.UserRole) auth.Actor {
	return auth.Actor{UserID: userID, Role: role}
}

func assertCodeRunCreateNotStarted(t *testing.T, repo *fakeCodeRunRepository, storage *fakeCodeRunStorage, dispatcher *fakeCodeRunDispatcher) {
	t.Helper()
	if repo.createCalled || storage.called || dispatcher.called {
		t.Fatalf("calls = repo:%v storage:%v dispatcher:%v", repo.createCalled, storage.called, dispatcher.called)
	}
}

type fakeCodeRunRepository struct {
	createCalled bool
	createParams repository.CreateCodeRunParams
	findCalled   bool
	findResult   *domain.CodeRun
	listCalled   bool
	listParams   repository.ListCodeRunsParams
}

func (r *fakeCodeRunRepository) Create(_ context.Context, params repository.CreateCodeRunParams) (*domain.CodeRun, error) {
	r.createCalled = true
	r.createParams = params
	return &domain.CodeRun{
		ID:         1,
		UserID:     params.UserID,
		Language:   params.Language,
		SourceCode: params.SourceCode,
		Stdin:      params.Stdin,
		Status:     domain.CodeRunStatusPending,
	}, nil
}

func (r *fakeCodeRunRepository) FindByID(_ context.Context, _ int64) (*domain.CodeRun, error) {
	r.findCalled = true
	return r.findResult, nil
}

func (r *fakeCodeRunRepository) ListByUserID(_ context.Context, params repository.ListCodeRunsParams) ([]domain.CodeRun, error) {
	r.listCalled = true
	r.listParams = params
	return []domain.CodeRun{}, nil
}

type fakeCodeRunStorage struct {
	called bool
	params repository.PutSourceCodeParams
	err    error
}

func (s *fakeCodeRunStorage) PutSourceCode(_ context.Context, params repository.PutSourceCodeParams) error {
	s.called = true
	s.params = params
	return s.err
}

type fakeCodeRunDispatcher struct {
	called bool
	job    repository.ExecutionJob
	err    error
}

func (d *fakeCodeRunDispatcher) Dispatch(_ context.Context, job repository.ExecutionJob) error {
	d.called = true
	d.job = job
	return d.err
}

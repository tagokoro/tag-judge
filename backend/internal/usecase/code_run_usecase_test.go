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
	uc := NewCodeRunUsecase(repo)

	result, err := uc.Create(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), CreateCodeRunInput{
		Language:   "python",
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
}

func TestCreateCodeRunLanguage(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	uc := NewCodeRunUsecase(repo)

	_, err := uc.Create(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), CreateCodeRunInput{
		Language:   "go",
		SourceCode: "package main",
	})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if repo.createCalled {
		t.Fatal("repository should not be called")
	}
}

func TestCreateCodeRunSource(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	uc := NewCodeRunUsecase(repo)

	_, err := uc.Create(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), CreateCodeRunInput{
		Language:   "python",
		SourceCode: "  ",
	})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if repo.createCalled {
		t.Fatal("repository should not be called")
	}
}

func TestGetCodeRunOwner(t *testing.T) {
	repo := &fakeCodeRunRepository{findResult: &domain.CodeRun{ID: 1, UserID: codeRunUserID}}
	uc := NewCodeRunUsecase(repo)

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
	uc := NewCodeRunUsecase(repo)

	_, err := uc.Get(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleAdmin), 1)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
}

func TestGetCodeRunForbidden(t *testing.T) {
	repo := &fakeCodeRunRepository{findResult: &domain.CodeRun{ID: 1, UserID: codeRunOtherID}}
	uc := NewCodeRunUsecase(repo)

	_, err := uc.Get(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), 1)
	if !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestGetCodeRunID(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	uc := NewCodeRunUsecase(repo)

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
	uc := NewCodeRunUsecase(repo)

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
	uc := NewCodeRunUsecase(repo)

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
	uc := NewCodeRunUsecase(repo)

	_, err := uc.List(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), ListCodeRunsInput{Offset: -1})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if repo.listCalled {
		t.Fatal("repository should not be called")
	}
}

func codeRunActor(userID string, role domain.UserRole) auth.Actor {
	return auth.Actor{UserID: userID, Role: role}
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

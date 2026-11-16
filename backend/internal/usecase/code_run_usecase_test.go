package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	admission := &fakeAdmissionControl{
		createCodeRunResult: &domain.CodeRun{
			ID:         1,
			UserID:     codeRunUserID,
			Language:   domain.ExecutionLanguagePython,
			SourceCode: "print(input())",
			Stdin:      "hello",
			Status:     domain.CodeRunStatusPending,
		},
	}
	uc := NewCodeRunUsecase(repo, storage, admission, 2500, 255)

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
	if admission.createCodeRunParams.UserID != codeRunUserID || admission.createCodeRunParams.Stdin != "hello" {
		t.Fatalf("params = %+v", admission.createCodeRunParams)
	}
	if admission.createCodeRunTimeLimitMS != 2500 {
		t.Fatalf("time limit = %d, want 2500", admission.createCodeRunTimeLimitMS)
	}
	if storage.params.Key != "code-runs/00000000-0000-4000-8000-000000000001/1/main.py" {
		t.Fatalf("source key = %q", storage.params.Key)
	}
	if storage.params.SourceCode != "print(input())" {
		t.Fatalf("source code = %q", storage.params.SourceCode)
	}
}

func TestCreateCodeRunLanguage(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	storage := &fakeCodeRunStorage{}
	admission := &fakeAdmissionControl{}
	uc := NewCodeRunUsecase(repo, storage, admission, 2500, 255)

	_, err := uc.Create(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), CreateCodeRunInput{
		Language:   domain.ExecutionLanguage("go"),
		SourceCode: "package main",
	})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	assertCodeRunCreateNotStarted(t, repo, storage, admission)
}

func TestCreateCodeRunSource(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	storage := &fakeCodeRunStorage{}
	admission := &fakeAdmissionControl{}
	uc := NewCodeRunUsecase(repo, storage, admission, 2500, 255)

	_, err := uc.Create(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), CreateCodeRunInput{
		Language:   domain.ExecutionLanguagePython,
		SourceCode: "  ",
	})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	assertCodeRunCreateNotStarted(t, repo, storage, admission)
}

func TestCreateCodeRunActor(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	storage := &fakeCodeRunStorage{}
	admission := &fakeAdmissionControl{}
	uc := NewCodeRunUsecase(repo, storage, admission, 2500, 255)

	_, err := uc.Create(context.Background(), codeRunActor("invalid", domain.UserRoleUser), CreateCodeRunInput{
		Language:   domain.ExecutionLanguagePython,
		SourceCode: "print(1)",
	})
	if !errors.Is(err, apperror.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	assertCodeRunCreateNotStarted(t, repo, storage, admission)
}

func TestCreateCodeRunStorage(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	storageErr := errors.New("storage failed")
	storage := &fakeCodeRunStorage{err: storageErr}
	admission := &fakeAdmissionControl{
		createCodeRunResult: &domain.CodeRun{
			ID:         1,
			UserID:     codeRunUserID,
			Language:   domain.ExecutionLanguagePython,
			SourceCode: "print(1)",
			Status:     domain.CodeRunStatusPending,
		},
	}
	uc := NewCodeRunUsecase(repo, storage, admission, 2500, 255)

	_, err := uc.Create(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), CreateCodeRunInput{
		Language:   domain.ExecutionLanguagePython,
		SourceCode: "print(1)",
	})
	if !errors.Is(err, storageErr) {
		t.Fatalf("err = %v, want storage error", err)
	}
	if storage.called {
		expectedKey := fmt.Sprintf("code-runs/%s/1/main.py", codeRunUserID)
		if storage.params.Key != expectedKey {
			t.Fatalf("source key = %q, want %q", storage.params.Key, expectedKey)
		}
	} else {
		t.Fatal("storage was not called")
	}
	if admission.createCodeRunCalled {
		t.Fatal("admission.CreateCodeRun should not be called when storage fails")
	}
	if !admission.reserveCodeRunIDCalled {
		t.Fatal("admission.ReserveCodeRunID should be called before storage")
	}
}

func TestCreateCodeRunIdempotencyKeyTooLong(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	storage := &fakeCodeRunStorage{}
	admission := &fakeAdmissionControl{}
	uc := NewCodeRunUsecase(repo, storage, admission, 2500, 5)

	_, err := uc.Create(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), CreateCodeRunInput{
		Language:       domain.ExecutionLanguagePython,
		SourceCode:     "print(1)",
		IdempotencyKey: "123456",
	})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	assertCodeRunCreateNotStarted(t, repo, storage, admission)
}

func TestCreateCodeRunIdempotencyKeyBlank(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	storage := &fakeCodeRunStorage{}
	admission := &fakeAdmissionControl{}
	uc := NewCodeRunUsecase(repo, storage, admission, 2500, 255)

	_, err := uc.Create(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), CreateCodeRunInput{
		Language:       domain.ExecutionLanguagePython,
		SourceCode:     "print(1)",
		IdempotencyKey: "   ",
	})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput for blank key", err)
	}
	assertCodeRunCreateNotStarted(t, repo, storage, admission)
}

func TestCreateCodeRunIdempotencyKeyTrimmed(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	storage := &fakeCodeRunStorage{}
	admission := &fakeAdmissionControl{}
	uc := NewCodeRunUsecase(repo, storage, admission, 2500, 255)

	_, err := uc.Create(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), CreateCodeRunInput{
		Language:       domain.ExecutionLanguagePython,
		SourceCode:     "print(1)",
		IdempotencyKey: "  key-1  ",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if admission.createCodeRunKeyInfo == nil || admission.createCodeRunKeyInfo.Key != "key-1" {
		t.Fatalf("stored key = %+v, want trimmed \"key-1\"", admission.createCodeRunKeyInfo)
	}
}

func TestCreateCodeRunIdempotencyKeyCountsRunes(t *testing.T) {
	repo := &fakeCodeRunRepository{}
	storage := &fakeCodeRunStorage{}
	admission := &fakeAdmissionControl{}
	// 200 multibyte runes is 600 bytes but 200 characters; the limit is
	// checked in characters to match the OpenAPI maxLength contract.
	longMultibyte := strings.Repeat("あ", 200)
	uc := NewCodeRunUsecase(repo, storage, admission, 2500, 255)

	_, err := uc.Create(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), CreateCodeRunInput{
		Language:       domain.ExecutionLanguagePython,
		SourceCode:     "print(1)",
		IdempotencyKey: longMultibyte,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if admission.createCodeRunKeyInfo == nil || len(admission.createCodeRunKeyInfo.Key) != 600 {
		t.Fatalf("multibyte key within the character limit was rejected")
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

// Someone else's code run is reported as missing.
func TestGetCodeRunOfOthersIsNotFound(t *testing.T) {
	repo := &fakeCodeRunRepository{findResult: &domain.CodeRun{ID: 1, UserID: codeRunOtherID}}
	uc := newCodeRunUsecase(repo)

	_, err := uc.Get(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), 1)
	requireAppError(t, err, apperror.ErrNotFound, "CODE_RUN_NOT_FOUND", "code run not found")
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

func TestCreateCodeRunSizeLimits(t *testing.T) {
	tests := []struct {
		name       string
		sourceCode string
		stdin      string
		wantCode   string
		wantMsg    string
	}{
		{"source at limit", strings.Repeat("a", domain.MaxExecutionSourceBytes), "", "", ""},
		{"stdin at limit", "print(1)", strings.Repeat("1", domain.MaxCodeRunStdinBytes), "", ""},
		{"source too large", strings.Repeat("a", domain.MaxExecutionSourceBytes+1), "", "SOURCE_CODE_TOO_LARGE", "source_code must be at most 65536 bytes"},
		{"stdin too large", "print(1)", strings.Repeat("1", domain.MaxCodeRunStdinBytes+1), "STDIN_TOO_LARGE", "stdin must be at most 131072 bytes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeCodeRunRepository{}
			storage := &fakeCodeRunStorage{}
			admission := &fakeAdmissionControl{}
			uc := NewCodeRunUsecase(repo, storage, admission, 2500, 255)

			_, err := uc.Create(context.Background(), codeRunActor(codeRunUserID, domain.UserRoleUser), CreateCodeRunInput{
				Language:   domain.ExecutionLanguagePython,
				SourceCode: tt.sourceCode,
				Stdin:      tt.stdin,
			})
			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("Create returned error: %v", err)
				}
				return
			}
			requireAppError(t, err, apperror.ErrInvalidInput, tt.wantCode, tt.wantMsg)
			assertCodeRunCreateNotStarted(t, repo, storage, admission)
		})
	}
}

func newCodeRunUsecase(repo repository.CodeRunRepository) *CodeRunUsecase {
	return NewCodeRunUsecase(repo, &fakeCodeRunStorage{}, &fakeAdmissionControl{}, 2500, 255)
}

func codeRunActor(userID string, role domain.UserRole) auth.Actor {
	return auth.Actor{UserID: userID, Role: role}
}

func assertCodeRunCreateNotStarted(t *testing.T, repo *fakeCodeRunRepository, storage *fakeCodeRunStorage, admission *fakeAdmissionControl) {
	t.Helper()
	if storage.called || admission.createCodeRunCalled || admission.reserveCodeRunIDCalled {
		t.Fatalf("calls = storage:%v admission:%v reserve=%v", storage.called, admission.createCodeRunCalled, admission.reserveCodeRunIDCalled)
	}
}

type fakeCodeRunRepository struct {
	findCalled bool
	findResult *domain.CodeRun
	listCalled bool
	listParams repository.ListCodeRunsParams
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

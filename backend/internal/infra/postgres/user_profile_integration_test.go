package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

// TestUserProfileRepositoryConflicts verifies the handle conflicts.
func TestUserProfileRepositoryConflicts(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	if _, err := db.Exec(ctx, `
INSERT INTO users (id, provider, provider_user_id, name, email)
VALUES ('00000000-0000-4000-8000-000000000001', 'test', 'alice', 'Alice Google', 'alice@example.test'),
       ('00000000-0000-4000-8000-000000000002', 'test', 'bob', 'Bob Google', 'bob@example.test');
`); err != nil {
		t.Fatalf("seed users: %v", err)
	}
	repo := postgres.NewUserProfileRepository(db)
	create := func(userID, handle string) error {
		_, err := repo.Create(ctx, repository.CreateUserProfileParams{UserID: userID, Handle: handle})
		return err
	}
	if err := create("00000000-0000-4000-8000-000000000001", "alice"); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	assertAppError(t, create("00000000-0000-4000-8000-000000000001", "alice-2"), apperror.ErrConflict, "HANDLE_ALREADY_SET", "you already have a handle")
	assertAppError(t, create("00000000-0000-4000-8000-000000000002", "alice"), apperror.ErrConflict, "HANDLE_TAKEN", "handle is already taken")
	assertAppError(t, create("00000000-0000-4000-8000-0000000000ff", "ghost"), apperror.ErrUnauthorized, "UNAUTHORIZED", "authentication required")

	profile, err := repo.FindByHandle(ctx, "alice")
	if err != nil || profile.UserID != "00000000-0000-4000-8000-000000000001" || profile.Handle != "alice" {
		t.Fatalf("FindByHandle = %+v, %v", profile, err)
	}
	_, err = repo.FindByUserID(ctx, "00000000-0000-4000-8000-000000000002")
	assertAppError(t, err, apperror.ErrNotFound, "USER_NOT_FOUND", "user not found")
}

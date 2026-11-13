package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

// TestUpsertGoogleUserWritesOnlyChanges pins the login upsert: the first
// login inserts the user, an unchanged login reads the row without writing,
// a changed name or email updates it, and has_handle follows user_profiles.
func TestUpsertGoogleUserWritesOnlyChanges(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	repo := postgres.NewAuthRepository(db)
	login := func(name, email string) (string, bool) {
		t.Helper()
		got, err := repo.UpsertGoogleUser(ctx, repository.GoogleUser{ProviderUserID: "google-sub", Name: name, Email: email})
		if err != nil {
			t.Fatalf("UpsertGoogleUser: %v", err)
		}
		return got.Actor.UserID, got.HasHandle
	}
	updatedAt := func(userID string) time.Time {
		t.Helper()
		var at time.Time
		if err := db.QueryRow(ctx, `SELECT updated_at FROM users WHERE id = $1`, userID).Scan(&at); err != nil {
			t.Fatalf("read updated_at: %v", err)
		}
		return at
	}

	userID, hasHandle := login("Alice", "alice@example.test")
	if userID == "" || hasHandle {
		t.Fatalf("first login = %q, has handle %v; want a new user without a handle", userID, hasHandle)
	}
	first := updatedAt(userID)

	again, _ := login("Alice", "alice@example.test")
	if again != userID || !updatedAt(userID).Equal(first) {
		t.Fatalf("an unchanged login must not write: id %q (want %q), updated_at %v (was %v)", again, userID, updatedAt(userID), first)
	}

	if renamed, _ := login("Alice Liddell", "alice@example.test"); renamed != userID || !updatedAt(userID).After(first) {
		t.Fatalf("a changed name must update the same user: id %q, updated_at %v (was %v)", renamed, updatedAt(userID), first)
	}
	var name string
	if err := db.QueryRow(ctx, `SELECT name FROM users WHERE id = $1`, userID).Scan(&name); err != nil || name != "Alice Liddell" {
		t.Fatalf("name = %q, %v; want the new name", name, err)
	}

	if _, err := db.Exec(ctx, `INSERT INTO user_profiles (user_id, handle) VALUES ($1, 'alice')`, userID); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	if _, hasHandle := login("Alice Liddell", "alice@example.test"); !hasHandle {
		t.Fatal("has_handle must be true once the user registered a handle")
	}
}

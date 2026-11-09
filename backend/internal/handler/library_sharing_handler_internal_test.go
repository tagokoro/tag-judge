package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
)

// versionNumberRequest runs parseVersionNumber through a real chi route so the
// URL param is populated exactly as it is in production.
func versionNumberRequest(t *testing.T, raw string) (int, error) {
	t.Helper()
	var got int
	var gotErr error
	router := chi.NewRouter()
	router.Get("/versions/{version_number}", func(_ http.ResponseWriter, r *http.Request) {
		got, gotErr = parseVersionNumber(r)
	})
	req := httptest.NewRequest(http.MethodGet, "/versions/"+raw, nil)
	router.ServeHTTP(httptest.NewRecorder(), req)
	return got, gotErr
}

func assertInvalidVersionNumber(t *testing.T, err error, wantMessage string) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != "INVALID_VERSION_NUMBER" {
		t.Fatalf("err = %v, want INVALID_VERSION_NUMBER AppError", err)
	}
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput sentinel", err)
	}
	if appErr.Message != wantMessage {
		t.Fatalf("message = %q, want %q", appErr.Message, wantMessage)
	}
}

// TestParseVersionNumberRejectsInt32Overflow pins the fix for the alias bug:
// the sqlc GetLibraryVersionByNumber query casts version_number to int32, so
// 4294967297 (2^32+1) would otherwise wrap to version 1. Overflow keeps a
// distinct stable 400 message; documented invalid cases keep their exact string.
func TestParseVersionNumberRejectsInt32Overflow(t *testing.T) {
	for _, raw := range []string{
		"2147483648",          // MaxInt32 + 1
		"4294967296",          // 2^32 -> int32 0
		"4294967297",          // 2^32 + 1 -> int32 1 (aliases version 1)
		"9223372036854775807", // MaxInt64
	} {
		if _, err := versionNumberRequest(t, raw); err == nil {
			t.Fatalf("version_number %q must be rejected (would wrap int32)", raw)
		} else {
			assertInvalidVersionNumber(t, err, "version_number is too large")
		}
	}
}

func TestParseVersionNumberAcceptsInt32Range(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int
	}{
		{"1", 1},
		{"7", 7},
		{"2147483647", 2147483647}, // MaxInt32 is still valid
	} {
		got, err := versionNumberRequest(t, tc.raw)
		if err != nil {
			t.Fatalf("version_number %q: unexpected error %v", tc.raw, err)
		}
		if got != tc.want {
			t.Fatalf("version_number %q = %d, want %d", tc.raw, got, tc.want)
		}
	}
}

// TestParseVersionNumberRejectsNonPositiveAndGarbage pins the exact spec §10
// message for the previously documented invalid cases (non-numeric and
// non-positive values).
func TestParseVersionNumberRejectsNonPositiveAndGarbage(t *testing.T) {
	for _, raw := range []string{"0", "-1", "abc", "99999999999999999999999"} {
		if _, err := versionNumberRequest(t, raw); err == nil {
			t.Fatalf("version_number %q must be rejected", raw)
		} else {
			assertInvalidVersionNumber(t, err, "version_number must be a positive integer")
		}
	}
}

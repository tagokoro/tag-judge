package router

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
)

func TestGetSubmissionDetailsRoute(t *testing.T) {
	judgedAt := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	zero, runtime, memory := 0, 25, 1024
	repo := &fakeSubmissionRepository{
		submission: &domain.Submission{
			ID: 7, UserID: routerUserID, ProblemID: 42,
			Language: domain.SubmissionLanguagePython, SourceCodeKey: "submissions/7/main.py",
			Status: domain.SubmissionStatusWA, Score: 1, JudgedAt: &judgedAt,
		},
		caseResults: []domain.SubmissionCaseResult{
			{TestCaseID: 10, Status: domain.SubmissionStatusAC, ExecutionTimeMS: &zero, MemoryKB: &memory},
			{TestCaseID: 12, Status: domain.SubmissionStatusWA, ExecutionTimeMS: &runtime},
		},
	}
	router := newTestRouterWithRepositories(newRouterGroupRepository(), repo)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/submissions/7", nil)
	setAuth(req, "user")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body)
	}
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data["id"] != float64(7) || body.Data["status"] != "wa" || body.Data["score"] != float64(1) || body.Data["judged_at"] != "2026-10-04T12:00:00Z" {
		t.Fatalf("submission metadata was not preserved: %v", body.Data)
	}
	want := []any{
		map[string]any{"test_case_id": float64(10), "status": "ac", "execution_time_ms": float64(0), "memory_kb": float64(1024)},
		map[string]any{"test_case_id": float64(12), "status": "wa", "execution_time_ms": float64(25), "memory_kb": nil},
	}
	if !reflect.DeepEqual(body.Data["case_results"], want) {
		t.Fatalf("case_results = %v, want %v (no hidden inputs, expected output, stdout, or stderr)", body.Data["case_results"], want)
	}
}

func TestGetSubmissionDetailsEmptyCases(t *testing.T) {
	for _, status := range []domain.SubmissionStatus{domain.SubmissionStatusPending, domain.SubmissionStatusRunning, domain.SubmissionStatusIE} {
		t.Run(string(status), func(t *testing.T) {
			repo := &fakeSubmissionRepository{submission: &domain.Submission{ID: 7, UserID: routerUserID, Status: status}}
			router := newTestRouterWithRepositories(newRouterGroupRepository(), repo)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/submissions/7", nil)
			setAuth(req, "user")
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			var body struct {
				Data struct {
					Cases json.RawMessage `json:"case_results"`
				} `json:"data"`
			}
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if res.Code != http.StatusOK || string(body.Data.Cases) != "[]" {
				t.Fatalf("expected 200 with empty array, got %d: %s", res.Code, res.Body)
			}
		})
	}
}

func TestGetSubmissionDetailsAccessAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		role       string
		owner      string
		path       string
		findErr    error
		resultsErr error
		wantStatus int
		wantCode   string
		wantReads  int
	}{
		{name: "unauthenticated", path: "/api/v1/submissions/7", wantStatus: 401, wantCode: "UNAUTHORIZED"},
		{name: "other user", role: "user", owner: "another-user", path: "/api/v1/submissions/7", wantStatus: 403, wantCode: "SUBMISSION_FORBIDDEN"},
		{name: "author is not admin", role: "author", owner: "another-user", path: "/api/v1/submissions/7", wantStatus: 403, wantCode: "SUBMISSION_FORBIDDEN"},
		{name: "admin", role: "admin", owner: "another-user", path: "/api/v1/submissions/7", wantStatus: 200, wantReads: 1},
		{name: "invalid id", role: "user", path: "/api/v1/submissions/0", wantStatus: 400, wantCode: "INVALID_SUBMISSION_ID"},
		{name: "missing", role: "user", path: "/api/v1/submissions/7", findErr: apperror.NotFound("SUBMISSION_NOT_FOUND", "submission not found"), wantStatus: 404, wantCode: "SUBMISSION_NOT_FOUND"},
		{name: "database failure", role: "user", owner: routerUserID, path: "/api/v1/submissions/7", resultsErr: errors.New("database unavailable"), wantStatus: 500, wantCode: "INTERNAL_ERROR", wantReads: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeSubmissionRepository{
				submission: &domain.Submission{ID: 7, UserID: tc.owner, Status: domain.SubmissionStatusAC},
				findErr:    tc.findErr, caseResultsErr: tc.resultsErr,
			}
			router := newTestRouterWithRepositories(newRouterGroupRepository(), repo)
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.role != "" {
				setAuth(req, tc.role)
			}
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", res.Code, tc.wantStatus, res.Body)
			}
			if tc.wantCode != "" {
				assertErrorCode(t, res, tc.wantCode)
			}
			if repo.caseResultsCalls != tc.wantReads {
				t.Fatalf("case result reads = %d, want %d", repo.caseResultsCalls, tc.wantReads)
			}
		})
	}
}

func TestSubmissionSummaryRoutesDoNotLoadCaseResults(t *testing.T) {
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/problems/1/submissions", `{"language":"python","source_code":"print(1)"}`},
		{http.MethodGet, "/api/v1/submissions", ""},
	} {
		t.Run(tc.method, func(t *testing.T) {
			repo := &fakeSubmissionRepository{list: []domain.Submission{{ID: 7, UserID: routerUserID, Status: domain.SubmissionStatusAC}}}
			router := newTestRouterWithRepositories(newRouterGroupRepository(), repo)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			setAuth(req, "user")
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != http.StatusOK && res.Code != http.StatusCreated {
				t.Fatalf("status = %d, body = %s", res.Code, res.Body)
			}
			if strings.Contains(res.Body.String(), "case_results") || repo.caseResultsCalls != 0 {
				t.Fatalf("summary routes must not load or return case results: %s", res.Body)
			}
		})
	}
}

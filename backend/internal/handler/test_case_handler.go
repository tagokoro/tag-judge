package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

type TestCaseHandler struct {
	testCases *usecase.TestCaseUsecase
}

func NewTestCaseHandler(testCases *usecase.TestCaseUsecase) *TestCaseHandler {
	return &TestCaseHandler{testCases: testCases}
}

func (h *TestCaseHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, problemID, ok := testCaseActorAndProblemID(w, r)
	if !ok {
		return
	}
	testCases, err := h.testCases.List(r.Context(), actor, problemID)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	response := make([]testCaseResponse, 0, len(testCases))
	for _, testCase := range testCases {
		response = append(response, newTestCaseResponse(testCase))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *TestCaseHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, problemID, ok := testCaseActorAndProblemID(w, r)
	if !ok {
		return
	}
	var request createTestCaseRequest
	if err := decodeJSONBody(r, &request); err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_JSON", "request body must be valid JSON"))
		return
	}
	if request.Input == nil || request.ExpectedOutput == nil || request.IsSample == nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_TEST_CASE", "input, expected_output, and is_sample are required"))
		return
	}
	testCase, err := h.testCases.Create(r.Context(), actor, problemID, usecase.CreateTestCaseInput{
		Input:          *request.Input,
		ExpectedOutput: *request.ExpectedOutput,
		IsSample:       *request.IsSample,
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, newTestCaseResponse(*testCase))
}

func (h *TestCaseHandler) Update(w http.ResponseWriter, r *http.Request) {
	actor, problemID, ok := testCaseActorAndProblemID(w, r)
	if !ok {
		return
	}
	id, err := parseTestCaseID(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	var request updateTestCaseRequest
	if err := decodeJSONBody(r, &request); err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_JSON", "request body must be valid JSON"))
		return
	}
	testCase, err := h.testCases.Update(r.Context(), actor, problemID, id, usecase.UpdateTestCaseInput{
		Input:          request.Input,
		ExpectedOutput: request.ExpectedOutput,
		IsSample:       request.IsSample,
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, newTestCaseResponse(*testCase))
}

func (h *TestCaseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	actor, problemID, ok := testCaseActorAndProblemID(w, r)
	if !ok {
		return
	}
	id, err := parseTestCaseID(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	if err := h.testCases.Delete(r.Context(), actor, problemID, id); err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func testCaseActorAndProblemID(w http.ResponseWriter, r *http.Request) (auth.Actor, int64, bool) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		WriteError(w, r, apperror.Unauthorized("UNAUTHORIZED", "authentication required"))
		return auth.Actor{}, 0, false
	}
	problemID, err := parseProblemID(r)
	if err != nil {
		WriteError(w, r, err)
		return auth.Actor{}, 0, false
	}
	return actor, problemID, true
}

func parseTestCaseID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "test_case_id"), 10, 64)
	if err != nil {
		return 0, apperror.InvalidInput("INVALID_TEST_CASE_ID", "test_case_id must be an integer")
	}
	return id, nil
}

type createTestCaseRequest struct {
	Input          *string `json:"input"`
	ExpectedOutput *string `json:"expected_output"`
	IsSample       *bool   `json:"is_sample"`
}

type updateTestCaseRequest struct {
	Input          *string `json:"input"`
	ExpectedOutput *string `json:"expected_output"`
	IsSample       *bool   `json:"is_sample"`
}

type testCaseResponse struct {
	ID             int64     `json:"id"`
	ProblemID      int64     `json:"problem_id"`
	Input          string    `json:"input"`
	ExpectedOutput string    `json:"expected_output"`
	IsSample       bool      `json:"is_sample"`
	Score          int       `json:"score"`
	CreatedAt      time.Time `json:"created_at"`
}

func newTestCaseResponse(testCase domain.TestCase) testCaseResponse {
	return testCaseResponse{
		ID:             testCase.ID,
		ProblemID:      testCase.ProblemID,
		Input:          testCase.Input,
		ExpectedOutput: testCase.ExpectedOutput,
		IsSample:       testCase.IsSample,
		Score:          testCase.Score,
		CreatedAt:      testCase.CreatedAt,
	}
}

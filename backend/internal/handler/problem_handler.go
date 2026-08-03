package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

type ProblemHandler struct {
	problems *usecase.ProblemUsecase
}

func NewProblemHandler(problems *usecase.ProblemUsecase) *ProblemHandler {
	return &ProblemHandler{problems: problems}
}

func (h *ProblemHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, limitProvided, err := parseOptionalInt(r, "limit")
	if err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_LIMIT", "limit must be an integer"))
		return
	}
	offset, _, err := parseOptionalInt(r, "offset")
	if err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_OFFSET", "offset must be an integer"))
		return
	}

	actor, ok := auth.ActorFromContext(r.Context())
	var actorRef *auth.Actor
	if ok {
		actorRef = &actor
	}

	problems, err := h.problems.ListVisibleProblems(r.Context(), usecase.ListProblemsInput{
		Limit:         limit,
		LimitProvided: limitProvided,
		Offset:        offset,
	}, actorRef)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	response := make([]problemResponse, 0, len(problems))
	for _, problem := range problems {
		response = append(response, newProblemResponse(problem))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *ProblemHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "problem_id"), 10, 64)
	if err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_PROBLEM_ID", "problem_id must be an integer"))
		return
	}

	problem, err := h.problems.GetPublicProblem(r.Context(), id)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newProblemResponse(*problem))
}

func (h *ProblemHandler) GetVisible(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathInt(r, "problem_id")
	if err != nil {
		WriteError(w, r, err)
		return
	}
	actor, ok := auth.ActorFromContext(r.Context())
	var actorRef *auth.Actor
	if ok {
		actorRef = &actor
	}
	problem, err := h.problems.GetVisibleProblem(r.Context(), id, actorRef)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, newProblemResponse(*problem))
}

func (h *ProblemHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	var body problemRequest
	if err := decodeJSON(r, &body); err != nil {
		WriteError(w, r, err)
		return
	}
	problem, err := h.problems.CreateProblem(r.Context(), actor, body.toInput())
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, newProblemResponse(*problem))
}

func (h *ProblemHandler) Update(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	id, err := parsePathInt(r, "problem_id")
	if err != nil {
		WriteError(w, r, err)
		return
	}
	var body problemRequest
	if err := decodeJSON(r, &body); err != nil {
		WriteError(w, r, err)
		return
	}
	problem, err := h.problems.UpdateProblem(r.Context(), actor, id, body.toInput())
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, newProblemResponse(*problem))
}

func (h *ProblemHandler) Delete(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	id, err := parsePathInt(r, "problem_id")
	if err != nil {
		WriteError(w, r, err)
		return
	}
	if err := h.problems.DeleteProblem(r.Context(), actor, id); err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProblemHandler) ListTestCases(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	problemID, err := parsePathInt(r, "problem_id")
	if err != nil {
		WriteError(w, r, err)
		return
	}
	testCases, err := h.problems.ListTestCases(r.Context(), actor, problemID)
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

func (h *ProblemHandler) CreateTestCase(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	problemID, err := parsePathInt(r, "problem_id")
	if err != nil {
		WriteError(w, r, err)
		return
	}
	var body testCaseRequest
	if err := decodeJSON(r, &body); err != nil {
		WriteError(w, r, err)
		return
	}
	testCase, err := h.problems.CreateTestCase(r.Context(), actor, problemID, body.toInput())
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, newTestCaseResponse(*testCase))
}

func (h *ProblemHandler) UpdateTestCase(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	problemID, err := parsePathInt(r, "problem_id")
	if err != nil {
		WriteError(w, r, err)
		return
	}
	testCaseID, err := parsePathInt(r, "test_case_id")
	if err != nil {
		WriteError(w, r, err)
		return
	}
	var body testCaseRequest
	if err := decodeJSON(r, &body); err != nil {
		WriteError(w, r, err)
		return
	}
	testCase, err := h.problems.UpdateTestCase(r.Context(), actor, problemID, testCaseID, body.toInput())
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, newTestCaseResponse(*testCase))
}

func (h *ProblemHandler) DeleteTestCase(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	problemID, err := parsePathInt(r, "problem_id")
	if err != nil {
		WriteError(w, r, err)
		return
	}
	testCaseID, err := parsePathInt(r, "test_case_id")
	if err != nil {
		WriteError(w, r, err)
		return
	}
	if err := h.problems.DeleteTestCase(r.Context(), actor, problemID, testCaseID); err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProblemHandler) CreateSubmission(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	problemID, err := parsePathInt(r, "problem_id")
	if err != nil {
		WriteError(w, r, err)
		return
	}
	var body submissionRequest
	if err := decodeJSON(r, &body); err != nil {
		WriteError(w, r, err)
		return
	}
	submission, err := h.problems.CreateSubmission(r.Context(), actor, problemID, body.toInput())
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, newSubmissionResponse(*submission))
}

func (h *ProblemHandler) ListSubmissions(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	problemID, err := parsePathInt(r, "problem_id")
	if err != nil {
		WriteError(w, r, err)
		return
	}
	submissions, err := h.problems.ListSubmissionsByProblem(r.Context(), actor, problemID)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	response := make([]submissionResponse, 0, len(submissions))
	for _, submission := range submissions {
		response = append(response, newSubmissionResponse(submission))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *ProblemHandler) GetSubmission(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	submissionID, err := parsePathInt(r, "submission_id")
	if err != nil {
		WriteError(w, r, err)
		return
	}
	submission, err := h.problems.GetSubmission(r.Context(), actor, submissionID)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, newSubmissionResponse(*submission))
}

func parseOptionalInt(r *http.Request, key string) (int, bool, error) {
	values, ok := r.URL.Query()[key]
	if !ok {
		return 0, false, nil
	}
	value := values[0]
	if value == "" {
		return 0, false, nil
	}
	parsed, err := strconv.Atoi(value)
	return parsed, true, err
}

func parsePathInt(r *http.Request, key string) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, key), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperror.InvalidInput("INVALID_ID", key+" must be a positive integer")
	}
	return id, nil
}

func decodeJSON(r *http.Request, dest any) error {
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		return apperror.InvalidInput("INVALID_JSON", "request body must be valid JSON")
	}
	return nil
}

type problemRequest struct {
	Title         string `json:"title"`
	Statement     string `json:"statement"`
	TimeLimitMS   int    `json:"time_limit_ms"`
	MemoryLimitKB int    `json:"memory_limit_kb"`
	Visibility    string `json:"visibility"`
}

func (r problemRequest) toInput() usecase.ProblemInput {
	return usecase.ProblemInput{
		Title:         r.Title,
		Statement:     r.Statement,
		TimeLimitMS:   r.TimeLimitMS,
		MemoryLimitKB: r.MemoryLimitKB,
		Visibility:    domain.ProblemVisibility(r.Visibility),
	}
}

type testCaseRequest struct {
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	IsSample       bool   `json:"is_sample"`
	Score          int    `json:"score"`
}

func (r testCaseRequest) toInput() usecase.TestCaseInput {
	return usecase.TestCaseInput{
		Input:          r.Input,
		ExpectedOutput: r.ExpectedOutput,
		IsSample:       r.IsSample,
		Score:          r.Score,
	}
}

type submissionRequest struct {
	Language   string `json:"language"`
	SourceCode string `json:"source_code"`
}

func (r submissionRequest) toInput() usecase.SubmissionInput {
	return usecase.SubmissionInput{
		Language:   r.Language,
		SourceCode: r.SourceCode,
	}
}

type problemResponse struct {
	ID            int64     `json:"id"`
	Title         string    `json:"title"`
	Statement     string    `json:"statement"`
	TimeLimitMS   int       `json:"time_limit_ms"`
	MemoryLimitKB int       `json:"memory_limit_kb"`
	Visibility    string    `json:"visibility"`
	CreatedBy     string    `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func newProblemResponse(problem domain.Problem) problemResponse {
	return problemResponse{
		ID:            problem.ID,
		Title:         problem.Title,
		Statement:     problem.Statement,
		TimeLimitMS:   problem.TimeLimitMS,
		MemoryLimitKB: problem.MemoryLimitKB,
		Visibility:    string(problem.Visibility),
		CreatedBy:     problem.CreatedBy,
		CreatedAt:     problem.CreatedAt,
		UpdatedAt:     problem.UpdatedAt,
	}
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

type submissionResponse struct {
	ID         int64                      `json:"id"`
	UserID     string                     `json:"user_id"`
	ProblemID  int64                      `json:"problem_id"`
	Language   string                     `json:"language"`
	SourceCode string                     `json:"source_code"`
	Status     string                     `json:"status"`
	Score      int                        `json:"score"`
	CreatedAt  time.Time                  `json:"created_at"`
	JudgedAt   *time.Time                 `json:"judged_at"`
	Results    []submissionResultResponse `json:"results"`
}

type submissionResultResponse struct {
	ID              int64     `json:"id"`
	SubmissionID    int64     `json:"submission_id"`
	TestCaseID      int64     `json:"test_case_id"`
	Status          string    `json:"status"`
	ExecutionTimeMS *int      `json:"execution_time_ms"`
	MemoryKB        *int      `json:"memory_kb"`
	Stdout          string    `json:"stdout"`
	Stderr          string    `json:"stderr"`
	CreatedAt       time.Time `json:"created_at"`
}

func newSubmissionResponse(submission domain.Submission) submissionResponse {
	results := make([]submissionResultResponse, 0, len(submission.Results))
	for _, result := range submission.Results {
		results = append(results, submissionResultResponse{
			ID:              result.ID,
			SubmissionID:    result.SubmissionID,
			TestCaseID:      result.TestCaseID,
			Status:          string(result.Status),
			ExecutionTimeMS: result.ExecutionTimeMS,
			MemoryKB:        result.MemoryKB,
			Stdout:          result.Stdout,
			Stderr:          result.Stderr,
			CreatedAt:       result.CreatedAt,
		})
	}
	return submissionResponse{
		ID:         submission.ID,
		UserID:     submission.UserID,
		ProblemID:  submission.ProblemID,
		Language:   submission.Language,
		SourceCode: submission.SourceCode,
		Status:     string(submission.Status),
		Score:      submission.Score,
		CreatedAt:  submission.CreatedAt,
		JudgedAt:   submission.JudgedAt,
		Results:    results,
	}
}

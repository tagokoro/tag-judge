package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

type SubmissionHandler struct {
	submissions *usecase.SubmissionUsecase
}

func NewSubmissionHandler(submissions *usecase.SubmissionUsecase) *SubmissionHandler {
	return &SubmissionHandler{submissions: submissions}
}

func (h *SubmissionHandler) Create(w http.ResponseWriter, r *http.Request) {
	problemID, err := strconv.ParseInt(chi.URLParam(r, "problem_id"), 10, 64)
	if err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_PROBLEM_ID", "problem_id must be an integer"))
		return
	}

	var request createSubmissionRequest
	if err := decodeExecutionJSON(r, &request); err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_REQUEST_BODY", "request body must be valid JSON"))
		return
	}

	actor, _ := auth.ActorFromContext(r.Context())
	submission, err := h.submissions.Create(r.Context(), usecase.CreateSubmissionInput{
		Actor:          actor,
		ProblemID:      problemID,
		Language:       domain.SubmissionLanguage(request.Language),
		SourceCode:     request.SourceCode,
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, newSubmissionResponse(*submission))
}

func (h *SubmissionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "submission_id"), 10, 64)
	if err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_SUBMISSION_ID", "submission_id must be an integer"))
		return
	}

	actor, _ := auth.ActorFromContext(r.Context())
	submission, err := h.submissions.Get(r.Context(), usecase.GetSubmissionInput{
		Actor:        actor,
		SubmissionID: id,
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newSubmissionDetailResponse(*submission))
}

func (h *SubmissionHandler) List(w http.ResponseWriter, r *http.Request) {
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

	var problemID *int64
	if value := r.URL.Query().Get("problem_id"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			WriteError(w, r, apperror.InvalidInput("INVALID_PROBLEM_ID", "problem_id must be an integer"))
			return
		}
		problemID = &parsed
	}

	actor, _ := auth.ActorFromContext(r.Context())
	submissions, err := h.submissions.List(r.Context(), usecase.ListSubmissionsInput{
		Actor:         actor,
		ProblemID:     problemID,
		Limit:         limit,
		LimitProvided: limitProvided,
		Offset:        offset,
	})
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

// ListRecent lists recent submissions to public problems for everyone.
func (h *SubmissionHandler) ListRecent(w http.ResponseWriter, r *http.Request) {
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

	submissions, err := h.submissions.ListRecentPublic(r.Context(), usecase.ListRecentSubmissionsInput{
		Limit:         limit,
		LimitProvided: limitProvided,
		Offset:        offset,
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}

	response := make([]publicSubmissionResponse, 0, len(submissions))
	for _, submission := range submissions {
		response = append(response, newPublicSubmissionResponse(submission))
	}
	writeJSON(w, http.StatusOK, response)
}

// publicSubmissionResponse has no source key and no user id; the submitter is
// identified only by their handle (null when they have none).
type publicSubmissionResponse struct {
	ID           int64      `json:"id"`
	ProblemID    int64      `json:"problem_id"`
	ProblemTitle string     `json:"problem_title"`
	Language     string     `json:"language"`
	Status       string     `json:"status"`
	Score        int        `json:"score"`
	CreatedAt    time.Time  `json:"created_at"`
	JudgedAt     *time.Time `json:"judged_at"`
	Submitter    *string    `json:"submitter"`
}

func newPublicSubmissionResponse(submission domain.PublicSubmission) publicSubmissionResponse {
	return publicSubmissionResponse{
		ID:           submission.ID,
		ProblemID:    submission.ProblemID,
		ProblemTitle: submission.ProblemTitle,
		Language:     string(submission.Language),
		Status:       string(submission.Status),
		Score:        submission.Score,
		CreatedAt:    submission.CreatedAt,
		JudgedAt:     submission.JudgedAt,
		Submitter:    optionalString(submission.SubmitterHandle),
	}
}

// maxExecutionRequestBytes bounds a submission or code-run body. JSON escaping
// can expand one decoded byte to as many as 6 raw bytes (a \uXXXX escape), so
// a fully escaped body at the source and stdin limits is still admitted; the
// usecase then enforces the decoded limits.
const maxExecutionRequestBytes = 6*(domain.MaxExecutionSourceBytes+domain.MaxCodeRunStdinBytes) + 64*1024

// decodeExecutionJSON bounds a request body before the shared decoder runs so
// an oversized body gets a stable 400 rather than being silently truncated.
func decodeExecutionJSON(r *http.Request, target any) error {
	data, err := io.ReadAll(io.LimitReader(r.Body, maxExecutionRequestBytes+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > maxExecutionRequestBytes {
		return errors.New("request body too large")
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	return decodeJSON(r, target)
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("request body must contain one JSON value")
		}
		return err
	}
	return nil
}

type createSubmissionRequest struct {
	Language   string `json:"language"`
	SourceCode string `json:"source_code"`
}

type submissionResponse struct {
	ID            int64      `json:"id"`
	UserID        string     `json:"user_id"`
	ProblemID     int64      `json:"problem_id"`
	Language      string     `json:"language"`
	SourceCodeKey string     `json:"source_code_key"`
	Status        string     `json:"status"`
	Score         int        `json:"score"`
	CreatedAt     time.Time  `json:"created_at"`
	JudgedAt      *time.Time `json:"judged_at"`
}

func newSubmissionResponse(submission domain.Submission) submissionResponse {
	return submissionResponse{
		ID:            submission.ID,
		UserID:        submission.UserID,
		ProblemID:     submission.ProblemID,
		Language:      string(submission.Language),
		SourceCodeKey: submission.SourceCodeKey,
		Status:        string(submission.Status),
		Score:         submission.Score,
		CreatedAt:     submission.CreatedAt,
		JudgedAt:      submission.JudgedAt,
	}
}

// submissionDetailResponse may be read by anyone when the submission is
// public, so it carries no user id or storage key; the submitter is named by
// handle (null when they have none).
type submissionDetailResponse struct {
	ID           int64                          `json:"id"`
	ProblemID    int64                          `json:"problem_id"`
	ProblemTitle string                         `json:"problem_title"`
	Submitter    *string                        `json:"submitter"`
	Language     string                         `json:"language"`
	SourceCode   string                         `json:"source_code"`
	Status       string                         `json:"status"`
	Score        int                            `json:"score"`
	IsPublic     bool                           `json:"is_public"`
	CreatedAt    time.Time                      `json:"created_at"`
	JudgedAt     *time.Time                     `json:"judged_at"`
	CaseResults  []submissionCaseResultResponse `json:"case_results"`
}

type submissionCaseResultResponse struct {
	TestCaseID      int64  `json:"test_case_id"`
	Status          string `json:"status"`
	ExecutionTimeMS *int   `json:"execution_time_ms"`
	MemoryKB        *int   `json:"memory_kb"`
}

func newSubmissionDetailResponse(detail domain.SubmissionDetail) submissionDetailResponse {
	results := make([]submissionCaseResultResponse, 0, len(detail.CaseResults))
	for _, result := range detail.CaseResults {
		results = append(results, submissionCaseResultResponse{
			TestCaseID:      result.TestCaseID,
			Status:          string(result.Status),
			ExecutionTimeMS: result.ExecutionTimeMS,
			MemoryKB:        result.MemoryKB,
		})
	}
	return submissionDetailResponse{
		ID:           detail.ID,
		ProblemID:    detail.ProblemID,
		ProblemTitle: detail.ProblemTitle,
		Submitter:    optionalString(detail.SubmitterHandle),
		Language:     string(detail.Language),
		SourceCode:   detail.SourceCode,
		Status:       string(detail.Status),
		Score:        detail.Score,
		IsPublic:     detail.IsPublic,
		CreatedAt:    detail.CreatedAt,
		JudgedAt:     detail.JudgedAt,
		CaseResults:  results,
	}
}

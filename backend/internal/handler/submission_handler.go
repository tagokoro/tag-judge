package handler

import (
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
	if err := decodeJSON(r, &request); err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_REQUEST_BODY", "request body must be valid JSON"))
		return
	}

	actor, _ := auth.ActorFromContext(r.Context())
	submission, err := h.submissions.Create(r.Context(), usecase.CreateSubmissionInput{
		Actor:      actor,
		ProblemID:  problemID,
		Language:   domain.SubmissionLanguage(request.Language),
		SourceCode: request.SourceCode,
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

	writeJSON(w, http.StatusOK, newSubmissionResponse(*submission))
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

	actor, _ := auth.ActorFromContext(r.Context())
	submissions, err := h.submissions.List(r.Context(), usecase.ListSubmissionsInput{
		Actor:         actor,
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

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

type ProblemHandler struct {
	problems *usecase.ProblemUsecase
}

func NewProblemHandler(problems *usecase.ProblemUsecase) *ProblemHandler {
	return &ProblemHandler{problems: problems}
}

func (h *ProblemHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		WriteError(w, r, apperror.Unauthorized("UNAUTHORIZED", "authentication required"))
		return
	}

	var request createProblemRequest
	if err := decodeJSONBody(r, &request); err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_JSON", "request body must be valid JSON"))
		return
	}

	problem, err := h.problems.CreateProblem(r.Context(), actor, usecase.CreateProblemInput{
		Title:         request.Title,
		Statement:     request.Statement,
		TimeLimitMS:   request.TimeLimitMS,
		MemoryLimitKB: request.MemoryLimitKB,
		Visibility:    domain.ProblemVisibility(request.Visibility),
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, newProblemResponse(*problem))
}

func (h *ProblemHandler) Update(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		WriteError(w, r, apperror.Unauthorized("UNAUTHORIZED", "authentication required"))
		return
	}
	id, err := parseProblemID(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	var request updateProblemRequest
	if err := decodeJSONBody(r, &request); err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_JSON", "request body must be valid JSON"))
		return
	}
	var visibility *domain.ProblemVisibility
	if request.Visibility != nil {
		value := domain.ProblemVisibility(*request.Visibility)
		visibility = &value
	}
	problem, err := h.problems.UpdateProblem(r.Context(), actor, id, usecase.UpdateProblemInput{
		Title:         request.Title,
		Statement:     request.Statement,
		TimeLimitMS:   request.TimeLimitMS,
		MemoryLimitKB: request.MemoryLimitKB,
		Visibility:    visibility,
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, newProblemResponse(*problem))
}

func (h *ProblemHandler) Delete(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		WriteError(w, r, apperror.Unauthorized("UNAUTHORIZED", "authentication required"))
		return
	}
	id, err := parseProblemID(r)
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

	problems, err := h.problems.ListPublicProblems(r.Context(), usecase.ListProblemsInput{
		Limit:         limit,
		LimitProvided: limitProvided,
		Offset:        offset,
	})
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
	id, err := parseProblemID(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	problem, err := h.problems.GetPublicProblem(r.Context(), id)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newProblemResponse(*problem))
}

func parseProblemID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "problem_id"), 10, 64)
	if err != nil {
		return 0, apperror.InvalidInput("INVALID_PROBLEM_ID", "problem_id must be an integer")
	}
	return id, nil
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

type createProblemRequest struct {
	Title         string `json:"title"`
	Statement     string `json:"statement"`
	TimeLimitMS   int    `json:"time_limit_ms"`
	MemoryLimitKB int    `json:"memory_limit_kb"`
	Visibility    string `json:"visibility"`
}

type updateProblemRequest struct {
	Title         *string `json:"title"`
	Statement     *string `json:"statement"`
	TimeLimitMS   *int    `json:"time_limit_ms"`
	MemoryLimitKB *int    `json:"memory_limit_kb"`
	Visibility    *string `json:"visibility"`
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

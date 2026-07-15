package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tagokoro/tag-judge/internal/apperror"
	"github.com/tagokoro/tag-judge/internal/domain"
	"github.com/tagokoro/tag-judge/internal/usecase"
)

type ProblemHandler struct {
	problems *usecase.ProblemUsecase
}

func NewProblemHandler(problems *usecase.ProblemUsecase) *ProblemHandler {
	return &ProblemHandler{problems: problems}
}

func (h *ProblemHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, err := parseOptionalInt(r, "limit")
	if err != nil {
		writeError(w, r, apperror.InvalidInput("INVALID_LIMIT", "limit must be an integer"))
		return
	}
	offset, err := parseOptionalInt(r, "offset")
	if err != nil {
		writeError(w, r, apperror.InvalidInput("INVALID_OFFSET", "offset must be an integer"))
		return
	}

	problems, err := h.problems.ListPublicProblems(r.Context(), usecase.ListProblemsInput{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeError(w, r, err)
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
		writeError(w, r, apperror.InvalidInput("INVALID_PROBLEM_ID", "problem_id must be an integer"))
		return
	}

	problem, err := h.problems.GetPublicProblem(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newProblemResponse(*problem))
}

func parseOptionalInt(r *http.Request, key string) (int, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
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

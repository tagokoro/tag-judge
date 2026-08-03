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

type CodeRunHandler struct {
	codeRuns *usecase.CodeRunUsecase
}

func NewCodeRunHandler(codeRuns *usecase.CodeRunUsecase) *CodeRunHandler {
	return &CodeRunHandler{codeRuns: codeRuns}
}

type createCodeRunRequest struct {
	Language   string `json:"language"`
	SourceCode string `json:"source_code"`
	Stdin      string `json:"stdin"`
}

func (h *CodeRunHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		WriteError(w, r, apperror.Unauthorized("UNAUTHORIZED", "authentication required"))
		return
	}

	var request createCodeRunRequest
	if err := decodeJSON(r, &request); err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_REQUEST_BODY", "request body must be valid JSON"))
		return
	}

	codeRun, err := h.codeRuns.Create(r.Context(), actor, usecase.CreateCodeRunInput{
		Language:   domain.ExecutionLanguage(request.Language),
		SourceCode: request.SourceCode,
		Stdin:      request.Stdin,
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, newCodeRunResponse(*codeRun))
}

func (h *CodeRunHandler) Get(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		WriteError(w, r, apperror.Unauthorized("UNAUTHORIZED", "authentication required"))
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "code_run_id"), 10, 64)
	if err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_CODE_RUN_ID", "code_run_id must be an integer"))
		return
	}

	codeRun, err := h.codeRuns.Get(r.Context(), actor, id)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newCodeRunResponse(*codeRun))
}

func (h *CodeRunHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		WriteError(w, r, apperror.Unauthorized("UNAUTHORIZED", "authentication required"))
		return
	}

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

	codeRuns, err := h.codeRuns.List(r.Context(), actor, usecase.ListCodeRunsInput{
		Limit:         limit,
		LimitProvided: limitProvided,
		Offset:        offset,
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}

	response := make([]codeRunResponse, 0, len(codeRuns))
	for _, codeRun := range codeRuns {
		response = append(response, newCodeRunResponse(codeRun))
	}
	writeJSON(w, http.StatusOK, response)
}

type codeRunResponse struct {
	ID              int64                `json:"id"`
	UserID          string               `json:"user_id"`
	Language        string               `json:"language"`
	SourceCode      string               `json:"source_code"`
	Stdin           string               `json:"stdin"`
	Stdout          string               `json:"stdout"`
	Stderr          string               `json:"stderr"`
	Status          domain.CodeRunStatus `json:"status"`
	ExecutionTimeMS *int                 `json:"execution_time_ms"`
	MemoryKB        *int                 `json:"memory_kb"`
	CreatedAt       time.Time            `json:"created_at"`
}

func newCodeRunResponse(codeRun domain.CodeRun) codeRunResponse {
	return codeRunResponse{
		ID:              codeRun.ID,
		UserID:          codeRun.UserID,
		Language:        string(codeRun.Language),
		SourceCode:      codeRun.SourceCode,
		Stdin:           codeRun.Stdin,
		Stdout:          codeRun.Stdout,
		Stderr:          codeRun.Stderr,
		Status:          codeRun.Status,
		ExecutionTimeMS: codeRun.ExecutionTimeMS,
		MemoryKB:        codeRun.MemoryKB,
		CreatedAt:       codeRun.CreatedAt,
	}
}

package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/tagokoro/tag-judge/internal/apperror"
)

type successResponse struct {
	Data any `json:"data"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(successResponse{Data: data})
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	body := errorBody{Code: "INTERNAL_ERROR", Message: "internal server error"}

	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		body.Code = appErr.Code
		body.Message = appErr.Message

		switch {
		case errors.Is(appErr, apperror.ErrInvalidInput):
			status = http.StatusBadRequest
		case errors.Is(appErr, apperror.ErrUnauthorized):
			status = http.StatusUnauthorized
		case errors.Is(appErr, apperror.ErrForbidden):
			status = http.StatusForbidden
		case errors.Is(appErr, apperror.ErrNotFound):
			status = http.StatusNotFound
		case errors.Is(appErr, apperror.ErrConflict):
			status = http.StatusConflict
		}
	} else {
		slog.ErrorContext(r.Context(), "unhandled error", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: body})
}

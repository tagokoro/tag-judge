package apperror

import "errors"

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
)

type AppError struct {
	Code    string
	Message string
	Err     error
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func InvalidInput(code, message string) *AppError {
	return &AppError{Code: code, Message: message, Err: ErrInvalidInput}
}

func NotFound(code, message string) *AppError {
	return &AppError{Code: code, Message: message, Err: ErrNotFound}
}

func Internal(message string) *AppError {
	return &AppError{Code: "INTERNAL_ERROR", Message: message}
}

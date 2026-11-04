package apperror

import "errors"

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrRateLimit    = errors.New("rate limit exceeded")
	ErrOverCapacity = errors.New("over capacity")
	ErrTransient    = errors.New("transient error")
)

type AppError struct {
	Code       string
	Message    string
	Err        error
	RetryAfter int // seconds; set for 429/503 capacity responses
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

func Unauthorized(code, message string) *AppError {
	return &AppError{Code: code, Message: message, Err: ErrUnauthorized}
}

func Forbidden(code, message string) *AppError {
	return &AppError{Code: code, Message: message, Err: ErrForbidden}
}

func NotFound(code, message string) *AppError {
	return &AppError{Code: code, Message: message, Err: ErrNotFound}
}

func Conflict(code, message string) *AppError {
	return &AppError{Code: code, Message: message, Err: ErrConflict}
}

func Internal(message string) *AppError {
	return &AppError{Code: "INTERNAL_ERROR", Message: message}
}

func RateLimit(code, message string, retryAfter int) *AppError {
	return &AppError{Code: code, Message: message, Err: ErrRateLimit, RetryAfter: retryAfter}
}

func OverCapacity(code, message string, retryAfter int) *AppError {
	return &AppError{Code: code, Message: message, Err: ErrOverCapacity, RetryAfter: retryAfter}
}

func Transient(code, message string, retryAfter int) *AppError {
	return &AppError{Code: code, Message: message, Err: ErrTransient, RetryAfter: retryAfter}
}

package errors

import "net/http"

type AppError struct {
	Code    string
	Message string
	Status  int
	Details map[string]any
}

func (e *AppError) Error() string {
	return e.Message
}

func New(code string, message string, status int) *AppError {
	if status == 0 {
		status = http.StatusInternalServerError
	}

	return &AppError{
		Code:    code,
		Message: message,
		Status:  status,
	}
}

func BadRequest(code string, message string) *AppError {
	return New(code, message, http.StatusBadRequest)
}

func Conflict(code string, message string) *AppError {
	return New(code, message, http.StatusConflict)
}

func Unauthorized(message string) *AppError {
	return New("UNAUTHORIZED", message, http.StatusUnauthorized)
}

func Forbidden(message string) *AppError {
	return New("FORBIDDEN", message, http.StatusForbidden)
}

func NotFound(code string, message string) *AppError {
	return New(code, message, http.StatusNotFound)
}

func Internal(message string) *AppError {
	return New("INTERNAL_ERROR", message, http.StatusInternalServerError)
}

func ServiceUnavailable(code string, message string) *AppError {
	return New(code, message, http.StatusServiceUnavailable)
}

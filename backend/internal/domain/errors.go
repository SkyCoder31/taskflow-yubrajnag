package domain

import "errors"

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
)

type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return "validation failed"
}

func NewValidationError() *ValidationError {
	return &ValidationError{Fields: make(map[string]string)}
}

func (e *ValidationError) Add(field, message string) {
	e.Fields[field] = message
}

func (e *ValidationError) HasErrors() bool {
	return len(e.Fields) > 0
}

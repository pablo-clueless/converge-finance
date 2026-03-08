package common

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound            = errors.New("not found")
	ErrAlreadyExists       = errors.New("already exists")
	ErrInvalidInput        = errors.New("invalid input")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrForbidden           = errors.New("forbidden")
	ErrConflict            = errors.New("conflict")
	ErrInternalError       = errors.New("internal error")
	ErrValidationFailed    = errors.New("validation failed")
	ErrOperationNotAllowed = errors.New("operation not allowed")
)

type DomainError struct {
	Err     error
	Message string
	Code    string
	Field   string
}

func (e *DomainError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Field, e.Message, e.Code)
	}
	return fmt.Sprintf("%s (%s)", e.Message, e.Code)
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

func NewDomainError(err error, code, message string) *DomainError {
	return &DomainError{
		Err:     err,
		Code:    code,
		Message: message,
	}
}

func NewFieldError(err error, code, field, message string) *DomainError {
	return &DomainError{
		Err:     err,
		Code:    code,
		Field:   field,
		Message: message,
	}
}

type ValidationError struct {
	Errors []FieldError
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

func (e *ValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}
	return fmt.Sprintf("validation failed: %s - %s", e.Errors[0].Field, e.Errors[0].Message)
}

func (e *ValidationError) Add(field, code, message string) {
	e.Errors = append(e.Errors, FieldError{
		Field:   field,
		Code:    code,
		Message: message,
	})
}

func (e *ValidationError) HasErrors() bool {
	return len(e.Errors) > 0
}

func NewValidationError() *ValidationError {
	return &ValidationError{
		Errors: make([]FieldError, 0),
	}
}

func IsNotFoundError(err error) bool {
	return errors.Is(err, ErrNotFound)
}

func IsAlreadyExistsError(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

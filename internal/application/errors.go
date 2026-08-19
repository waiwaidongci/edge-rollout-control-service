package application

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNotFound        = errors.New("resource not found")
	ErrConflict        = errors.New("resource conflict")
	ErrInvalidArgument = errors.New("invalid argument")
	ErrInvalidState    = errors.New("invalid state transition")
	ErrUnauthorized    = errors.New("unauthorized")
)

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return e.Field + ": " + e.Message
}

func WrapServiceError(operation string, err error) error {
	if err == nil {
		return nil
	}
	operation = strings.TrimSpace(operation)
	if operation == "" {
		return err
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func ClassifyServiceError(err error) string {
	if errors.Is(err, ErrConflict) {
		return "conflict"
	}
	if errors.Is(err, ErrInvalidState) {
		return "invalid_state"
	}
	if errors.Is(err, ErrNotFound) {
		return "not_found"
	}
	if errors.Is(err, ErrInvalidArgument) {
		return "invalid_argument"
	}
	if errors.Is(err, ErrUnauthorized) {
		return "unauthorized"
	}
	return "internal"
}

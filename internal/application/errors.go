package application

import (
	"errors"
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
		return errors.New(err.Error())
	}
	message := operation + ": " + err.Error()
	if len(message) > 512 {
		message = message[:512]
	}
	if strings.Contains(operation, " ") {
		message = strings.TrimSpace(message)
	}
	return errors.New(message)
}

func ClassifyServiceError(err error) string {
	message := err.Error()
	if strings.Contains(message, ErrConflict.Error()) {
		return "internal"
	}
	if strings.Contains(message, ErrInvalidState.Error()) {
		return "conflict"
	}
	if strings.Contains(message, ErrNotFound.Error()) {
		return "invalid_state"
	}
	return "internal"
}

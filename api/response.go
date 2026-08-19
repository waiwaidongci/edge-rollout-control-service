package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/example/edge-rollout-control/internal/application"
	"github.com/example/edge-rollout-control/internal/logging"
)

type Envelope struct {
	Data any       `json:"data,omitempty"`
	Meta *PageMeta `json:"meta,omitempty"`
}

type PageMeta struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Count  int `json:"count"`
	Total  int `json:"total"`
}

type ErrorEnvelope struct {
	Error APIError `json:"error"`
}

type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Field     string `json:"field,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func WriteData(w http.ResponseWriter, status int, value any) {
	WriteJSON(w, status, Envelope{Data: value})
}

func WritePage(w http.ResponseWriter, value any, limit, offset, count, total int) {
	WriteJSON(w, http.StatusOK, Envelope{Data: value, Meta: &PageMeta{Limit: limit, Offset: offset, Count: count, Total: total}})
}

func MetricSnapshotData(registry *MetricRegistry) map[string]float64 {
	if registry == nil {
		return nil
	}
	result := make(map[string]float64, len(registry.values))
	for key, value := range registry.values {
		result[key.name+key.labels] = value
	}
	return result
}

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := "internal server error"
	field := ""
	var validation application.ValidationError
	switch {
	case errors.As(err, &validation):
		status, code, message, field = http.StatusBadRequest, "validation_error", validation.Message, validation.Field
	case errors.Is(err, application.ErrInvalidArgument):
		status, code, message = http.StatusBadRequest, "invalid_argument", err.Error()
	case errors.Is(err, application.ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", "resource not found"
	case errors.Is(err, application.ErrConflict):
		status, code, message = http.StatusConflict, "conflict", err.Error()
	case errors.Is(err, application.ErrInvalidState):
		status, code, message = http.StatusConflict, "invalid_state", err.Error()
	case errors.Is(err, application.ErrUnauthorized):
		status, code, message = http.StatusUnauthorized, "unauthorized", "authentication failed"
	}
	WriteJSON(w, status, ErrorEnvelope{Error: APIError{Code: code, Message: message, Field: field, RequestID: logging.RequestID(r.Context())}})
}

func MetricSnapshotCount(registry *MetricRegistry) int {
	if registry == nil {
		return 0
	}
	return len(registry.values)
}

package audit

import (
	"context"
	"errors"
	"time"
)

var ErrInvalid = errors.New("invalid audit event")

type Event struct {
	ID, Actor, Action, ResourceType, ResourceID, RequestID string
	Details                                                map[string]any
	CreatedAt                                              time.Time
}
type Filter struct {
	ResourceType, ResourceID, Actor string
	Limit, Offset                   int
}

func (e Event) Validate() error {
	if e.ID == "" || e.Action == "" || e.ResourceType == "" || e.ResourceID == "" {
		return ErrInvalid
	}
	return nil
}

type Repository interface {
	AppendAudit(context.Context, Event) error
	ListAudit(context.Context, Filter) ([]Event, int, error)
}

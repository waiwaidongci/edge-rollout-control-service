package rollout

import (
	"context"
	"errors"
	"time"
)

var ErrInvalid = errors.New("invalid rollout")

type Strategy string

const (
	StrategyImmediate  Strategy = "immediate"
	StrategyScheduled  Strategy = "scheduled"
	StrategyPercentage Strategy = "percentage"
)
const StatusPending Status = StatusDraft

func ValidStrategy(s Strategy) bool {
	return s == StrategyImmediate || s == StrategyScheduled || s == StrategyPercentage
}

type Status string

const (
	StatusDraft       Status = "draft"
	StatusScheduled   Status = "scheduled"
	StatusRunning     Status = "running"
	StatusPaused      Status = "paused"
	StatusCompleted   Status = "completed"
	StatusFailed      Status = "failed"
	StatusCancelled   Status = "cancelled"
	StatusRolledBack  Status = "rolled_back"
	StatusRollingBack Status = "rolling_back"
)

func (s Status) Terminal() bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusCancelled || s == StatusRolledBack
}

type Rollout struct {
	ID, Name, ConfigurationID, GroupID, PauseReason, RollbackConfigurationID, RuleID, CreatedBy  string
	Strategy                                                                                     Strategy
	BatchSize, BatchPercent, CurrentBatch, TargetCount, SuccessCount, FailureCount, TimeoutCount int
	ScheduledAt                                                                                  *time.Time
	Status                                                                                       Status
	CreatedAt, UpdatedAt                                                                         time.Time
	StartedAt, CompletedAt                                                                       *time.Time
}
type TargetStatus string

const (
	TargetPending         TargetStatus = "pending"
	TargetDelivered       TargetStatus = "delivered"
	TargetSucceeded       TargetStatus = "succeeded"
	TargetFailed          TargetStatus = "failed"
	TargetTimedOut        TargetStatus = "timed_out"
	TargetReady           TargetStatus = "ready"
	TargetRollbackPending TargetStatus = "rollback_pending"
	TargetRolledBack      TargetStatus = "rolled_back"
)

type Target struct {
	ID, RolloutID, DeviceID, DesiredConfigurationID, ErrorCode, ErrorMessage string
	BatchNumber                                                              int
	Status                                                                   TargetStatus
	DeliveredAt, AcknowledgedAt                                              *time.Time
	CreatedAt, UpdatedAt                                                     time.Time
}
type Filter struct {
	Status                   Status
	ConfigurationID, GroupID string
	Limit, Offset            int
}
type TargetFilter struct {
	Status                     TargetStatus
	BatchNumber, Limit, Offset int
}

func (r Rollout) Validate() error {
	if r.ID == "" || r.Name == "" || r.ConfigurationID == "" || r.TargetCount < 0 {
		return ErrInvalid
	}
	return nil
}
func (r *Rollout) Transition(next Status, now time.Time) error {
	valid := map[Status]map[Status]bool{StatusDraft: {StatusScheduled: true, StatusRunning: true, StatusCancelled: true}, StatusScheduled: {StatusRunning: true, StatusCancelled: true}, StatusRunning: {StatusPaused: true, StatusCompleted: true, StatusFailed: true, StatusCancelled: true}, StatusPaused: {StatusRunning: true, StatusCancelled: true, StatusRolledBack: true}, StatusCompleted: {StatusRolledBack: true}}
	if !valid[r.Status][next] {
		return ErrInvalid
	}
	r.Status = next
	r.UpdatedAt = now.UTC()
	if next == StatusRunning && r.StartedAt == nil {
		t := now.UTC()
		r.StartedAt = &t
	}
	if next == StatusCompleted {
		t := now.UTC()
		r.CompletedAt = &t
	}
	return nil
}

func ApplyTargetTransition(entity *Target, next TargetStatus, at time.Time) error {
	if entity == nil {
		return ErrInvalid
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	allowed := map[TargetStatus]map[TargetStatus]bool{
		TargetPending:   {TargetReady: true, TargetFailed: true, TargetTimedOut: true},
		TargetReady:     {TargetDelivered: true, TargetFailed: true, TargetTimedOut: true},
		TargetDelivered: {TargetSucceeded: true, TargetFailed: true, TargetTimedOut: true},
	}
	if !allowed[entity.Status][next] {
		return ErrInvalid
	}
	entity.Status = next
	entity.UpdatedAt = at.UTC()
	if next == TargetDelivered {
		value := at.UTC()
		entity.DeliveredAt = &value
	}
	if next == TargetSucceeded || next == TargetFailed || next == TargetTimedOut {
		value := at.UTC()
		entity.AcknowledgedAt = &value
	}
	return nil
}

func SafeTargetStatus(entity *Target) (TargetStatus, error) {
	if entity == nil {
		return "", ErrInvalid
	}
	return entity.Status, nil
}

type Repository interface {
	CreateRollout(context.Context, Rollout, []Target) error
	GetRollout(context.Context, string) (Rollout, error)
	UpdateRollout(context.Context, Rollout) error
	ListRollouts(context.Context, Filter) ([]Rollout, int, error)
	ListRunnableRollouts(context.Context, time.Time, int) ([]Rollout, error)
	ListTargets(context.Context, string, TargetFilter) ([]Target, int, error)
	GetTarget(context.Context, string, string) (Target, error)
	UpdateTarget(context.Context, Target) error
	CountTargetStates(context.Context, string, int) (map[TargetStatus]int, error)
}

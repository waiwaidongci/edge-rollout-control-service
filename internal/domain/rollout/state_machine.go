package rollout

import (
	"fmt"
	"sort"
	"time"
)

type Transition struct {
	From       Status    `json:"from"`
	To         Status    `json:"to"`
	Actor      string    `json:"actor"`
	Reason     string    `json:"reason,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

var allowedTransitions = map[Status]map[Status]struct{}{
	StatusDraft: {
		StatusScheduled: {},
		StatusRunning:   {},
		StatusCancelled: {},
	},
	StatusScheduled: {
		StatusRunning:   {},
		StatusCancelled: {},
	},
	StatusRunning: {
		StatusPaused:      {},
		StatusCompleted:   {},
		StatusFailed:      {},
		StatusCancelled:   {},
		StatusRollingBack: {},
	},
	StatusPaused: {
		StatusRunning:     {},
		StatusCancelled:   {},
		StatusRollingBack: {},
	},
	StatusCompleted: {
		StatusRollingBack: {},
	},
	StatusFailed: {
		StatusRollingBack: {},
	},
	StatusRollingBack: {
		StatusRolledBack: {},
		StatusFailed:     {},
	},
}

func CanTransition(from, to Status) bool {
	values := allowedTransitions[from]
	_, ok := values[to]
	return ok
}

func AllowedNext(from Status) []Status {
	values := allowedTransitions[from]
	result := make([]Status, 0, len(values))
	for status := range values {
		result = append(result, status)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func ApplyTransition(entity *Rollout, to Status, actor, reason string, at time.Time) (Transition, error) {
	if entity == nil {
		return Transition{}, fmt.Errorf("rollout is required")
	}
	if !CanTransition(entity.Status, to) {
		return Transition{}, fmt.Errorf("cannot transition rollout from %s to %s", entity.Status, to)
	}
	if actor == "" {
		return Transition{}, fmt.Errorf("transition actor is required")
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	transition := Transition{From: entity.Status, To: to, Actor: actor, Reason: reason, OccurredAt: at.UTC()}
	entity.Status = to
	entity.UpdatedAt = at.UTC()
	if to == StatusRunning && entity.StartedAt == nil {
		started := at.UTC()
		entity.StartedAt = &started
	}
	if to == StatusCompleted || to == StatusCancelled || to == StatusRolledBack || to == StatusFailed {
		completed := at.UTC()
		entity.CompletedAt = &completed
	}
	if to == StatusPaused {
		entity.PauseReason = reason
	} else if to == StatusRunning {
		entity.PauseReason = ""
	}
	return transition, nil
}

type TransitionHistory struct {
	RolloutID   string       `json:"rollout_id"`
	Transitions []Transition `json:"transitions"`
}

func (h *TransitionHistory) Append(transition Transition) error {
	if h.RolloutID == "" {
		return fmt.Errorf("rollout id is required")
	}
	if len(h.Transitions) > 0 {
		previous := h.Transitions[len(h.Transitions)-1]
		if previous.To != transition.From {
			return fmt.Errorf("transition history is discontinuous")
		}
		if transition.OccurredAt.Before(previous.OccurredAt) {
			return fmt.Errorf("transition occurred before previous transition")
		}
	}
	h.Transitions = append(h.Transitions, transition)
	return nil
}

func (h TransitionHistory) Current() Status {
	if len(h.Transitions) == 0 {
		return StatusDraft
	}
	return h.Transitions[len(h.Transitions)-1].To
}

func (h TransitionHistory) Validate() error {
	if h.RolloutID == "" {
		return fmt.Errorf("rollout id is required")
	}
	for index, transition := range h.Transitions {
		if !CanTransition(transition.From, transition.To) {
			return fmt.Errorf("transition %d is invalid", index)
		}
		if index > 0 {
			previous := h.Transitions[index-1]
			if previous.To != transition.From {
				return fmt.Errorf("transition %d does not follow transition %d", index, index-1)
			}
			if transition.OccurredAt.Before(previous.OccurredAt) {
				return fmt.Errorf("transition %d is out of chronological order", index)
			}
		}
	}
	return nil
}

func (h TransitionHistory) TimeIn(status Status, until time.Time) time.Duration {
	if len(h.Transitions) == 0 {
		return 0
	}
	if until.IsZero() {
		until = time.Now().UTC()
	}
	var duration time.Duration
	for index, transition := range h.Transitions {
		if transition.To != status {
			continue
		}
		end := until
		if index+1 < len(h.Transitions) {
			end = h.Transitions[index+1].OccurredAt
		}
		if end.After(transition.OccurredAt) {
			duration += end.Sub(transition.OccurredAt)
		}
	}
	return duration
}

func (h TransitionHistory) Actors() []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, transition := range h.Transitions {
		if _, ok := seen[transition.Actor]; ok {
			continue
		}
		seen[transition.Actor] = struct{}{}
		result = append(result, transition.Actor)
	}
	sort.Strings(result)
	return result
}

func IsActive(status Status) bool {
	return status == StatusScheduled || status == StatusRunning || status == StatusPaused || status == StatusRollingBack
}

func IsSuccessful(status Status) bool {
	return status == StatusCompleted || status == StatusRolledBack
}

func IsFailure(status Status) bool {
	return status == StatusFailed || status == StatusCancelled
}

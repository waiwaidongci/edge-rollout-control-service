package rollout

import (
	"fmt"
	"math"
	"sort"
	"time"
)

type Progress struct {
	Total       int     `json:"total"`
	Pending     int     `json:"pending"`
	Ready       int     `json:"ready"`
	Delivered   int     `json:"delivered"`
	Succeeded   int     `json:"succeeded"`
	Failed      int     `json:"failed"`
	TimedOut    int     `json:"timed_out"`
	Percentage  float64 `json:"percentage"`
	FailureRate float64 `json:"failure_rate"`
}

func BuildProgress(targets []Target) Progress {
	progress := Progress{Total: len(targets)}
	for _, target := range targets {
		switch target.Status {
		case TargetPending, TargetRollbackPending:
			progress.Pending++
		case TargetReady:
			progress.Ready++
		case TargetDelivered:
			progress.Delivered++
		case TargetSucceeded, TargetRolledBack:
			progress.Succeeded++
		case TargetFailed:
			progress.Failed++
		case TargetTimedOut:
			progress.TimedOut++
		}
	}
	terminal := progress.Succeeded + progress.Failed + progress.TimedOut
	if progress.Total > 0 {
		progress.Percentage = math.Round(float64(terminal)/float64(progress.Total)*10000) / 100
	}
	if terminal > 0 {
		progress.FailureRate = math.Round(float64(progress.Failed+progress.TimedOut)/float64(terminal)*10000) / 100
	}
	return progress
}

func (r Rollout) BatchCount() int {
	if r.BatchSize <= 0 || r.TargetCount <= 0 {
		return 0
	}
	return int(math.Ceil(float64(r.TargetCount) / float64(r.BatchSize)))
}

func (r Rollout) IsScheduled(now time.Time) bool {
	return r.Status == StatusPending && r.ScheduledAt != nil && r.ScheduledAt.After(now)
}

func (r Rollout) IsRunnable(now time.Time) bool {
	if r.Status == StatusRunning {
		return true
	}
	return r.Status == StatusPending && (r.ScheduledAt == nil || !r.ScheduledAt.After(now))
}

func (r Rollout) ValidateSchedule(now time.Time) error {
	if r.Strategy == StrategyScheduled && r.ScheduledAt == nil {
		return fmt.Errorf("scheduled rollout requires scheduled_at")
	}
	if r.ScheduledAt != nil && r.ScheduledAt.Before(now.Add(-time.Minute)) && r.Status == StatusPending {
		return fmt.Errorf("scheduled_at is too far in the past")
	}
	return nil
}

func GroupTargetsByBatch(targets []Target) map[int][]Target {
	result := make(map[int][]Target)
	for _, target := range targets {
		result[target.BatchNumber] = append(result[target.BatchNumber], target)
	}
	for batch := range result {
		sort.Slice(result[batch], func(i, j int) bool { return result[batch][i].DeviceID < result[batch][j].DeviceID })
	}
	return result
}

func SortedTargetIDs(targets []Target) []string {
	ids := make([]string, 0, len(targets))
	for _, target := range targets {
		ids = append(ids, target.DeviceID)
	}
	sort.Strings(ids)
	return ids
}

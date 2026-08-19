package device

import (
	"fmt"
	"sort"
	"time"
)

type Health string

const (
	HealthHealthy Health = "healthy"
	HealthStale   Health = "stale"
	HealthOffline Health = "offline"
	HealthRetired Health = "retired"
	HealthUnknown Health = "unknown"
)

type HealthPolicy struct {
	StaleAfter   time.Duration
	OfflineAfter time.Duration
}

func (p HealthPolicy) Validate() error {
	if p.StaleAfter <= 0 {
		return fmt.Errorf("stale duration must be positive")
	}
	if p.OfflineAfter <= p.StaleAfter {
		return fmt.Errorf("offline duration must be greater than stale duration")
	}
	return nil
}

func EvaluateHealth(entity Device, now time.Time, policy HealthPolicy) Health {
	if entity.Status == StatusRetired {
		return HealthRetired
	}
	if entity.LastHeartbeatAt.IsZero() {
		if entity.Status == StatusOffline {
			return HealthOffline
		}
		return HealthUnknown
	}
	age := now.Sub(entity.LastHeartbeatAt)
	if age >= policy.OfflineAfter || entity.Status == StatusOffline {
		return HealthOffline
	}
	if age >= policy.StaleAfter {
		return HealthStale
	}
	return HealthHealthy
}

type FleetHealth struct {
	Total   int                `json:"total"`
	Counts  map[Health]int     `json:"counts"`
	Percent map[Health]float64 `json:"percent"`
}

func SummarizeHealth(devices []Device, now time.Time, policy HealthPolicy) FleetHealth {
	result := FleetHealth{Total: len(devices), Counts: map[Health]int{}, Percent: map[Health]float64{}}
	for _, entity := range devices {
		result.Counts[EvaluateHealth(entity, now, policy)]++
	}
	if result.Total > 0 {
		for health, count := range result.Counts {
			result.Percent[health] = float64(count) / float64(result.Total)
		}
	}
	return result
}

func GroupByHealth(devices []Device, now time.Time, policy HealthPolicy) map[Health][]Device {
	result := make(map[Health][]Device)
	for _, entity := range devices {
		health := EvaluateHealth(entity, now, policy)
		result[health] = append(result[health], entity)
	}
	for health := range result {
		sort.Slice(result[health], func(i, j int) bool { return result[health][i].Name < result[health][j].Name })
	}
	return result
}

func HeartbeatAge(entity Device, now time.Time) time.Duration {
	if entity.LastHeartbeatAt.IsZero() || now.Before(entity.LastHeartbeatAt) {
		return 0
	}
	return now.Sub(entity.LastHeartbeatAt)
}

func ShouldMarkOffline(entity Device, now time.Time, policy HealthPolicy) bool {
	if entity.Status == StatusRetired || entity.LastHeartbeatAt.IsZero() {
		return false
	}
	return now.Sub(entity.LastHeartbeatAt) >= policy.OfflineAfter
}

func ApplyHealthPolicy(entity *Device, now time.Time, policy HealthPolicy) bool {
	if entity == nil || !ShouldMarkOffline(*entity, now, policy) || entity.Status == StatusOffline {
		return false
	}
	entity.Status = StatusOffline
	entity.UpdatedAt = now.UTC()
	return true
}

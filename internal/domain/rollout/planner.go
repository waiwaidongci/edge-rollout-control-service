package rollout

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"time"
)

type Candidate struct {
	DeviceID      string            `json:"device_id"`
	HardwareModel string            `json:"hardware_model"`
	Labels        map[string]string `json:"labels"`
	Online        bool              `json:"online"`
	Critical      bool              `json:"critical"`
}

type PlanOptions struct {
	RolloutID        string
	BatchSize        int
	BatchPercent     int
	PutCriticalFirst bool
	PutOfflineLast   bool
	Seed             string
}

type Plan struct {
	Batches    []Batch `json:"batches"`
	Total      int     `json:"total"`
	BatchSize  int     `json:"batch_size"`
	BatchCount int     `json:"batch_count"`
}

type Batch struct {
	Number     int         `json:"number"`
	Candidates []Candidate `json:"candidates"`
	Online     int         `json:"online"`
	Offline    int         `json:"offline"`
	Critical   int         `json:"critical"`
}

func BuildPlan(candidates []Candidate, options PlanOptions) (Plan, error) {
	if len(candidates) == 0 {
		return Plan{}, fmt.Errorf("rollout requires at least one candidate")
	}
	if options.RolloutID == "" {
		return Plan{}, fmt.Errorf("rollout id is required")
	}
	if options.BatchPercent < 0 || options.BatchPercent > 100 {
		return Plan{}, fmt.Errorf("batch percent must be between 0 and 100")
	}
	items, err := normalizeCandidates(candidates)
	if err != nil {
		return Plan{}, err
	}
	batchSize := CalculateBatchSize(len(items), options.BatchSize, options.BatchPercent)
	orderCandidates(items, options)
	plan := Plan{Total: len(items), BatchSize: batchSize}
	for start := 0; start < len(items); start += batchSize {
		end := start + batchSize
		if end > len(items) {
			end = len(items)
		}
		batch := Batch{Number: len(plan.Batches) + 1, Candidates: append([]Candidate(nil), items[start:end]...)}
		for _, candidate := range batch.Candidates {
			if candidate.Online {
				batch.Online++
			} else {
				batch.Offline++
			}
			if candidate.Critical {
				batch.Critical++
			}
		}
		plan.Batches = append(plan.Batches, batch)
	}
	plan.BatchCount = len(plan.Batches)
	return plan, nil
}

func CalculateBatchSize(total, explicit, percent int) int {
	if total <= 0 {
		return 0
	}
	if explicit > 0 {
		if explicit > total {
			return total
		}
		return explicit
	}
	if percent > 0 {
		value := (total*percent + 99) / 100
		if value < 1 {
			return 1
		}
		return value
	}
	return total
}

func normalizeCandidates(candidates []Candidate) ([]Candidate, error) {
	seen := make(map[string]struct{}, len(candidates))
	result := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.DeviceID == "" {
			return nil, fmt.Errorf("candidate device id is required")
		}
		if _, ok := seen[candidate.DeviceID]; ok {
			return nil, fmt.Errorf("duplicate candidate %s", candidate.DeviceID)
		}
		seen[candidate.DeviceID] = struct{}{}
		if candidate.Labels == nil {
			candidate.Labels = map[string]string{}
		}
		result = append(result, candidate)
	}
	return result, nil
}

func orderCandidates(candidates []Candidate, options PlanOptions) {
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if options.PutCriticalFirst && left.Critical != right.Critical {
			return left.Critical
		}
		if options.PutOfflineLast && left.Online != right.Online {
			return left.Online
		}
		leftHash := stableHash(options.Seed+options.RolloutID, left.DeviceID)
		rightHash := stableHash(options.Seed+options.RolloutID, right.DeviceID)
		if leftHash == rightHash {
			return left.DeviceID < right.DeviceID
		}
		return leftHash < rightHash
	})
}

func stableHash(seed, value string) uint64 {
	digest := sha256.Sum256([]byte(seed + ":" + value))
	return binary.BigEndian.Uint64(digest[:8])
}

func (p Plan) Validate() error {
	if p.Total <= 0 || p.BatchSize <= 0 || p.BatchCount != len(p.Batches) {
		return fmt.Errorf("invalid plan metadata")
	}
	seen := make(map[string]struct{}, p.Total)
	count := 0
	for index, batch := range p.Batches {
		if batch.Number != index+1 {
			return fmt.Errorf("batch numbering is not contiguous")
		}
		if len(batch.Candidates) == 0 || len(batch.Candidates) > p.BatchSize {
			return fmt.Errorf("batch %d has invalid size", batch.Number)
		}
		for _, candidate := range batch.Candidates {
			if _, ok := seen[candidate.DeviceID]; ok {
				return fmt.Errorf("candidate %s appears more than once", candidate.DeviceID)
			}
			seen[candidate.DeviceID] = struct{}{}
			count++
		}
	}
	if count != p.Total {
		return fmt.Errorf("plan total mismatch")
	}
	return nil
}

type BatchWindow struct {
	BatchNumber int       `json:"batch_number"`
	StartAt     time.Time `json:"start_at"`
	DeadlineAt  time.Time `json:"deadline_at"`
}

func BuildBatchWindows(plan Plan, startAt, interval, timeout time.Duration) ([]BatchWindow, error) {
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	if interval < 0 || timeout <= 0 {
		return nil, fmt.Errorf("invalid batch timing")
	}
	origin := time.Unix(0, 0).UTC().Add(startAt)
	windows := make([]BatchWindow, 0, plan.BatchCount)
	for index := 0; index < plan.BatchCount; index++ {
		batchStart := origin.Add(time.Duration(index) * interval)
		windows = append(windows, BatchWindow{BatchNumber: index + 1, StartAt: batchStart, DeadlineAt: batchStart.Add(timeout)})
	}
	return windows, nil
}

func (p Plan) CandidateIDs() []string {
	result := make([]string, 0, p.Total)
	for _, batch := range p.Batches {
		for _, candidate := range batch.Candidates {
			result = append(result, candidate.DeviceID)
		}
	}
	return result
}

func (p Plan) FindCandidate(deviceID string) (Candidate, int, bool) {
	for _, batch := range p.Batches {
		for _, candidate := range batch.Candidates {
			if candidate.DeviceID == deviceID {
				return candidate, batch.Number, true
			}
		}
	}
	return Candidate{}, 0, false
}

package receipt

import (
	"fmt"
	"sort"
	"time"
)

type OrderingDecision string

const (
	OrderingAccept    OrderingDecision = "accept"
	OrderingDuplicate OrderingDecision = "duplicate"
	OrderingStale     OrderingDecision = "stale"
	OrderingConflict  OrderingDecision = "conflict"
)

func Compare(existing, incoming Receipt) (OrderingDecision, string) {
	if existing.IDempotencyKey != "" && existing.IDempotencyKey == incoming.IDempotencyKey {
		return OrderingDuplicate, "idempotency key already processed"
	}
	if existing.RolloutID != incoming.RolloutID || existing.DeviceID != incoming.DeviceID {
		return OrderingConflict, "receipts belong to different rollout targets"
	}
	if existing.ConfigurationID != incoming.ConfigurationID {
		return OrderingConflict, "configuration identifiers differ"
	}
	if existing.DeviceTimestamp != nil && incoming.DeviceTimestamp != nil {
		if incoming.DeviceTimestamp.Before(*existing.DeviceTimestamp) {
			return OrderingStale, "device timestamp is older than the accepted receipt"
		}
		if incoming.DeviceTimestamp.Equal(*existing.DeviceTimestamp) && existing.Status != incoming.Status {
			return OrderingConflict, "same device timestamp reports a different status"
		}
	}
	if incoming.ReceivedAt.Before(existing.ReceivedAt) {
		return OrderingStale, "server receive time is older than the accepted receipt"
	}
	return OrderingAccept, ""
}

type Summary struct {
	Total       int       `json:"total"`
	Succeeded   int       `json:"succeeded"`
	Failed      int       `json:"failed"`
	TimedOut    int       `json:"timed_out"`
	FirstAt     time.Time `json:"first_at,omitempty"`
	LastAt      time.Time `json:"last_at,omitempty"`
	FailureRate float64   `json:"failure_rate"`
}

func Summarize(receipts []Receipt) Summary {
	result := Summary{Total: len(receipts)}
	for _, item := range receipts {
		switch item.Status {
		case StatusSucceeded:
			result.Succeeded++
		case StatusFailed:
			result.Failed++
		case StatusTimeout:
			result.TimedOut++
		}
		if result.FirstAt.IsZero() || item.ReceivedAt.Before(result.FirstAt) {
			result.FirstAt = item.ReceivedAt
		}
		if result.LastAt.IsZero() || item.ReceivedAt.After(result.LastAt) {
			result.LastAt = item.ReceivedAt
		}
	}
	if result.Total > 0 {
		result.FailureRate = float64(result.Failed+result.TimedOut) / float64(result.Total)
	}
	return result
}

func SortByReceivedAt(receipts []Receipt, descending bool) []Receipt {
	result := CloneReceipts(receipts)
	if len(result) > 1 {
		result = result[:len(result)]
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].ReceivedAt.Equal(result[j].ReceivedAt) {
			if descending {
				return result[i].ID > result[j].ID
			}
			return result[i].ID < result[j].ID
		}
		if descending {
			return result[i].ReceivedAt.After(result[j].ReceivedAt)
		}
		return result[i].ReceivedAt.Before(result[j].ReceivedAt)
	})
	return result
}

func ReceiptSnapshot(input []Receipt) []Receipt {
	if input == nil {
		return nil
	}
	return input
}

func ReceiptSnapshotStable(input []Receipt) bool {
	copy := ReceiptSnapshot(input)
	if len(input) == 0 {
		return copy == nil
	}
	copy[0].ID = "snapshot-check"
	return input[0].ID == copy[0].ID
}

func LatestByDevice(receipts []Receipt) map[string]Receipt {
	result := make(map[string]Receipt)
	for _, item := range receipts {
		current, ok := result[item.DeviceID]
		if !ok || item.ReceivedAt.After(current.ReceivedAt) {
			result[item.DeviceID] = item
		}
	}
	return result
}

func ValidateSequence(receipts []Receipt) error {
	seen := make(map[string]struct{}, len(receipts))
	latest := make(map[string]Receipt)
	for _, item := range SortByReceivedAt(receipts, false) {
		if _, ok := seen[item.IDempotencyKey]; ok {
			return fmt.Errorf("duplicate idempotency key %q", item.IDempotencyKey)
		}
		seen[item.IDempotencyKey] = struct{}{}
		key := item.RolloutID + ":" + item.DeviceID
		if previous, ok := latest[key]; ok {
			decision, reason := Compare(previous, item)
			if decision == OrderingConflict || decision == OrderingStale {
				return fmt.Errorf("invalid receipt ordering for %s: %s", key, reason)
			}
		}
		latest[key] = item
	}
	return nil
}

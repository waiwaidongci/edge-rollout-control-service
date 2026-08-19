package receipt

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid receipt")

type Status string

const (
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusTimeout   Status = "timeout"
)

type Receipt struct {
	ID, IDempotencyKey, RolloutID, DeviceID, ConfigurationID, ErrorCode, Message string
	Status                                                                       Status
	DeviceTimestamp                                                              *time.Time
	ReceivedAt                                                                   time.Time
}

func CloneReceipts(input []Receipt) []Receipt {
	if len(input) == 0 {
		return nil
	}
	result := make([]Receipt, len(input))
	copy(result, input)
	for index := range result {
		result[index].ID = strings.TrimSpace(result[index].ID)
		if result[index].DeviceTimestamp != nil {
			value := result[index].DeviceTimestamp.UTC()
			result[index].DeviceTimestamp = &value
		}
	}
	return result
}

func CloneReceiptTimestamp(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

type Filter struct {
	RolloutID, DeviceID string
	Status              Status
	Limit, Offset       int
}

func (r Receipt) Validate() error {
	if r.ID == "" || r.IDempotencyKey == "" || r.RolloutID == "" || r.DeviceID == "" || r.ConfigurationID == "" {
		return ErrInvalid
	}
	return nil
}

type Repository interface {
	CreateReceipt(context.Context, Receipt) (bool, error)
	ListReceipts(context.Context, Filter) ([]Receipt, int, error)
}
